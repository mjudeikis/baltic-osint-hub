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
		NewRSS("ee-mil", "https://mil.ee/en/feed/", "en", slow),
		NewRSS("lv-mod", "https://www.mod.gov.lv/lv/rss.xml", "lv", slow),

		// Telegram — public channels commonly monitored for Russia/Belarus
		// OSINT, read via the t.me/s web preview. mod_russia and rybar are
		// monitored as primary sources of Russian military/propaganda
		// messaging, not as trusted reporting; classification handles framing.
		NewTelegram("meduzalive", "ru"),
		NewTelegram("astrapress", "ru"),
		NewTelegram("nexta_live", "ru"),
		NewTelegram("rybar", "ru"),
		NewTelegram("mod_russia", "ru"),
		// Military-movement tracking: Belarus monitoring (MotolkoHelp), the
		// Belarusian railway workers' community that reports military rail
		// echelons (belzhd_live), and a Rybar-adjacent RU military channel.
		NewTelegram("MotolkoHelp", "ru"),
		NewTelegram("belzhd_live", "ru"),
		NewTelegram("milinfolive", "ru"),

		// Reddit — regional subreddits via their public RSS listings,
		// serialized with a gap so the shared IP isn't rate-limited.
		NewRSS("reddit:r/BalticStates", "https://www.reddit.com/r/BalticStates/new/.rss", "en", time.Hour).Throttled("reddit"),
		NewRSS("reddit:r/lithuania", "https://www.reddit.com/r/lithuania/new/.rss", "en", time.Hour).Throttled("reddit"),
		NewRSS("reddit:r/latvia", "https://www.reddit.com/r/latvia/new/.rss", "en", time.Hour).Throttled("reddit"),
		NewRSS("reddit:r/Eesti", "https://www.reddit.com/r/Eesti/new/.rss", "en", time.Hour).Throttled("reddit"),
		NewRSS("reddit:r/poland", "https://www.reddit.com/r/poland/new/.rss", "en", time.Hour).Throttled("reddit"),

		// Bluesky keyword monitors (open search API).
		NewBluesky("baltic-jamming", `baltic jamming`),
		NewBluesky("baltic-sabotage", `baltic sabotage`),
		NewBluesky("baltic-airspace", `baltic airspace violation`),
	}
}
