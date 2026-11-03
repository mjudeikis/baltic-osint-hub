package api

import (
	"encoding/xml"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/sources"
	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

func feedFixture() (feedMeta, []incidentOut) {
	ev := int64(42)
	m := feedMeta{
		Title:    "Baltic OSINT Hub — incidents",
		SelfURL:  "https://osintbaltic.com/api/incidents.rss?country=LT",
		PageURL:  "https://osintbaltic.com/?country=LT",
		TagHost:  "osintbaltic.com",
		Language: "en",
	}
	items := []incidentOut{{
		Incident: store.Incident{
			ID:         7,
			EventID:    &ev,
			Category:   "sabotage",
			Countries:  []string{"LT", "LV"},
			Severity:   3,
			Tone:       "negative",
			Place:      "Klaipėda",
			SummaryEN:  "Cable damaged.",
			OccurredAt: time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC),
			Source:     "lrt-en",
			// A hostile headline: markup, an entity and a control character
			// that is not legal anywhere in XML 1.0.
			Title:   `<script>alert(1)</script> & "x"` + "\x01",
			URL:     "https://example.test/a?b=1&c=2",
			Reports: 2,
			Sources: []string{"lrt-en", "tass-en"},
		},
		Credibility:     sources.CredIndependent,
		ConfidenceLabel: "corroborated",
	}}
	return m, items
}

// Both feeds must parse as XML whatever a publisher put in a headline, and
// the hostile text must come back as text, not as elements.
func TestFeedsAreWellFormedAndEscaped(t *testing.T) {
	m, items := feedFixture()

	rss, err := renderRSS(m, "https://osintbaltic.com", items)
	if err != nil {
		t.Fatalf("renderRSS: %v", err)
	}
	var r rssDoc
	if err := xml.Unmarshal(rss, &r); err != nil {
		t.Fatalf("RSS is not well-formed: %v\n%s", err, rss)
	}
	if strings.Contains(string(rss), "<script>") {
		t.Error("RSS carries an unescaped <script> element")
	}
	if len(r.Channel.Items) != 1 {
		t.Fatalf("RSS items = %d, want 1", len(r.Channel.Items))
	}
	it := r.Channel.Items[0]
	if !strings.HasPrefix(it.Title, "<script>") || !strings.HasSuffix(it.Title, "�") {
		t.Errorf("RSS title = %q, want the headline as text with the control char replaced", it.Title)
	}
	if it.GUID.Value != "tag:osintbaltic.com,2026:event:42" || it.GUID.IsPermaLink {
		t.Errorf("RSS guid = %+v, want the event tag URI, not a permalink", it.GUID)
	}
	if it.Link != "https://example.test/a?b=1&c=2" {
		t.Errorf("RSS link = %q", it.Link)
	}
	if it.PubDate != "Mon, 02 Nov 2026 10:00:00 +0000" {
		t.Errorf("RSS pubDate = %q", it.PubDate)
	}
	if !strings.Contains(it.Description, "tass-en (state-controlled)") {
		t.Errorf("RSS description = %q, want the state outlet marked", it.Description)
	}
	if !strings.Contains(it.Description, "confidence: corroborated") {
		t.Errorf("RSS description = %q, want the confidence label", it.Description)
	}

	atom, err := renderAtom(m, "https://osintbaltic.com", items)
	if err != nil {
		t.Fatalf("renderAtom: %v", err)
	}
	var a atomFeed
	if err := xml.Unmarshal(atom, &a); err != nil {
		t.Fatalf("Atom is not well-formed: %v\n%s", err, atom)
	}
	if strings.Contains(string(atom), "<script>") {
		t.Error("Atom carries an unescaped <script> element")
	}
	if len(a.Entries) != 1 || a.Entries[0].ID != it.GUID.Value {
		t.Fatalf("Atom entries = %+v, want one entry sharing the RSS identity", a.Entries)
	}
	if a.Entries[0].Summary.Type != "text" || a.Entries[0].Updated != "2026-11-02T10:00:00Z" {
		t.Errorf("Atom entry = %+v", a.Entries[0])
	}
	if a.ID != m.SelfURL || a.Author.Name == "" {
		t.Errorf("Atom feed id/author = %q/%q, both are required", a.ID, a.Author.Name)
	}
}

func TestEmptyFeedsAreValid(t *testing.T) {
	m, _ := feedFixture()
	for name, render := range map[string]feedRenderer{"rss": renderRSS, "atom": renderAtom} {
		out, err := render(m, "https://osintbaltic.com", nil)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if err := xml.Unmarshal(out, new(struct{})); err != nil {
			t.Errorf("%s empty feed is not well-formed: %v", name, err)
		}
	}
}

// A bare feed URL must look like the dashboard; anything the subscriber
// spelled out, including severity=0, must win.
func TestFeedFilterDefaults(t *testing.T) {
	bare := feedFilter(httptest.NewRequest("GET", "/api/incidents.rss", nil))
	if bare.Severity != 2 || bare.Limit != 50 || !bare.ClusteredOnly {
		t.Errorf("bare filter = %+v, want severity 2, limit 50, clustered only", bare)
	}
	explicit := feedFilter(httptest.NewRequest("GET", "/api/incidents.rss?severity=0&limit=200", nil))
	if explicit.Severity != 0 || explicit.Limit != 200 {
		t.Errorf("explicit filter = %+v, want severity 0, limit 200", explicit)
	}
}

func TestDashboardQuery(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/incidents.atom?country=LT&severity=3&limit=10", nil)
	if got := dashboardQuery(r.URL.Query()); got != "?country=LT&sev=3" {
		t.Errorf("dashboardQuery = %q", got)
	}
}

func TestETagMatches(t *testing.T) {
	tag := `W/"abc"`
	for header, want := range map[string]bool{
		`W/"abc"`:      true,
		`"abc"`:        true,
		`"x", W/"abc"`: true,
		`*`:            true,
		`W/"abd"`:      false,
		``:             false,
	} {
		if got := etagMatches(header, tag); got != want {
			t.Errorf("etagMatches(%q) = %v, want %v", header, got, want)
		}
	}
}
