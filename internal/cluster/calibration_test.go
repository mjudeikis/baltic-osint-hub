package cluster_test

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/mjudeikis/baltic-osint-hub/internal/cluster"
	"github.com/mjudeikis/baltic-osint-hub/internal/enrich"
)

// Calibration against real embeddings. The threshold is the one number in this
// package that cannot be reasoned into correctness — it depends entirely on how
// the embedding model spaces real summaries — so this test measures it.
//
// It costs a fraction of a cent and needs a key, so it is opt-in:
//
//	OPENAI_API_KEY=sk-... go test ./internal/cluster/ -run Calibration -v
//
// Run it whenever the embedding model, the dimension count or the threshold
// changes. This test is why DefaultThreshold is not 0.86: that value was
// reasoned rather than measured, and it turned out to merge nothing at all —
// every genuine duplicate pair below scores under it.
//
// Cases are compared the way the pipeline compares them — same category, and
// through Best() so the shared-country guard applies — because the guards are
// part of the decision and calibrating raw cosine without them gives a
// threshold that is wrong in production.
type pair struct {
	name      string
	category  string
	aCountry  []string
	bCountry  []string
	a, b      string
	wantMerge bool
	// sameEvent records that a pair really is one event even where wantMerge
	// is false. Those are the duplicates we knowingly miss at the chosen
	// operating point — under-merging is the safe direction, but it should be
	// visible in the test rather than quietly relabelled as "different".
	sameEvent bool
}

func TestCalibrationAgainstRealEmbeddings(t *testing.T) {
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set; skipping embedding calibration")
	}
	emb := enrich.NewEmbedder(key, os.Getenv("OPENAI_BASE_URL"))

	cases := []pair{
		// --- One event, independent newsrooms. Must merge. ---
		{"same/rail-arson", "sabotage", []string{"LT"}, []string{"LT"},
			"Lithuanian police detained a man suspected of setting fire to railway signalling equipment near Kaunas.",
			"A suspect was arrested in Lithuania after an arson attack damaged rail signalling infrastructure close to Kaunas.", true, true},
		{"same/cable", "undersea-infrastructure", []string{"EE"}, []string{"EE"},
			"An undersea telecommunications cable between Finland and Estonia was damaged, and a vessel is under investigation.",
			"Authorities are investigating a ship after a subsea data cable linking Estonia and Finland was cut.", true, true},
		{"same/airspace", "airspace-border", []string{"EE"}, []string{"EE"},
			"Two Russian military aircraft violated Estonian airspace over the Gulf of Finland for about a minute.",
			"Estonia summoned Russia's envoy after two military jets entered its airspace near the Gulf of Finland.", true, true},
		{"same/espionage", "espionage", []string{"PL"}, []string{"PL"},
			"Polish authorities charged a man with spying for Russian intelligence services in Warsaw.",
			"A man has been indicted in Warsaw on charges of espionage on behalf of Russia.", true, true},
		// KNOWN MISS. A state outlet's denial of an event an independent outlet
		// reported: the same event, but written so differently it scores 0.68,
		// inside the band where distinct events also live. We decline it rather
		// than lower the threshold into that band, so it stays two rows. The
		// cost is a duplicate in the feed; the alternative risks merging
		// genuinely separate adverse events, which would understate the week.
		{"same/state-framing", "sabotage", []string{"LV"}, []string{"LV"},
			"Latvian security services detained a Russian citizen accused of planning an attack on infrastructure.",
			"Moscow denied involvement after Latvia detained a Russian national over an alleged infrastructure plot.", false, true},

		// --- The danger zone: same category, same country, different event.
		// These share every guard, so only the threshold separates them. ---
		{"diff/two-jamming-episodes", "gps-jamming", []string{"LT"}, []string{"LT"},
			"GPS interference disrupted flights approaching Vilnius airport on Tuesday.",
			"Shipping in the Gulf of Riga reported satellite navigation outages lasting several hours.", false, false},
		{"diff/two-lt-arrests", "espionage", []string{"LT"}, []string{"LT"},
			"A Lithuanian citizen was detained on suspicion of passing military information to Russian intelligence.",
			"Lithuania expelled a Russian diplomat accused of activity incompatible with diplomatic status.", false, false},
		{"diff/deployment-vs-sabotage", "military", []string{"LT"}, []string{"LT"},
			"Germany deployed additional air defence systems to Lithuania.",
			"A sabotage attempt damaged a Lithuanian military supply depot.", false, false},

		// --- Distinct events the country guard alone should stop, whatever
		// the cosine says. ---
		{"diff/two-arsons-different-countries", "sabotage", []string{"LT"}, []string{"PL"},
			"A man was detained in Lithuania over an arson attack on a shopping centre.",
			"Polish police detained a man over an arson attack on a paint warehouse in Warsaw.", false, false},
		{"diff/two-drone-incursions", "airspace-border", []string{"EE"}, []string{"LV"},
			"Estonia reported a drone incursion over its eastern border on Monday.",
			"Latvia reported a drone crossing its border from Belarus on Saturday.", false, false},
	}

	texts := make([]string, 0, len(cases)*2)
	for _, c := range cases {
		texts = append(texts, c.a, c.b)
	}
	vecs, err := emb.Embed(context.Background(), texts)
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(vecs[0]) != enrich.EmbedDims {
		t.Errorf("got %d dimensions, want %d", len(vecs[0]), enrich.EmbedDims)
	}

	// Separation is measured only over pairs the guards actually let through,
	// since those are the only ones the threshold has to decide.
	minMerge, maxComparableDistinct := 1.0, 0.0
	for i, c := range cases {
		va, vb := vecs[i*2], vecs[i*2+1]
		score := cluster.Cosine(va, vb)
		_, _, merged := cluster.Best(va, c.aCountry,
			[]cluster.Candidate{{EventID: 1, Embedding: vb, Countries: c.bCountry}},
			cluster.DefaultThreshold)

		guarded := !sharesAny(c.aCountry, c.bCountry)
		note := ""
		if guarded {
			note = " (country guard)"
		}
		if c.sameEvent && !c.wantMerge {
			note = " (known miss: same event, declined)"
		}
		t.Logf("%-36s cos=%.4f merged=%-5v want=%-5v%s", c.name, score, merged, c.wantMerge, note)

		if merged != c.wantMerge {
			t.Errorf("%s: cos=%.4f merged=%v, want %v at threshold %.2f",
				c.name, score, merged, c.wantMerge, cluster.DefaultThreshold)
		}
		if c.wantMerge && score < minMerge {
			minMerge = score
		}
		if !c.wantMerge && !c.sameEvent && !guarded && score > maxComparableDistinct {
			maxComparableDistinct = score
		}
	}

	t.Logf("must-merge floor %.4f; highest distinct pair the guards let through %.4f",
		minMerge, maxComparableDistinct)
	if maxComparableDistinct >= minMerge {
		t.Fatalf("no threshold separates these: comparable distinct pairs reach %.4f, duplicates fall to %.4f",
			maxComparableDistinct, minMerge)
	}
	// Margin on both sides, so a slightly unusual pair does not flip the
	// decision. If this fails the threshold still works but has drifted to an
	// edge of the window and should be recentred.
	const margin = 0.03
	if cluster.DefaultThreshold > minMerge-margin || cluster.DefaultThreshold < maxComparableDistinct+margin {
		t.Errorf("threshold %.2f sits at the edge of the safe window (%.4f .. %.4f)",
			cluster.DefaultThreshold, maxComparableDistinct, minMerge)
	}
}

func sharesAny(a, b []string) bool {
	for _, x := range a {
		if slices.Contains(b, x) {
			return true
		}
	}
	return false
}
