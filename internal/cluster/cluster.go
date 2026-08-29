// Package cluster groups incident reports that describe the same real-world
// event, so counts on the dashboard measure events rather than press coverage.
//
// The existing content-hash dedupe in the store catches only verbatim
// syndication — an identical headline republished by a wire subscriber. It
// cannot match "Lithuania detains man over railway arson" against "Man held
// after rail sabotage attempt in Lithuania", and it has no chance at all
// across Lithuanian, Estonian, Polish and Russian. This package closes that
// gap using the English summary the classifier already produces for every
// item regardless of source language.
package cluster

import (
	"math"
	"slices"
	"time"
)

// Window is how far apart two reports of the same event may be. Regional
// press typically picks up an incident within a day; three days leaves room
// for weekend reporting and for outlets that publish a considered follow-up,
// without letting a recurring category of event (a second jamming episode the
// following week) fold into the first.
const Window = 72 * time.Hour

// DefaultThreshold is the cosine similarity above which two summaries are
// treated as the same event.
//
// This number is MEASURED, not reasoned — see TestCalibrationAgainstRealEmbeddings.
// An earlier version of this constant was set to 0.86 on the argument that a
// high threshold is the safe choice. Running the calibration showed 0.86 would
// have merged nothing whatsoever: genuine duplicate reports of one event score
// 0.68–0.86, so the "safe" value was silently a no-op.
//
// Against text-embedding-3-small at 512 dimensions, with the category and
// shared-country guards applied:
//
//	clear duplicates, independent newsrooms   0.73 – 0.86
//	distinct events the guards let through    up to 0.64
//
// 0.70 sits in that gap with margin on both sides. The failure modes are not
// symmetric — merging too eagerly collapses distinct adverse events and
// publishes a calmer reading than reality, whereas merging too timidly just
// leaves us counting articles as before — so it is placed above the midpoint
// and accepts missing the hardest duplicates.
//
// Override with CLUSTER_THRESHOLD to retune without a rebuild. Re-run the
// calibration test after changing the embedding model or dimension count.
const DefaultThreshold = 0.70

// Candidate is an already-clustered incident that a new one may join.
type Candidate struct {
	EventID   int64
	Embedding []float32
	Countries []string
}

// Cosine returns the cosine similarity of two equal-length vectors, or 0 if
// they are empty or mismatched. OpenAI returns unit-normalised vectors, but
// normalising here costs little and keeps the function correct on any input.
func Cosine(a, b []float32) float64 {
	if len(a) == 0 || len(a) != len(b) {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		x, y := float64(a[i]), float64(b[i])
		dot += x * y
		na += x * x
		nb += y * y
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}

// sharesCountry reports whether two country sets intersect. It is a guard
// against semantic similarity standing in for identity: two arson attacks on
// rail infrastructure in the same week read almost identically in one-sentence
// summaries, and the only thing separating them may be that one is in Poland
// and the other in Estonia.
func sharesCountry(a, b []string) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}

// Best returns the event whose member is most similar to vec, provided the
// similarity clears threshold and the two share at least one country. The
// caller supplies only candidates already narrowed to the same category and
// time window.
func Best(vec []float32, countries []string, candidates []Candidate, threshold float64) (eventID int64, score float64, ok bool) {
	if len(vec) == 0 {
		return 0, 0, false
	}
	for _, c := range candidates {
		if c.EventID == 0 || !sharesCountry(countries, c.Countries) {
			continue
		}
		if s := Cosine(vec, c.Embedding); s > score {
			score, eventID = s, c.EventID
		}
	}
	if score < threshold || eventID == 0 {
		return 0, score, false
	}
	return eventID, score, true
}
