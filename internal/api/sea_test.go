package api

import (
	"testing"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// A week of live data gave ~500 sea events: 23 involving a listed vessel and
// 326 AIS gaps, median gap 1.6 h — reception dropouts. Showing every gap
// identically buried the listed vessels, so only extended gaps by vessels
// that could damage a cable count as notable.
func TestNotable(t *testing.T) {
	at := func(h float64) *time.Time {
		ts := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC).Add(-time.Duration(h * float64(time.Hour)))
		return &ts
	}
	detected := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)
	typ := func(t int) *int { return &t }

	cases := []struct {
		name     string
		event    string
		started  *time.Time
		shipType *int
		listed   bool
		want     bool
	}{
		{"sanctioned vessel loitering over a cable", "loitering", nil, nil, true, true},
		{"sanctioned vessel going dark, however briefly", "ais-gap", at(1), nil, true, true},
		{"unlisted vessel merely stopping", "loitering", nil, nil, false, false},
		// The median unlisted gap is ~1.6 h at current receiver coverage: a
		// dropout, not dark activity. It stays recorded, but as baseline.
		{"unlisted short gap", "ais-gap", at(1.5), nil, false, false},
		{"unlisted extended gap, unknown type", "ais-gap", at(6), nil, false, true},
		{"unlisted extended gap, cargo", "ais-gap", at(6), typ(70), false, true},
		{"unlisted extended gap, tanker", "ais-gap", at(6), typ(84), false, true},
		// A ferry cannot drag a cable-breaking anchor; going dark for hours is
		// still its problem, not the corridor's.
		{"unlisted extended gap, passenger ferry", "ais-gap", at(6), typ(60), false, false},
		{"unlisted extended gap, fishing", "ais-gap", at(6), typ(30), false, false},
		// A gap with no recorded start cannot be judged for length.
		{"unlisted gap without a start time", "ais-gap", nil, nil, false, false},
	}
	for _, c := range cases {
		e := store.SeaEvent{Event: c.event, DetectedAt: detected, StartedAt: c.started}
		got := notable(e, c.listed, c.shipType)
		if got != c.want {
			t.Errorf("%s: notable = %v, want %v", c.name, got, c.want)
		}
	}
}
