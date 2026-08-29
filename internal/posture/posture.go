// Package posture turns the incident mix into a single, explainable regional
// reading. The dashboard is a firehose of threat reporting; without an
// explicit balance between adverse and favourable developments it reads as
// uniformly dire even during an ordinary week.
package posture

import (
	"sort"
	"strconv"
)

// Level is an ascending scale — 1 is calmest, 5 is worst. It is deliberately
// NOT DEFCON numbering (which counts down); the word is the meaning and the
// number only shows position on the ladder.
type Level int

const (
	Calm Level = iota + 1
	Watchful
	Elevated
	High
	Severe
)

func (l Level) String() string {
	switch l {
	case Calm:
		return "Calm"
	case Watchful:
		return "Watchful"
	case Elevated:
		return "Elevated"
	case High:
		return "High"
	case Severe:
		return "Severe"
	}
	return "Unknown"
}

// Counts is the incident mix over the reading window, bucketed by tone and
// by the severity of the adverse subset.
type Counts struct {
	Positive int `json:"positive"`
	Neutral  int `json:"neutral"`
	Negative int `json:"negative"`
	// Adverse severity histogram, index 1..5 (index 0 unused).
	NegativeBySeverity [6]int `json:"negative_by_severity"`
	PositiveWeight     int    `json:"-"`
}

// Reading is the published verdict.
type Reading struct {
	Level       Level  `json:"level"`
	LevelName   string `json:"level_name"`
	Headline    string `json:"headline"`
	Explanation string `json:"explanation"`
	Counts      Counts `json:"counts"`
	// Balance is the share of tone-bearing items that are favourable, 0..1.
	Balance float64 `json:"balance"`
	// Context answers "is this week unusual?" against the trailing weekly
	// history. A bare count invites the reader to assume the worst; the
	// comparison is what makes it informative.
	Context string `json:"context"`
	// TypicalWeek is the median adverse count of recent weeks, -1 when there
	// is not yet enough history to say.
	TypicalWeek int `json:"typical_week"`
	// Trend names the direction of travel: "escalating", "steady",
	// "de-escalating", or "" when there is not enough history.
	//
	// This exists because a level that can only ratchet upward eventually
	// stops meaning anything. The US Homeland Security Advisory System died of
	// exactly that — its 2009 review found the national baseline had settled
	// permanently at "guarded" and the two lowest levels were never used once
	// in nine years. The US Drought Monitor avoids it by naming a class that
	// explicitly includes "coming out of" drought, and this is our version:
	// improvement is visible in its own right rather than only as the absence
	// of a rise.
	Trend string `json:"trend"`
}

const (
	TrendEscalating   = "escalating"
	TrendSteady       = "steady"
	TrendDeEscalating = "de-escalating"
)

// WithHistory adds the "is this normal?" comparison. History is adverse counts
// per completed week, oldest first.
func (r Reading) WithHistory(history []int) Reading {
	r.TypicalWeek = -1
	if len(history) < 3 {
		r.Context = "Not enough history yet to say whether this is a typical week."
		return r
	}
	sorted := append([]int(nil), history...)
	sort.Ints(sorted)
	typical := sorted[len(sorted)/2]
	r.TypicalWeek = typical

	switch n := r.Counts.Negative; {
	case typical == 0 && n == 0:
		r.Trend = TrendSteady
		r.Context = "In line with recent weeks, which have also been quiet."
	case n > typical*2 && n-typical >= 3:
		r.Trend = TrendEscalating
		r.Context = "Above the recent norm — a typical week over the last " +
			strconv.Itoa(len(history)) + " weeks saw " + strconv.Itoa(typical) + "."
	case n*2 < typical:
		r.Trend = TrendDeEscalating
		r.Context = "Below the recent norm — a typical week saw " + strconv.Itoa(typical) + "."
	default:
		r.Trend = TrendSteady
		r.Context = "About normal for this region — a typical week over the last " +
			strconv.Itoa(len(history)) + " weeks saw " + strconv.Itoa(typical) + "."
	}
	return r
}

// Rule is one published step of the ladder, so a reader can check the reading
// against the data rather than taking it on trust. ACLED publishes the weight
// vector behind its conflict index; this is the equivalent for ours.
type Rule struct {
	Level     int    `json:"level"`
	LevelName string `json:"level_name"`
	Condition string `json:"condition"`
}

// Rules describes, in order of precedence, exactly how Evaluate decides. It is
// written by hand rather than derived, so any change to Evaluate must be
// mirrored here — TestRulesMatchEvaluate fails if the two drift apart.
func Rules() []Rule {
	return []Rule{
		{int(Severe), Severe.String(), "any adverse event at severity 5"},
		{int(High), High.String(), "any adverse event at severity 4"},
		{int(Elevated), Elevated.String(), "3 or more adverse events at severity 3"},
		{int(Watchful), Watchful.String(), "any adverse event at severity 3, or 5 or more adverse events at any severity"},
		{int(Watchful), Watchful.String(), "at least one adverse event"},
		{int(Calm), Calm.String(), "no adverse events"},
	}
}

// Adjustments are the rules that can move a reading after the ladder has set
// it. Both directions are published for the same reason the ladder is.
func Adjustments() []string {
	return []string{
		"Elevated steps down to Watchful when favourable developments outnumber adverse ones.",
		"High and Severe never step down: a serious event stands on its own regardless of good news elsewhere.",
		"State-controlled outlets are excluded from every count, so adversary messaging cannot move the reading.",
		"Counts are per event, not per article: multiple outlets reporting one incident count once.",
	}
}

// Evaluate derives the level from the adverse mix, letting favourable
// developments pull the reading down — but never below what a genuinely
// serious event justifies. A severity-5 adverse event is not cancelled out by
// a good week elsewhere, which is why the top two levels are set by absolute
// severity rather than by any ratio.
func Evaluate(c Counts) Reading {
	r := Reading{Counts: c}

	toned := c.Positive + c.Negative
	if toned > 0 {
		r.Balance = float64(c.Positive) / float64(toned)
	}

	switch {
	case c.NegativeBySeverity[5] > 0:
		r.Level = Severe
		r.Headline = "Critical adverse event in the last 7 days"
	case c.NegativeBySeverity[4] > 0:
		r.Level = High
		r.Headline = pluralise(c.NegativeBySeverity[4], "serious adverse event", "serious adverse events") + " in the last 7 days"
	case c.NegativeBySeverity[3] >= 3:
		r.Level = Elevated
		r.Headline = "Repeated notable adverse activity"
	case c.NegativeBySeverity[3] > 0 || c.Negative >= 5:
		r.Level = Watchful
		r.Headline = "Ordinary background of hybrid activity"
	case c.Negative > 0:
		r.Level = Watchful
		r.Headline = "Low adverse activity"
	default:
		r.Level = Calm
		r.Headline = "No adverse activity recorded"
	}

	// A clearly favourable week softens the middle of the scale by one step:
	// sustained defensive progress genuinely is a different situation from the
	// same adverse count with nothing going right. The top two levels are
	// immune — an actual serious incident stands on its own.
	if r.Level == Elevated && c.Positive > c.Negative {
		r.Level = Watchful
		r.Headline = "Adverse activity offset by defensive progress"
	}

	r.LevelName = r.Level.String()
	r.Explanation = explain(c)
	return r
}

func explain(c Counts) string {
	if c.Positive+c.Neutral+c.Negative == 0 {
		return "No classified incidents in the last 7 days."
	}
	return pluralise(c.Negative, "adverse development", "adverse developments") + ", " +
		pluralise(c.Positive, "favourable development", "favourable developments") + " and " +
		pluralise(c.Neutral, "neutral report", "neutral reports") + " in the last 7 days."
}

func pluralise(n int, one, many string) string {
	word := many
	if n == 1 {
		word = one
	}
	return strconv.Itoa(n) + " " + word
}
