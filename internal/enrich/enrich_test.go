package enrich

import "testing"

func TestPassesPrefilter(t *testing.T) {
	cases := []struct {
		source, title string
		want          bool
	}{
		{"lrt-en", "GPS jamming disrupts flights over Lithuania", true},
		{"lrt-en", "Vilnius marathon draws record crowd", false},
		{"err-news", "Estonia expels Russian diplomat over espionage", true},
		{"err-news", "Estonian film wins festival award", false},
		{"euvsdisinfo", "Weekly disinfo review", true}, // always-relevant source
		{"notes-from-poland", "Drone incursion closes Warsaw airport", true},
		{"gdelt:example.com", "Undersea cable damaged in Baltic Sea", true},
	}
	for _, c := range cases {
		if got := PassesPrefilter(c.source, c.title, ""); got != c.want {
			t.Errorf("PassesPrefilter(%q, %q) = %v, want %v", c.source, c.title, got, c.want)
		}
	}
}

func TestParseVerdicts(t *testing.T) {
	text := "Here are the results:\n```json\n" +
		`[{"id": 1, "relevant": true, "category": "gps-jamming", "countries": ["LT"], "severity": 3, "summary": "Jamming reported."},
		  {"id": 2, "relevant": false},
		  {"id": 3, "relevant": true, "category": "bogus", "countries": ["EE"], "severity": 9, "summary": "x"}]` +
		"\n```"
	verdicts, err := parseVerdicts(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(verdicts) != 3 {
		t.Fatalf("got %d verdicts", len(verdicts))
	}
	if !verdicts[0].Relevant || verdicts[0].Category != "gps-jamming" {
		t.Errorf("verdict 0: %+v", verdicts[0])
	}
	if verdicts[1].Relevant {
		t.Errorf("verdict 1 should be irrelevant")
	}
	// Unknown category falls back, severity clamps.
	if verdicts[2].Category != "political" || verdicts[2].Severity != 5 {
		t.Errorf("verdict 2 not sanitized: %+v", verdicts[2])
	}
}

func TestParseVerdictsNoJSON(t *testing.T) {
	if _, err := parseVerdicts("I cannot classify these items."); err == nil {
		t.Fatal("expected error for response without JSON")
	}
}
