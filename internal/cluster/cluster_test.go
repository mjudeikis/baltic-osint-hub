package cluster

import "testing"

func TestCosine(t *testing.T) {
	cases := []struct {
		name string
		a, b []float32
		want float64
	}{
		{"identical", []float32{1, 0, 0}, []float32{1, 0, 0}, 1},
		{"orthogonal", []float32{1, 0}, []float32{0, 1}, 0},
		{"opposite", []float32{1, 0}, []float32{-1, 0}, -1},
		{"scale invariant", []float32{3, 4}, []float32{6, 8}, 1},
		{"length mismatch", []float32{1, 0}, []float32{1, 0, 0}, 0},
		{"empty", nil, nil, 0},
		{"zero vector", []float32{0, 0}, []float32{1, 1}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := Cosine(tc.a, tc.b)
			if diff := got - tc.want; diff > 1e-9 || diff < -1e-9 {
				t.Errorf("Cosine = %v, want %v", got, tc.want)
			}
		})
	}
}

// Behavioural tests pass an explicit threshold rather than DefaultThreshold:
// they check what Best does with a threshold, not what the tuned constant
// happens to be. Tying them to the constant made a calibration change look
// like a logic regression.
const testThreshold = 0.8

func TestBestRequiresSharedCountry(t *testing.T) {
	vec := []float32{1, 0}
	// An identical vector must still be refused when the countries disjoin:
	// two rail sabotage reports read alike, but one in Poland and one in
	// Estonia are two events, not one.
	candidates := []Candidate{
		{EventID: 7, Embedding: []float32{1, 0}, Countries: []string{"EE"}},
	}
	if _, _, ok := Best(vec, []string{"PL"}, candidates, testThreshold); ok {
		t.Fatal("matched across disjoint countries")
	}
	if id, _, ok := Best(vec, []string{"EE", "LV"}, candidates, testThreshold); !ok || id != 7 {
		t.Fatalf("expected match on shared country, got id=%d ok=%v", id, ok)
	}
}

func TestBestPicksHighestScore(t *testing.T) {
	vec := []float32{1, 0}
	candidates := []Candidate{
		{EventID: 1, Embedding: []float32{0.9, 0.436}, Countries: []string{"LT"}}, // cos = 0.9
		{EventID: 2, Embedding: []float32{0.99, 0.141}, Countries: []string{"LT"}},
		{EventID: 3, Embedding: []float32{0, 1}, Countries: []string{"LT"}},
	}
	id, score, ok := Best(vec, []string{"LT"}, candidates, testThreshold)
	if !ok || id != 2 {
		t.Fatalf("got id=%d score=%v ok=%v, want id=2", id, score, ok)
	}
}

func TestBestBelowThresholdDeclines(t *testing.T) {
	vec := []float32{1, 0}
	candidates := []Candidate{
		{EventID: 1, Embedding: []float32{0.6, 0.8}, Countries: []string{"LT"}}, // cos = 0.6
	}
	id, score, ok := Best(vec, []string{"LT"}, candidates, testThreshold)
	if ok {
		t.Fatalf("matched at %v, below threshold %v", score, testThreshold)
	}
	if id != 0 {
		t.Errorf("expected no event id, got %d", id)
	}
}

// A new incident with no embedding must never join an event: silently
// clustering on a zero vector would merge unrelated reports.
func TestBestWithoutEmbedding(t *testing.T) {
	if _, _, ok := Best(nil, []string{"LT"}, []Candidate{
		{EventID: 1, Embedding: []float32{1, 0}, Countries: []string{"LT"}},
	}, testThreshold); ok {
		t.Fatal("clustered an incident that has no embedding")
	}
}
