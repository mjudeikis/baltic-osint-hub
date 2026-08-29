package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// GDELTFetcher queries the GDELT DOC 2.0 API (free, no key) for recent
// articles matching region + threat keywords across languages.
type GDELTFetcher struct{}

// GDELT is frequently overloaded: TCP connect alone can take 30s+ and it
// rate-limits to one request per 5s. One patient request per run.
var gdeltClient = &http.Client{
	Timeout: 3 * time.Minute,
	Transport: userAgentTransport{base: &http.Transport{
		DialContext:         (&net.Dialer{Timeout: 90 * time.Second}).DialContext,
		TLSHandshakeTimeout: 60 * time.Second,
	}},
}

func (f *GDELTFetcher) Name() string            { return "gdelt" }
func (f *GDELTFetcher) Interval() time.Duration { return 30 * time.Minute }

// gdeltQuery: region terms AND threat terms. GDELT requires OR-groups to be
// parenthesized. English only — national-language coverage comes from the
// direct RSS feeds, and this keeps the result set deduplicatable by title.
const gdeltQuery = `(Lithuania OR Latvia OR Estonia OR Poland OR Baltic OR Kaliningrad OR Suwalki) (sabotage OR jamming OR GPS OR GNSS OR drone OR airspace OR incursion OR cyberattack OR disinformation OR espionage OR spy OR "hybrid attack" OR "undersea cable" OR arson OR provocation) sourcelang:english`

type gdeltResponse struct {
	Articles []struct {
		URL      string `json:"url"`
		Title    string `json:"title"`
		SeenDate string `json:"seendate"`
		Domain   string `json:"domain"`
		Language string `json:"language"`
	} `json:"articles"`
}

func (f *GDELTFetcher) Fetch(ctx context.Context) ([]store.RawItem, error) {
	q := url.Values{
		"query":      {gdeltQuery},
		"mode":       {"artlist"},
		"format":     {"json"},
		"maxrecords": {"250"},
		"timespan":   {"1d"},
		"sort":       {"datedesc"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://api.gdeltproject.org/api/v2/doc/doc?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := gdeltClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gdelt: status %d", resp.StatusCode)
	}
	var out gdeltResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("gdelt: decode: %w", err)
	}
	var items []store.RawItem
	for _, a := range out.Articles {
		if a.URL == "" || a.Title == "" {
			continue
		}
		var published *time.Time
		if t, err := time.Parse("20060102T150405Z", a.SeenDate); err == nil {
			published = &t
		}
		items = append(items, store.RawItem{
			Source:      "gdelt:" + a.Domain,
			URL:         a.URL,
			Title:       a.Title,
			Lang:        a.Language,
			PublishedAt: published,
			ContentHash: ContentHash(a.Title),
		})
	}
	return items, nil
}
