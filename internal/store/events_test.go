package store

import (
	"testing"
	"time"
)

func at(day int) time.Time {
	return time.Date(2026, 8, day, 12, 0, 0, 0, time.UTC)
}

func f64(v float64) *float64 { return &v }

func TestAggregateTakesEarliestTimeAndMaxSeverity(t *testing.T) {
	ev := aggregate([]eventMember{
		{source: "lrt-en", tone: "negative", severity: 3, countries: []string{"LT"}, occurredAt: at(12)},
		{source: "err-news", tone: "negative", severity: 4, countries: []string{"LT", "LV"}, occurredAt: at(10)},
	})
	if !ev.OccurredAt.Equal(at(10)) {
		t.Errorf("OccurredAt = %v, want the earliest report %v", ev.OccurredAt, at(10))
	}
	if ev.Severity != 4 {
		t.Errorf("Severity = %d, want the most consequential assessment 4", ev.Severity)
	}
	if len(ev.Countries) != 2 || ev.Countries[0] != "LT" || ev.Countries[1] != "LV" {
		t.Errorf("Countries = %v, want the union [LT LV]", ev.Countries)
	}
	if ev.TotalReports != 2 || ev.SourceCount != 2 {
		t.Errorf("reports=%d sources=%d, want 2 and 2", ev.TotalReports, ev.SourceCount)
	}
}

// State media is kept as evidence but must not corroborate: four Kremlin wires
// repeating one claim is one claim, not four independent confirmations.
func TestAggregateExcludesStateMediaFromCorroboration(t *testing.T) {
	ev := aggregate([]eventMember{
		{source: "lrt-en", tone: "negative", severity: 3, countries: []string{"LT"}, occurredAt: at(10)},
		{source: "tass-en", tone: "positive", severity: 3, countries: []string{"LT"}, occurredAt: at(10), stateRun: true},
		{source: "ria", tone: "positive", severity: 3, countries: []string{"LT"}, occurredAt: at(10), stateRun: true},
		{source: "zvezda", tone: "positive", severity: 3, countries: []string{"LT"}, occurredAt: at(10), stateRun: true},
	})
	if ev.SourceCount != 1 {
		t.Errorf("SourceCount = %d, want 1 independent source", ev.SourceCount)
	}
	if ev.TotalReports != 4 {
		t.Errorf("TotalReports = %d, want all 4 kept as evidence", ev.TotalReports)
	}
	// Three state outlets outvote one independent one only if we let them.
	if ev.Tone != "negative" {
		t.Errorf("Tone = %q, want negative — state media must not carry the tone", ev.Tone)
	}
	if ev.Confidence != 0.5 {
		t.Errorf("Confidence = %v, want single-source 0.5", ev.Confidence)
	}
}

func TestAggregateStateMediaOnlyEventKeepsItsToneButScoresLow(t *testing.T) {
	ev := aggregate([]eventMember{
		{source: "tass-en", tone: "negative", severity: 2, countries: []string{"EE"}, occurredAt: at(10), stateRun: true},
	})
	if ev.SourceCount != 0 {
		t.Errorf("SourceCount = %d, want 0", ev.SourceCount)
	}
	if ev.Tone != "negative" {
		t.Errorf("Tone = %q, want the member's own tone when nothing else exists", ev.Tone)
	}
	if ev.Confidence != 0.15 {
		t.Errorf("Confidence = %v, want 0.15", ev.Confidence)
	}
}

func TestAggregateToneIsMajorityOfIndependentMembers(t *testing.T) {
	ev := aggregate([]eventMember{
		{source: "a", tone: "positive", severity: 2, countries: []string{"PL"}, occurredAt: at(10)},
		{source: "b", tone: "positive", severity: 2, countries: []string{"PL"}, occurredAt: at(11)},
		{source: "c", tone: "negative", severity: 2, countries: []string{"PL"}, occurredAt: at(12)},
	})
	if ev.Tone != "positive" {
		t.Errorf("Tone = %q, want positive (2 of 3)", ev.Tone)
	}
	if ev.Confidence != 0.95 {
		t.Errorf("Confidence = %v, want 0.95 for 3 independent sources", ev.Confidence)
	}
}

// The representative summary and pin should come from an independent outlet
// that actually geocoded the event, not from whichever row sorted first.
func TestAggregatePrefersLocatedIndependentMember(t *testing.T) {
	ev := aggregate([]eventMember{
		{source: "tass-en", tone: "negative", severity: 2, countries: []string{"LV"}, occurredAt: at(10),
			stateRun: true, place: "Moscow", summary: "state framing", lat: f64(55.7), lon: f64(37.6)},
		{source: "lsm-en", tone: "negative", severity: 2, countries: []string{"LV"}, occurredAt: at(11),
			summary: "no location"},
		{source: "err-news", tone: "negative", severity: 2, countries: []string{"LV"}, occurredAt: at(12),
			place: "Ventspils", summary: "located report", lat: f64(57.39), lon: f64(21.56)},
	})
	if ev.Place != "Ventspils" || ev.SummaryEN != "located report" {
		t.Errorf("representative = %q/%q, want the located independent report", ev.Place, ev.SummaryEN)
	}
	if ev.Lat == nil || *ev.Lat != 57.39 {
		t.Errorf("Lat = %v, want 57.39 — never the state outlet's Moscow pin", ev.Lat)
	}
}

func TestConfidenceLabel(t *testing.T) {
	cases := map[int]string{0: "state media only", 1: "single source", 2: "corroborated", 5: "corroborated"}
	for n, want := range cases {
		if got := ConfidenceLabel(n); got != want {
			t.Errorf("ConfidenceLabel(%d) = %q, want %q", n, got, want)
		}
	}
}
