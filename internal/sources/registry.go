package sources

import "time"

const (
	fast = 30 * time.Minute
	slow = 6 * time.Hour
)

// All returns every configured fetcher. Feed URLs verified 2026-08-29.
// Sources without public RSS (CERT.LV, NKSC, TVP World) are covered
// indirectly via GDELT.
func All() []Fetcher {
	return []Fetcher{
		// Tier 2 — national English-language news.
		NewRSS("lrt-en", "https://www.lrt.lt/en/news-in-english?rss", "en", fast),
		NewRSS("err-news", "https://news.err.ee/rss", "en", fast),
		NewRSS("lsm-en", "https://eng.lsm.lv/rss/", "en", fast),
		NewRSS("notes-from-poland", "https://notesfrompoland.com/feed/", "en", fast),

		// Tier 1 — structured / institutional.
		&GDELTFetcher{},
		NewRSS("euvsdisinfo", "https://euvsdisinfo.eu/feed/", "en", slow),
		NewRSS("cert-pl", "https://cert.pl/en/rss.xml", "en", slow),

		// Tier 3 — think tanks and government.
		NewRSS("cepa", "https://cepa.org/feed/", "en", slow),
		NewRSS("jamestown", "https://jamestown.org/feed/", "en", slow),
		NewRSS("icds", "https://icds.ee/en/feed/", "en", slow),
		NewRSS("warsaw-institute", "https://warsawinstitute.org/feed/", "en", slow),
		NewRSS("lt-mod", "https://kam.lt/en/feed/", "en", slow),
	}
}
