package sources

import (
	"net/url"
	"time"
)

// NewGoogleNews builds an RSS fetcher over a Google News topical search. It is
// the cheapest way to cover a specific angle across the whole press at once —
// notably the favourable side (arrests, prosecutions, deployments), which
// threat-oriented feeds systematically under-report.
//
// hl is the interface language and ceid the "COUNTRY:lang" edition; together
// they decide which national press the query actually reaches.
func NewGoogleNews(name, query, hl, ceid string) *RSSFetcher {
	q := url.Values{
		"q":    {query},
		"hl":   {hl},
		"gl":   {ceid[:2]},
		"ceid": {ceid},
	}
	feed := "https://news.google.com/rss/search?" + q.Encode()
	f := NewRSS(name, feed, hl, fast)
	// Google News aggregates the same story from many outlets; a shorter
	// horizon keeps repeats down without losing anything current.
	f.maxAge = 7 * 24 * time.Hour
	return f
}
