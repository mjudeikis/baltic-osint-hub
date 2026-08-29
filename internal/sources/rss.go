package sources

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/mmcdole/gofeed"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// RSSFetcher pulls a single RSS/Atom feed.
type RSSFetcher struct {
	name     string
	url      string
	lang     string
	interval time.Duration
	// maxAge drops items older than this on ingest (protects against
	// backfilling years of archive on first run).
	maxAge time.Duration
	// throttleGroup serializes fetchers sharing the same group with a gap
	// between requests — for hosts that rate-limit per IP (reddit.com).
	throttleGroup string
}

func NewRSS(name, url, lang string, interval time.Duration) *RSSFetcher {
	return &RSSFetcher{name: name, url: url, lang: lang, interval: interval, maxAge: 14 * 24 * time.Hour}
}

// Throttled marks the feed as part of a serialization group.
func (f *RSSFetcher) Throttled(group string) *RSSFetcher {
	f.throttleGroup = group
	return f
}

func (f *RSSFetcher) Name() string            { return f.name }
func (f *RSSFetcher) Interval() time.Duration { return f.interval }

var throttleGroups sync.Map // group name -> *sync.Mutex

func (f *RSSFetcher) Fetch(ctx context.Context) ([]store.RawItem, error) {
	if f.throttleGroup != "" {
		mu, _ := throttleGroups.LoadOrStore(f.throttleGroup, &sync.Mutex{})
		m := mu.(*sync.Mutex)
		m.Lock()
		defer func() {
			time.Sleep(10 * time.Second) // gap before the group's next request
			m.Unlock()
		}()
	}
	parser := gofeed.NewParser()
	parser.Client = HTTPClient
	feed, err := parser.ParseURLWithContext(f.url, ctx)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", f.url, err)
	}
	cutoff := time.Now().Add(-f.maxAge)
	var items []store.RawItem
	for _, entry := range feed.Items {
		if entry.Link == "" || entry.Title == "" {
			continue
		}
		published := entry.PublishedParsed
		if published == nil {
			published = entry.UpdatedParsed
		}
		if published != nil && published.Before(cutoff) {
			continue
		}
		body := entry.Description
		if entry.Content != "" {
			body = entry.Content
		}
		items = append(items, store.RawItem{
			Source:      f.name,
			URL:         entry.Link,
			Title:       entry.Title,
			Body:        Truncate(stripHTML(body), 2000),
			Lang:        f.lang,
			PublishedAt: published,
			ContentHash: ContentHash(entry.Title),
		})
	}
	return items, nil
}
