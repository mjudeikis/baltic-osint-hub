package layers

import (
	"math"
	"testing"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// series builds observations 6 days apart from the given bright fractions.
func series(values ...float64) []store.SARObservation {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	out := make([]store.SARObservation, len(values))
	for i, v := range values {
		out[i] = store.SARObservation{
			Start:          base.AddDate(0, 0, i*6),
			End:            base.AddDate(0, 0, i*6+6),
			BrightFraction: v,
			SampleCount:    100000,
		}
	}
	return out
}

func TestDetectAnomalyFlagsRealRise(t *testing.T) {
	// Stable site around 4% bright pixels, then a jump to 12%.
	a := DetectAnomaly(series(
		0.040, 0.042, 0.038, 0.041, 0.039, 0.043, 0.040, 0.041, 0.039, 0.120,
	))
	if !a.Detected {
		t.Fatalf("expected detection, got %+v", a)
	}
	if a.ZScore < ZThreshold {
		t.Errorf("z = %v, want >= %v", a.ZScore, ZThreshold)
	}
	if a.Baseline != 9 {
		t.Errorf("baseline = %d, want 9", a.Baseline)
	}
}

func TestDetectAnomalyIgnoresNoise(t *testing.T) {
	// Normal wobble must not trip the detector.
	a := DetectAnomaly(series(
		0.040, 0.052, 0.031, 0.047, 0.036, 0.049, 0.033, 0.045, 0.038, 0.048,
	))
	if a.Detected {
		t.Errorf("noise flagged as anomaly: %+v", a)
	}
}

func TestDetectAnomalyIgnoresDrop(t *testing.T) {
	// A site emptying out is not the signal we track.
	a := DetectAnomaly(series(
		0.120, 0.118, 0.122, 0.119, 0.121, 0.117, 0.120, 0.123, 0.119, 0.020,
	))
	if a.Detected {
		t.Errorf("drop flagged as anomaly: %+v", a)
	}
}

func TestDetectAnomalyRequiresAbsoluteRise(t *testing.T) {
	// Statistically huge but physically negligible: a near-constant baseline
	// with a 0.2 percentage-point rise must not flag.
	a := DetectAnomaly(series(
		0.0100, 0.0100, 0.0101, 0.0100, 0.0099, 0.0100, 0.0101, 0.0100, 0.0100, 0.0120,
	))
	if a.Detected {
		t.Errorf("sub-threshold rise flagged: %+v (rise %.4f)", a, a.Latest-a.Median)
	}
}

func TestDetectAnomalyFlatBaselineFiniteHandling(t *testing.T) {
	// Perfectly flat baseline gives an infinite z; the verdict must still be
	// sane and driven by the absolute rise.
	a := DetectAnomaly(series(
		0.05, 0.05, 0.05, 0.05, 0.05, 0.05, 0.05, 0.05, 0.05, 0.09,
	))
	if !a.Detected {
		t.Fatalf("expected detection on flat baseline, got %+v", a)
	}
	if !math.IsInf(a.ZScore, 1) {
		t.Errorf("z = %v, want +Inf for zero-MAD baseline", a.ZScore)
	}
}

func TestDetectAnomalyNeedsEnoughHistory(t *testing.T) {
	a := DetectAnomaly(series(0.04, 0.04, 0.30))
	if a.Detected {
		t.Errorf("flagged with only %d baseline points", a.Baseline)
	}
	if a.Baseline != 2 {
		t.Errorf("baseline = %d, want 2", a.Baseline)
	}
}

func TestDetectAnomalySortsByTime(t *testing.T) {
	s := series(0.04, 0.041, 0.039, 0.042, 0.04, 0.038, 0.041, 0.04, 0.039, 0.15)
	// Shuffle: the newest observation must still be the one evaluated.
	s[0], s[9] = s[9], s[0]
	a := DetectAnomaly(s)
	if !a.Detected || a.Latest != 0.15 {
		t.Errorf("unsorted input mishandled: %+v", a)
	}
}
