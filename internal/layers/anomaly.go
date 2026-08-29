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
}

// Detection thresholds. The absolute floor stops a very stable site from
// flagging on statistically-large but physically-meaningless wobble.
const (
	MinBaselinePoints = 8
	ZThreshold        = 3.0
	MinAbsoluteRise   = 0.01 // 1 percentage point of bright pixels
)

// DetectAnomaly compares the newest observation against a robust baseline
// (median + MAD) built from the preceding ones. Median/MAD rather than
// mean/stdev because SAR series carry occasional wild outliers (weather,
// off-nominal passes) that would inflate a standard deviation and mask real
// change.
//
// Only upward change is flagged: equipment arriving is the signal of
// interest, and a drop usually means the site emptied or the pass was poor.
func DetectAnomaly(series []store.SARObservation) Anomaly {
	if len(series) < MinBaselinePoints+1 {
		return Anomaly{Baseline: max(len(series)-1, 0)}
	}
	sorted := make([]store.SARObservation, len(series))
	copy(sorted, series)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start.Before(sorted[j].Start) })

	latest := sorted[len(sorted)-1]
	baseline := sorted[:len(sorted)-1]

	values := make([]float64, len(baseline))
	for i, o := range baseline {
		values[i] = o.BrightFraction
	}
	med := median(values)

	deviations := make([]float64, len(values))
	for i, v := range values {
		deviations[i] = math.Abs(v - med)
	}
	mad := median(deviations)

	rise := latest.BrightFraction - med
	a := Anomaly{
		Latest:   latest.BrightFraction,
		Median:   med,
		Baseline: len(baseline),
	}
	// 1.4826 rescales MAD to be a consistent estimator of sigma for normal data.
	if sigma := 1.4826 * mad; sigma > 1e-9 {
		a.ZScore = rise / sigma
	} else if rise > 0 {
		// A perfectly flat baseline: any real rise is infinitely many sigmas,
		// but report a finite number so the UI stays sane.
		a.ZScore = math.Inf(1)
	}
	a.Detected = rise >= MinAbsoluteRise && a.ZScore >= ZThreshold
	return a
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
