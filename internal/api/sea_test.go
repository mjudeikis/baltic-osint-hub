package api

import (
	"testing"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// A week of live data gave 120 sea events, 4 of them involving a sanctioned
// vessel. Showing all 120 identically buried those 4.
func TestNotable(t *testing.T) {
	cases := []struct {
		name   string
		event  string
		listed bool
		want   bool
	}{
		{"sanctioned vessel loitering over a cable", "loitering", true, true},
		{"sanctioned vessel going dark", "ais-gap", true, true},
		{"unlisted vessel going dark", "ais-gap", false, true},
		{"unlisted vessel merely stopping", "loitering", false, false},
	}
	for _, c := range cases {
		got := notable(store.SeaEvent{Event: c.event}, c.listed)
		if got != c.want {
			t.Errorf("%s: notable = %v, want %v", c.name, got, c.want)
		}
	}
}
