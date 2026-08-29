package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// BlueskyFetcher runs a keyword search against the public Bluesky AppView —
// open endpoint, no authentication.
type BlueskyFetcher struct {
	name  string
	query string
}

func NewBluesky(name, query string) *BlueskyFetcher {
	return &BlueskyFetcher{name: name, query: query}
}

func (f *BlueskyFetcher) Name() string            { return "bsky:" + f.name }
func (f *BlueskyFetcher) Interval() time.Duration { return 30 * time.Minute }

type bskyResponse struct {
	Posts []struct {
		URI    string `json:"uri"` // at://did:plc:xxx/app.bsky.feed.post/rkey
		Author struct {
			Handle string `json:"handle"`
		} `json:"author"`
		Record struct {
			Text      string    `json:"text"`
			CreatedAt time.Time `json:"createdAt"`
		} `json:"record"`
	} `json:"posts"`
}

func (f *BlueskyFetcher) Fetch(ctx context.Context) ([]store.RawItem, error) {
	q := url.Values{
		"q":     {f.query},
		"limit": {"50"},
		"sort":  {"latest"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		"https://public.api.bsky.app/xrpc/app.bsky.feed.searchPosts?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("bluesky: status %d", resp.StatusCode)
	}
	var out bskyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("bluesky: decode: %w", err)
	}
	cutoff := time.Now().Add(-48 * time.Hour)
	var items []store.RawItem
	for _, p := range out.Posts {
		rkey := p.URI[strings.LastIndex(p.URI, "/")+1:]
		if rkey == "" || p.Record.Text == "" || p.Record.CreatedAt.Before(cutoff) {
			continue
		}
		created := p.Record.CreatedAt
		title := Truncate(p.Record.Text, 200)
		items = append(items, store.RawItem{
			Source:      f.Name(),
			URL:         fmt.Sprintf("https://bsky.app/profile/%s/post/%s", p.Author.Handle, rkey),
			Title:       "@" + p.Author.Handle + ": " + title,
			Body:        Truncate(p.Record.Text, 2000),
			Lang:        "en",
			PublishedAt: &created,
			ContentHash: ContentHash(p.Author.Handle + " " + p.Record.Text),
		})
	}
	return items, nil
}
