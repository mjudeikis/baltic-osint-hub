package sources

import "time"

const (
	fast = 30 * time.Minute
	slow = 6 * time.Hour
)

// All returns every configured fetcher. Feed URLs verified 2026-08-29.
// Sources without public RSS (CERT.LV, NKSC, TVP World) are covered
// indirectly via GDELT.
//
// Deliberately NOT ingested, so the reasons are not rediscovered later:
//   - Shadowserver's daily country telemetry is the best open cyber signal for
//     this region, but their published terms forbid scraping. It needs their
//     permission first, which is not ours to assume.
//   - Großwald's Baltic Sea Security Tracker and ISW both forbid incorporation
//     into other datasets or mapping platforms. Cite and link, never ingest.
//   - Google's threat-intelligence blog serves an HTML shell at its advertised
//     RSS path, not a feed.
func All() []Fetcher {
	feeds := []Fetcher{
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
		// Both ministries quietly abandoned their English feeds (kam.lt/en
		// stopped 2026-08-14, mil.ee/en 2026-07-31) while the native-language
		// ones stayed live; the enricher translates anyway.
		NewRSS("lt-mod", "https://www.kam.lt/feed/", "lt", slow),
		NewRSS("ee-mil", "https://mil.ee/feed/", "et", slow),
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
	return append(feeds, extraSources()...)
}

// extraSources are the second wave: national-language press, security services,
// EU agencies, and targeted Google News queries. Several exist specifically to
// balance the feed — arrests, prosecutions, deployments and procurement are
// news too, and a threat-only source list makes an ordinary week look dire.
// All URLs verified 2026-08-29.
func extraSources() []Fetcher {
	return []Fetcher{
		// --- Security services & national CERTs ---
		NewRSS("lt-vsd", "https://www.vsd.lt/rss/", "lt", slow),
		NewRSS("cert-lv", "https://cert.lv/en/feed/rss/all", "en", slow),
		NewRSS("ee-ria", "https://www.ria.ee/rss-feeds/rss.xml", "et", slow),

		// --- Government / defence (heavily favourable-leaning) ---
		NewRSS("ee-mod", "https://www.kaitseministeerium.ee/en/rss-feeds/rss.xml", "en", slow),
		NewRSS("ee-gov", "https://www.valitsus.ee/en/rss-feeds/rss.xml", "en", slow),
		NewRSS("ee-mfa", "https://vm.ee/en/rss-feeds/rss.xml", "en", slow),
		NewRSS("lv-mod-lv", "https://www.mod.gov.lv/lv/rss.xml", "lv", slow),

		// --- EU agencies: interdictions, border ops, sanctions ---
		NewRSS("frontex", "https://frontex.europa.eu/media-centre/news/news-release/feed", "en", slow),
		NewRSS("europol", "https://www.europol.europa.eu/rss.xml", "en", slow),
		NewRSS("ec-press", "https://ec.europa.eu/commission/presscorner/api/rss?language=en", "en", slow),

		// --- National-language press (catches what the English editions omit) ---
		NewRSS("err-et", "https://www.err.ee/rss", "et", fast),
		NewRSS("err-ru", "https://rus.err.ee/rss", "ru", fast),
		NewRSS("postimees", "https://news.postimees.ee/rss", "et", fast),
		NewRSS("delfi-lv", "https://www.delfi.lv/rss.xml", "lv", fast),
		NewRSS("lsm-lv", "https://www.lsm.lv/rss/", "lv", fast),
		NewRSS("diena-lv", "https://www.diena.lv/rss", "lv", slow),
		NewRSS("15min-lt", "https://www.15min.lt/rss", "lt", fast),
		// Delfi retired the old /rss/feeds/*.xml endpoints (frozen at
		// 2026-08-12, still served with a 200); v2 channel feeds replaced them.
		NewRSS("delfi-lt", "https://feed.delfi.lt/v2/channel/global-lt?format=rss", "lt", fast),
		NewRSS("defence24-pl", "https://defence24.pl/rss", "pl", fast),
		NewRSS("tvn24", "https://tvn24.pl/najnowsze.xml", "pl", fast),
		NewRSS("rzeczpospolita", "https://www.rp.pl/rss/1019", "pl", slow),
		NewRSS("bnn", "https://bnn-news.com/feed", "en", slow),

		// --- Research & investigation ---
		NewRSS("osw-warsaw", "https://www.osw.waw.pl/en/rss.xml", "en", slow),
		NewRSS("bellingcat", "https://www.bellingcat.com/feed/", "en", slow),
		NewRSS("disinfolab", "https://www.disinfo.eu/feed/", "en", slow),

		// --- Regional fact-checking hubs (EDMO) ---
		// BECID is the Baltic EDMO hub (Tartu, Re:Baltica, Delfi EE/LT) and
		// FACT hub also covers LT/LV/EE. Both publish analysis rather than
		// data, so they are ingested as reporting; the classifier drops the
		// training-and-events items that make up much of the feed.
		//
		// BECID's newest post when this was added was 10 June 2026, so it
		// yields nothing under the 14-day ingest window — that is the feed
		// being dormant, not a fetcher bug. Kept so it resumes on its own if
		// they start publishing again. The regional disinfo-analysis
		// ecosystem is generally quiet: Debunk.org managed four posts in all
		// of 2026.
		NewRSS("becid", "https://becid.ut.ee/feed/", "en", slow),
		NewRSS("fact-hub", "https://fact-hub.eu/feed/", "en", slow),

		// --- Cyber reporting ---
		// Recorded Future's newsroom, one of the very few threat-intelligence
		// outlets with an open feed — most of that industry publishes nothing
		// machine-readable without a sales conversation. Carries only a handful
		// of items per fetch, so it is polled on the fast interval.
		NewRSS("the-record", "https://therecord.media/feed", "en", fast),

		// --- Maritime: cable incidents and shadow-fleet reporting ---
		NewRSS("maritime-exec", "https://maritime-executive.com/articles.rss", "en", slow),

		// --- Nordic press: the Baltic Sea half of the watch ---
		// Undersea infrastructure is one of our ten categories, but until these
		// were added every source sat on the southern and eastern shore. The
		// cables that matter run Finland–Estonia (Balticconnector, EstLink) and
		// Sweden–Baltic, and those incidents are routinely broken by Finnish
		// and Swedish newsrooms hours before the Baltic press picks them up.
		// Neither country is in our monitored set; they are here because what
		// happens in their waters lands on LT/LV/EE/PL.
		//
		// Yle permits RSS reuse for headlines that link back to the original
		// but forbids copying article text, so it is headlines-only.
		NewRSS("yle-news", "https://feeds.yle.fi/uutiset/v1/recent.rss?publisherIds=YLE_NEWS", "en", fast).HeadlinesOnly(),
		NewRSS("svt-nyheter", "https://www.svt.se/rss.xml", "sv", fast),

		// --- Independent Russian-language press in exile ---
		// Classified independent, not state-controlled: the distinction is the
		// whole point of the credibility field. They cover Russian domestic
		// decision-making that state wires present only after the fact.
		NewRSS("moscow-times", "https://www.themoscowtimes.com/rss/news", "en", fast),
		NewRSS("novaya-europe", "https://novayagazeta.eu/feed/rss", "ru", slow),
		NewRSS("meduza-en", "https://meduza.io/rss/en/all", "en", fast),

		// --- Energy infrastructure ---
		NewRSS("elering", "https://elering.ee/rss.xml", "et", slow),

		// --- Targeted Google News queries. These are the main lever for a
		// balanced feed: two of the four exist to surface interdictions and
		// reinforcement, which generic threat feeds under-report.
		NewGoogleNews("gnews-arrests", "Baltic OR Lithuania OR Latvia OR Estonia OR Poland saboteur OR spy arrested OR charged OR convicted", "en", "US:en"),
		NewGoogleNews("gnews-nato", "NATO Baltic deployment OR reinforcement OR air defence", "en", "US:en"),
		NewGoogleNews("gnews-cable", "Baltic Sea cable OR pipeline damage vessel", "en", "US:en"),
		NewGoogleNews("gnews-lt", "sabotažas OR diversija OR šnipinėjimas", "lt", "LT:lt"),

		// --- Russian and Belarusian state media ---
		// The official layer. Telegram already gives us the propaganda and the
		// exile-press layers, but formal signalling — exercise announcements,
		// policy statements, ministry releases — happens on the wires. Every
		// one of these is classified state-controlled: monitored as evidence
		// of messaging, marked in the UI, and excluded from the posture
		// reading so an adversary cannot move our own threat gauge.
		NewRSS("tass-en", "https://tass.com/rss/v2.xml", "en", fast),
		NewRSS("tass-ru", "https://tass.ru/rss/v2.xml", "ru", fast),
		NewRSS("ria", "https://ria.ru/export/rss2/index.xml", "ru", fast),
		NewRSS("interfax", "https://www.interfax.ru/rss.asp", "ru", fast),
		NewRSS("kommersant", "https://www.kommersant.ru/RSS/news.xml", "ru", slow),
		NewRSS("zvezda", "https://tvzvezda.ru/export/rss.xml", "ru", slow),
		NewRSS("belta", "https://www.belta.by/rss", "ru", slow),

		// Kaliningrad regional press publishes no reachable feed (klops,
		// newkaliningrad and rugrad all refuse automated clients), so the
		// exclave's local reporting is covered by a scoped news query instead.
		NewGoogleNews("gnews-kaliningrad", "Калининград учения OR военные OR полигон OR железная дорога", "ru", "RU:ru"),

		// --- Telegram: Belarus monitoring and Russian investigative outlets ---
		NewTelegram("zerkalo_io", "ru"),
		NewTelegram("belarusian_silovik", "ru"),
		NewTelegram("belamova", "ru"),
		NewTelegram("mediazzzona", "ru"),
		NewTelegram("theinsider", "ru"),
		NewTelegram("LRT_lt", "lt"),
		// Kremlin-aligned outlet targeting Baltic audiences — monitored to see
		// the narrative aimed at the region, never as reporting.
		NewTelegram("baltnews", "ru"),
	}
}
