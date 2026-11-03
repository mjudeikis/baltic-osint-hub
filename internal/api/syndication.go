package api

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/sources"
	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// The RSS and Atom feeds are the same rows as /api/incidents and the file
// exports, for readers who want to be told rather than to check. They differ
// from the exports in three deliberate ways:
//
//   - Only clustered incidents are published. A feed item's identity is its
//     event; an incident the clusterer has not reached has none yet, and
//     giving it a provisional one would make it reappear as a second item an
//     hour later. Publishing waits for the next collector run instead.
//   - A bare feed URL defaults to severity 2 and 50 items, matching what the
//     dashboard shows. Feed readers poll the URL they were given and never
//     add parameters, so the defaults are what most subscribers get.
//   - Descriptions are plain text, never HTML. Headlines and sources are text
//     we did not write, and most readers render a description as markup.

const (
	feedDefaultLimit    = 50
	feedDefaultSeverity = 2
	feedName            = "Baltic OSINT Hub"
)

// feedFilter is filterFrom with the feed defaults applied to whatever the
// caller left unset. Explicit parameters always win, including severity=0.
func feedFilter(r *http.Request) store.IncidentFilter {
	q := r.URL.Query()
	f := filterFrom(r)
	if !q.Has("limit") {
		f.Limit = feedDefaultLimit
	}
	if !q.Has("severity") {
		f.Severity = feedDefaultSeverity
	}
	f.ClusteredOnly = true
	return f
}

// feedMeta is what both formats say about the feed itself.
type feedMeta struct {
	Title    string
	SelfURL  string // the feed's own absolute URL, as requested
	PageURL  string // the dashboard view showing the same filters
	TagHost  string // authority for the tag: URIs that identify items
	Language string
}

func (s *Server) feedMeta(r *http.Request, f store.IncidentFilter) feedMeta {
	title := feedName + " — incidents"
	var scope []string
	for _, v := range []string{f.Country, f.Category, f.Tone} {
		if v != "" {
			scope = append(scope, v)
		}
	}
	if len(scope) > 0 {
		title += " (" + strings.Join(scope, ", ") + ")"
	}
	host := "osintbaltic.com"
	if u, err := url.Parse(s.baseURL); err == nil && u.Host != "" {
		host = u.Hostname()
	}
	return feedMeta{
		Title:    title,
		SelfURL:  s.baseURL + r.URL.RequestURI(),
		PageURL:  s.baseURL + "/" + dashboardQuery(r.URL.Query()),
		TagHost:  host,
		Language: "en",
	}
}

// dashboardQuery maps API filter parameters onto the dashboard's own URL
// state, so a feed's home link opens the view the feed was built from.
func dashboardQuery(q url.Values) string {
	p := url.Values{}
	for _, k := range []string{"days", "country", "category", "tone", "day"} {
		if v := q.Get(k); v != "" {
			p.Set(k, v)
		}
	}
	if v := q.Get("severity"); v != "" {
		p.Set("sev", v)
	}
	if len(p) == 0 {
		return ""
	}
	return "?" + p.Encode()
}

// itemID is the stable identity of a feed item: a tag URI (RFC 4151) on the
// event, so the item keeps its identity as more reports join the event and
// its representative headline changes.
func itemID(host string, o incidentOut) string {
	return "tag:" + host + ",2026:event:" + strconv.FormatInt(*o.EventID, 10)
}

func itemTitle(o incidentOut) string {
	if t := strings.TrimSpace(o.Title); t != "" {
		return t
	}
	if o.Place != "" {
		return o.Category + " — " + o.Place
	}
	return o.Category
}

// itemText is the plain-text body: our English summary, then the facts the
// dashboard shows beside it. State-controlled sources are named as such, as
// they are in the UI, so a reader never takes TASS for a corroborating outlet.
func itemText(o incidentOut) string {
	var b strings.Builder
	b.WriteString(strings.TrimSpace(o.SummaryEN))
	facts := []string{fmt.Sprintf("Severity %d/5", o.Severity), o.Category}
	if len(o.Countries) > 0 {
		facts = append(facts, strings.Join(o.Countries, ", "))
	}
	if o.Place != "" {
		facts = append(facts, o.Place)
	}
	if o.Tone != "" {
		facts = append(facts, "tone: "+o.Tone)
	}
	if o.ConfidenceLabel != "" {
		facts = append(facts, "confidence: "+o.ConfidenceLabel)
	}
	if b.Len() > 0 {
		b.WriteString(" — ")
	}
	b.WriteString(strings.Join(facts, " · "))
	if len(o.Sources) > 0 {
		names := make([]string, 0, len(o.Sources))
		for _, src := range o.Sources {
			if sources.Credibility(src) == sources.CredStateControlled {
				src += " (state-controlled)"
			}
			names = append(names, src)
		}
		fmt.Fprintf(&b, ". Reported by %d: %s.", o.Reports, strings.Join(names, ", "))
	}
	return b.String()
}

func itemLink(base string, o incidentOut) string {
	if o.URL != "" {
		return o.URL
	}
	return base + "/"
}

func itemCategories(o incidentOut) []string {
	return append([]string{o.Category}, o.Countries...)
}

// newest is the latest occurrence in the feed. An empty feed reports now,
// which only means "nothing to show as of this request".
func newest(items []incidentOut) time.Time {
	var t time.Time
	for _, o := range items {
		if o.OccurredAt.After(t) {
			t = o.OccurredAt
		}
	}
	if t.IsZero() {
		t = time.Now()
	}
	return t.UTC()
}

// --- RSS 2.0 ---

type rssDoc struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	AtomNS  string     `xml:"xmlns:atom,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title         string    `xml:"title"`
	Link          string    `xml:"link"`
	Description   string    `xml:"description"`
	Language      string    `xml:"language"`
	LastBuildDate string    `xml:"lastBuildDate"`
	TTL           int       `xml:"ttl"`
	Self          atomLink  `xml:"atom:link"`
	Items         []rssItem `xml:"item"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr,omitempty"`
}

type rssItem struct {
	Title       string   `xml:"title"`
	Link        string   `xml:"link"`
	Description string   `xml:"description"`
	GUID        rssGUID  `xml:"guid"`
	PubDate     string   `xml:"pubDate"`
	Categories  []string `xml:"category"`
}

type rssGUID struct {
	IsPermaLink bool   `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

func renderRSS(m feedMeta, base string, items []incidentOut) ([]byte, error) {
	ch := rssChannel{
		Title: m.Title,
		Link:  m.PageURL,
		Description: "Open-source reporting on hybrid threats against Lithuania, Latvia, " +
			"Estonia and Poland, one item per event, classified and corroboration-graded.",
		Language:      m.Language,
		LastBuildDate: newest(items).Format(time.RFC1123Z),
		// The collector runs hourly; polling faster finds nothing new.
		TTL:   60,
		Self:  atomLink{Href: m.SelfURL, Rel: "self", Type: "application/rss+xml"},
		Items: make([]rssItem, 0, len(items)),
	}
	for _, o := range items {
		ch.Items = append(ch.Items, rssItem{
			Title:       itemTitle(o),
			Link:        itemLink(base, o),
			Description: itemText(o),
			GUID:        rssGUID{Value: itemID(m.TagHost, o)},
			PubDate:     o.OccurredAt.UTC().Format(time.RFC1123Z),
			Categories:  itemCategories(o),
		})
	}
	return marshalXML(rssDoc{Version: "2.0", AtomNS: "http://www.w3.org/2005/Atom", Channel: ch})
}

// --- Atom (RFC 4287) ---

type atomFeed struct {
	XMLName xml.Name    `xml:"http://www.w3.org/2005/Atom feed"`
	Lang    string      `xml:"xml:lang,attr"`
	Title   string      `xml:"title"`
	ID      string      `xml:"id"`
	Updated string      `xml:"updated"`
	Links   []atomLink  `xml:"link"`
	Author  atomPerson  `xml:"author"`
	Entries []atomEntry `xml:"entry"`
}

type atomPerson struct {
	Name string `xml:"name"`
	URI  string `xml:"uri,omitempty"`
}

type atomText struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

type atomCategory struct {
	Term string `xml:"term,attr"`
}

type atomEntry struct {
	Title      atomText       `xml:"title"`
	ID         string         `xml:"id"`
	Updated    string         `xml:"updated"`
	Published  string         `xml:"published"`
	Link       atomLink       `xml:"link"`
	Summary    atomText       `xml:"summary"`
	Categories []atomCategory `xml:"category"`
}

func renderAtom(m feedMeta, base string, items []incidentOut) ([]byte, error) {
	feed := atomFeed{
		Lang:    m.Language,
		Title:   m.Title,
		ID:      m.SelfURL,
		Updated: newest(items).Format(time.RFC3339),
		Links: []atomLink{
			{Href: m.SelfURL, Rel: "self", Type: "application/atom+xml"},
			{Href: m.PageURL, Rel: "alternate", Type: "text/html"},
		},
		Author:  atomPerson{Name: feedName, URI: base + "/"},
		Entries: make([]atomEntry, 0, len(items)),
	}
	for _, o := range items {
		ts := o.OccurredAt.UTC().Format(time.RFC3339)
		cats := make([]atomCategory, 0, 1+len(o.Countries))
		for _, c := range itemCategories(o) {
			cats = append(cats, atomCategory{Term: c})
		}
		feed.Entries = append(feed.Entries, atomEntry{
			Title:      atomText{Type: "text", Value: itemTitle(o)},
			ID:         itemID(m.TagHost, o),
			Updated:    ts,
			Published:  ts,
			Link:       atomLink{Href: itemLink(base, o), Rel: "alternate"},
			Summary:    atomText{Type: "text", Value: itemText(o)},
			Categories: cats,
		})
	}
	return marshalXML(feed)
}

func marshalXML(v any) ([]byte, error) {
	var buf bytes.Buffer
	buf.WriteString(xml.Header)
	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// --- handlers ---

func (s *Server) handleIncidentsRSS(w http.ResponseWriter, r *http.Request) {
	s.serveFeed(w, r, "application/rss+xml; charset=utf-8", renderRSS)
}

func (s *Server) handleIncidentsAtom(w http.ResponseWriter, r *http.Request) {
	s.serveFeed(w, r, "application/atom+xml; charset=utf-8", renderAtom)
}

type feedRenderer func(feedMeta, string, []incidentOut) ([]byte, error)

// serveFeed answers conditional requests with an ETag over the rendered
// body. There is deliberately no Last-Modified: the newest occurrence is not
// when the feed last changed — an event clustered late can carry an older
// date — and a reader trusting If-Modified-Since would never see it.
func (s *Server) serveFeed(w http.ResponseWriter, r *http.Request, contentType string, render feedRenderer) {
	f := feedFilter(r)
	out, err := s.list(r.Context(), f)
	if err != nil {
		s.fail(w, err)
		return
	}
	body, err := render(s.feedMeta(r, f), s.baseURL, out)
	if err != nil {
		s.fail(w, err)
		return
	}
	sum := sha256.Sum256(body)
	etag := `W/"` + hex.EncodeToString(sum[:12]) + `"`

	w.Header().Set("Cache-Control", "public, max-age=60")
	w.Header().Set("ETag", etag)
	allowCORS(w)
	if etagMatches(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	w.Header().Set("Content-Type", contentType)
	if _, err := w.Write(body); err != nil {
		s.log.Error("feed write", "err", err)
	}
}

// etagMatches implements the weak comparison If-None-Match calls for.
func etagMatches(header, etag string) bool {
	want := strings.TrimPrefix(etag, "W/")
	for c := range strings.SplitSeq(header, ",") {
		c = strings.TrimSpace(c)
		if c == "*" || strings.TrimPrefix(c, "W/") == want {
			return true
		}
	}
	return false
}
