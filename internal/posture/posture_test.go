package posture

import (
	"strings"
	"testing"
)

func sev(counts ...int) [6]int {
	var s [6]int
	for i, n := range counts {
		if i+1 <= 5 {
			s[i+1] = n
		}
	}
	return s
}

func TestCriticalEventDominates(t *testing.T) {
	// A severity-5 adverse event outranks any amount of good news.
	r := Evaluate(Counts{
		Positive: 40, Negative: 1,
		NegativeBySeverity: sev(0, 0, 0, 0, 1),
	})
	if r.Level != Severe {
		t.Errorf("level = %v (%s), want Severe", r.Level, r.LevelName)
	}
}

func TestSeriousEventNotOffsetByGoodNews(t *testing.T) {
	r := Evaluate(Counts{
		Positive: 30, Negative: 2,
		NegativeBySeverity: sev(0, 0, 0, 2, 0),
	})
	if r.Level != High {
		t.Errorf("level = %v (%s), want High — a serious event must not be softened", r.Level, r.LevelName)
	}
}

func TestGoodWeekSoftensElevated(t *testing.T) {
	// Repeated notable adverse activity would be Elevated, but a week with
	// more favourable than adverse developments steps it down.
	base := Counts{Negative: 4, NegativeBySeverity: sev(0, 0, 4, 0, 0)}

	grim := base
	grim.Positive = 0
	if got := Evaluate(grim); got.Level != Elevated {
		t.Errorf("without good news: level = %s, want Elevated", got.LevelName)
	}

	good := base
	good.Positive = 9
	if got := Evaluate(good); got.Level != Watchful {
		t.Errorf("with good news: level = %s, want Watchful", got.LevelName)
	}
}

func TestQuietWeekIsCalm(t *testing.T) {
	r := Evaluate(Counts{Positive: 3, Neutral: 5})
	if r.Level != Calm {
		t.Errorf("level = %s, want Calm", r.LevelName)
	}
	if r.Balance != 1.0 {
		t.Errorf("balance = %v, want 1.0 (all tone-bearing items favourable)", r.Balance)
	}
}

func TestBalanceIgnoresNeutral(t *testing.T) {
	// Neutral reporting is the bulk of the feed; letting it drag the balance
	// toward 0.5 would make every week look identical.
	r := Evaluate(Counts{Positive: 3, Negative: 1, Neutral: 96})
	if r.Balance != 0.75 {
		t.Errorf("balance = %v, want 0.75", r.Balance)
	}
}

func TestNoIncidentsIsNotAlarm(t *testing.T) {
	r := Evaluate(Counts{})
	if r.Level != Calm {
		t.Errorf("level = %s, want Calm on empty data", r.LevelName)
	}
	if r.Balance != 0 {
		t.Errorf("balance = %v, want 0 with no tone-bearing items", r.Balance)
	}
	if r.Explanation == "" {
		t.Error("explanation must never be empty")
	}
}

func TestExplanationCounts(t *testing.T) {
	r := Evaluate(Counts{Positive: 1, Negative: 2, Neutral: 3})
	want := "2 adverse developments, 1 favourable development and 3 neutral reports in the last 7 days."
	if r.Explanation != want {
		t.Errorf("explanation = %q, want %q", r.Explanation, want)
	}
}

func TestEveryLevelHasAName(t *testing.T) {
	for l := Calm; l <= Severe; l++ {
		if l.String() == "Unknown" {
			t.Errorf("level %d has no name", l)
		}
	}
}

func TestHistoryContext(t *testing.T) {
	// A quiet-ish week against a busier norm should read as reassuring, not
	// alarming — the whole reason the comparison exists.
	r := Evaluate(Counts{Negative: 2, Positive: 5, NegativeBySeverity: sev(0, 2)}).
		WithHistory([]int{6, 8, 7, 9, 6, 7})
	if r.TypicalWeek != 7 {
		t.Errorf("typical week = %d, want 7", r.TypicalWeek)
	}
	if !contains(r.Context, "Below the recent norm") {
		t.Errorf("context = %q", r.Context)
	}
}

func TestHistoryFlagsUnusualWeek(t *testing.T) {
	r := Evaluate(Counts{Negative: 12, NegativeBySeverity: sev(0, 0, 12)}).
		WithHistory([]int{2, 1, 3, 2, 2, 1})
	if !contains(r.Context, "Above the recent norm") {
		t.Errorf("context = %q", r.Context)
	}
}

func TestHistoryTooShortSaysSo(t *testing.T) {
	r := Evaluate(Counts{Negative: 4, NegativeBySeverity: sev(0, 4)}).WithHistory([]int{3})
	if r.TypicalWeek != -1 || !contains(r.Context, "Not enough history") {
		t.Errorf("context = %q typical = %d", r.Context, r.TypicalWeek)
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

// The published rules are what a reader checks the reading against, so they
// must not drift from what Evaluate actually does. Each case constructs the
// minimal Counts satisfying one rule and asserts the stated level comes back.
func TestRulesMatchEvaluate(t *testing.T) {
	cases := []struct {
		condition string
		counts    Counts
		want      Level
	}{
		{"any adverse event at severity 5",
			Counts{Negative: 1, NegativeBySeverity: sev(0, 0, 0, 0, 1)}, Severe},
		{"any adverse event at severity 4",
			Counts{Negative: 1, NegativeBySeverity: sev(0, 0, 0, 1)}, High},
		{"3 or more adverse events at severity 3",
			Counts{Negative: 3, NegativeBySeverity: sev(0, 0, 3)}, Elevated},
		{"any adverse event at severity 3",
			Counts{Negative: 1, NegativeBySeverity: sev(0, 0, 1)}, Watchful},
		{"5 or more adverse events at any severity",
			Counts{Negative: 5, NegativeBySeverity: sev(5)}, Watchful},
		{"at least one adverse event",
			Counts{Negative: 1, NegativeBySeverity: sev(1)}, Watchful},
		{"no adverse events", Counts{Positive: 4, Neutral: 9}, Calm},
	}
	for _, tc := range cases {
		t.Run(tc.condition, func(t *testing.T) {
			if got := Evaluate(tc.counts); got.Level != tc.want {
				t.Errorf("Evaluate = %s, but Rules() promises %s for %q",
					got.LevelName, tc.want, tc.condition)
			}
		})
	}

	// Every level the ladder can produce must appear in the published rules,
	// or a reader could see a level with no documented cause.
	published := map[string]bool{}
	for _, r := range Rules() {
		published[r.LevelName] = true
	}
	for l := Calm; l <= Severe; l++ {
		if !published[l.String()] {
			t.Errorf("level %s is reachable but not published in Rules()", l)
		}
	}
}

func TestTrendNamesImprovement(t *testing.T) {
	// The point of a de-escalating state: a week that is clearly better than
	// the norm has to be legible as improvement, not merely as a lower number.
	improving := Evaluate(Counts{Negative: 2, NegativeBySeverity: sev(0, 2)}).
		WithHistory([]int{6, 8, 7, 9, 6, 7})
	if improving.Trend != TrendDeEscalating {
		t.Errorf("trend = %q, want %q", improving.Trend, TrendDeEscalating)
	}

	worsening := Evaluate(Counts{Negative: 12, NegativeBySeverity: sev(0, 0, 12)}).
		WithHistory([]int{2, 1, 3, 2, 2, 1})
	if worsening.Trend != TrendEscalating {
		t.Errorf("trend = %q, want %q", worsening.Trend, TrendEscalating)
	}

	ordinary := Evaluate(Counts{Negative: 3, NegativeBySeverity: sev(0, 3)}).
		WithHistory([]int{3, 4, 3, 2, 3, 4})
	if ordinary.Trend != TrendSteady {
		t.Errorf("trend = %q, want %q", ordinary.Trend, TrendSteady)
	}
}

func TestTrendUnknownWithoutHistory(t *testing.T) {
	r := Evaluate(Counts{Negative: 4, NegativeBySeverity: sev(0, 4)}).WithHistory([]int{3})
	if r.Trend != "" {
		t.Errorf("trend = %q, want empty when history is too short to judge", r.Trend)
	}
}
