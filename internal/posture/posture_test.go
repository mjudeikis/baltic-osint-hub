package posture

import "testing"

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
