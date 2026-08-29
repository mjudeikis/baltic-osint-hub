package sources

import (
	"context"
	"fmt"
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
}

func NewRSS(name, url, lang string, interval time.Duration) *RSSFetcher {
	return &RSSFetcher{name: name, url: url, lang: lang, interval: interval, maxAge: 14 * 24 * time.Hour}
}

func (f *RSSFetcher) Name() string            { return f.name }
func (f *RSSFetcher) Interval() time.Duration { return f.interval }

func (f *RSSFetcher) Fetch(ctx context.Context) ([]store.RawItem, error) {
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
