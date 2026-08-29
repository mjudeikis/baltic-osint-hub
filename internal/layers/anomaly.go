package layers

import (
	"math"
	"sort"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// Anomaly is the verdict for the most recent observation of an AOI.
type Anomaly struct {
	Detected bool
	Latest   float64
	Median   float64
	ZScore   float64
	// Baseline is how many prior observations backed the verdict.
	Baseline int
	// SceneAdjusted reports whether the verdict discounted scene-wide
	// brightness changes (weather, harvest, sea state) rather than reading the
	// raw bright-pixel fraction.
	SceneAdjusted bool
	// SceneShifted means the latest pass was taken under conditions unlike
	// anything in the baseline, so no comparison is meaningful and detection
	// is suppressed. "We cannot tell" is the honest answer, not "alarm".
	SceneShifted bool
}

// Detection thresholds. The absolute floor stops a very stable site from
// flagging on statistically-large but physically-meaningless wobble.
const (
	MinBaselinePoints = 8
	ZThreshold        = 3.0
	MinAbsoluteRise   = 0.01 // 1 percentage point of bright pixels
	// How far the scene mean may drift from its baseline before the pass is
	// treated as taken under different conditions entirely.
	SceneShiftSigma = 4.0
	// Passes held out between the reference period and the pass being judged.
	ReferenceGap = 2
)

// DetectAnomaly compares the newest observation against a robust baseline
// (median + MAD) built from the preceding ones. Median/MAD rather than
// mean/stdev because SAR series carry occasional wild outliers (weather,
// off-nominal passes) that would inflate a standard deviation and mask real
// change.
//
// Only upward change is flagged: equipment arriving is the signal of
// interest, and a drop usually means the site emptied or the pass was poor.
//
// Crucially the bright-pixel fraction is measured against the scene's own
// mean backscatter first. Rain, harvest, sea state and orbit geometry brighten
// an entire scene, which mechanically pushes pixels past a fixed dB threshold
// and would otherwise flag every site at once. Only brightness that the scene
// mean does NOT explain indicates new metal on the ground.
func DetectAnomaly(series []store.SARObservation) Anomaly {
	if len(series) < MinBaselinePoints+1 {
		return Anomaly{Baseline: max(len(series)-1, 0)}
	}
	sorted := make([]store.SARObservation, len(series))
	copy(sorted, series)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start.Before(sorted[j].Start) })

	latest := sorted[len(sorted)-1]
	// Hold the most recent passes out of the reference period. A sustained
	// change otherwise creeps into its own baseline: three shifted passes in a
	// thirty-point window widen the spread enough to hide the fourth. The gap
	// is dropped when the series is too short to afford it.
	baseline := sorted[:len(sorted)-1]
	if gapped := len(sorted) - 1 - ReferenceGap; gapped >= MinBaselinePoints {
		baseline = sorted[:gapped]
	}

	values := make([]float64, len(baseline))
	for i, o := range baseline {
		values[i] = o.BrightFraction
	}
	med := median(values)

	a := Anomaly{
		Latest:   latest.BrightFraction,
		Median:   med,
		Baseline: len(baseline),
	}

	// Residual against the scene mean: how much brighter is this pass than the
	// site's own bright-vs-mean relationship predicts?
	slope, intercept, ok := fitBrightVsMean(baseline)
	residuals := make([]float64, len(baseline))
	var latestSignal float64
	if ok {
		for i, o := range baseline {
			residuals[i] = o.BrightFraction - (slope*o.MeanDB + intercept)
		}
		latestSignal = latest.BrightFraction - (slope*latest.MeanDB + intercept)
		a.SceneAdjusted = true
	} else {
		// No usable dB variation to regress against; fall back to the raw
		// fraction, which is the pre-adjustment behaviour.
		copy(residuals, values)
		for i := range residuals {
			residuals[i] -= med
		}
		latestSignal = latest.BrightFraction - med
	}

	centre := median(residuals)
	spread := make([]float64, len(residuals))
	for i, r := range residuals {
		spread[i] = math.Abs(r - centre)
	}
	mad := median(spread)

	rise := latestSignal - centre
	// 1.4826 rescales MAD to be a consistent estimator of sigma for normal data.
	if sigma := 1.4826 * mad; sigma > 1e-9 {
		a.ZScore = rise / sigma
	} else if rise > 0 {
		// A perfectly flat baseline: any real rise is infinitely many sigmas,
		// but report a finite number so the UI stays sane.
		a.ZScore = math.Inf(1)
	}
	a.Detected = rise >= MinAbsoluteRise && a.ZScore >= ZThreshold

	// Guard against extrapolation. When the scene mean itself is far outside
	// its baseline range the bright-fraction relationship cannot be projected
	// there — a fit over a 1 dB spread says nothing about a 3 dB excursion.
	// Region-wide weather does exactly this, and would otherwise light up
	// every site simultaneously.
	if a.SceneAdjusted && sceneOutOfRange(baseline, latest.MeanDB) {
		a.SceneShifted = true
		a.Detected = false
	}
	return a
}

// sceneOutOfRange reports whether the latest scene mean sits far outside the
// spread of the baseline's own scene means.
func sceneOutOfRange(baseline []store.SARObservation, latestDB float64) bool {
	dbs := make([]float64, len(baseline))
	for i, o := range baseline {
		dbs[i] = o.MeanDB
	}
	centre := median(dbs)
	dev := make([]float64, len(dbs))
	for i, v := range dbs {
		dev[i] = math.Abs(v - centre)
	}
	sigma := 1.4826 * median(dev)
	if sigma < 1e-9 {
		// A perfectly stable scene: any material shift is out of range.
		return math.Abs(latestDB-centre) > 1.0
	}
	return math.Abs(latestDB-centre)/sigma > SceneShiftSigma
}

// fitBrightVsMean does an ordinary least-squares fit of bright fraction
// against scene mean backscatter. Reports ok=false when the dB values barely
// vary, since the slope would then be meaningless.
func fitBrightVsMean(obs []store.SARObservation) (slope, intercept float64, ok bool) {
	n := float64(len(obs))
	if n < 4 {
		return 0, 0, false
	}
	var sumX, sumY float64
	for _, o := range obs {
		sumX += o.MeanDB
		sumY += o.BrightFraction
	}
	meanX, meanY := sumX/n, sumY/n

	var sxx, sxy float64
	for _, o := range obs {
		dx := o.MeanDB - meanX
		sxx += dx * dx
		sxy += dx * (o.BrightFraction - meanY)
	}
	// Require real spread in dB before trusting a regression on it.
	if sxx < 0.5 {
		return 0, 0, false
	}
	slope = sxy / sxx
	return slope, meanY - slope*meanX, true
}

func median(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := make([]float64, len(v))
	copy(s, v)
	sort.Float64s(s)
	mid := len(s) / 2
	if len(s)%2 == 1 {
		return s[mid]
	}
	return (s[mid-1] + s[mid]) / 2
}
