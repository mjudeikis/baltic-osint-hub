# Competitive landscape

Who else is doing this, where we are genuinely ahead, and where we are behind.
Researched August 2026 by direct URL fetching against canonical addresses.

## How to read this document

Two limits shaped the research and both change how the findings may be used.

**Negative findings are "not found", not "verified absent".** The search budget
was exhausted before the sweep began, so discovery was direct fetching of known
and guessed URLs rather than open search. Findings that depend on nothing
existing are marked **[NF]** throughout. The most consequential of these — that
nobody else runs automated Sentinel-1 change detection over a fixed military
watchlist — is [NF], and **must not be stated as a settled negative in public
methodology copy.** Phrase it as "we are not aware of another"; that claim is
defensible and the absolute one is not.

**The live site was not reachable** from the research environment, so all
findings about this project come from reading the repository rather than from
using the dashboard. Anything about rendered behaviour is inferred from source.

One finding in the original research was wrong and is recorded here for
honesty: it reported that our README links a dead `72stundas.lv` domain. It does
not — the README names the Latvian campaign, and `Preparedness.tsx` links
`sargs.lv/lv/72-stundas`, which is correct. Verify before acting on any single
line of this document.

## The three closest comparators

**Großwald Baltic Sea Security Tracker** —
<https://www.grosswald.org/baltic-sea-security-tracker/>

The nearest match by subject: shadow-fleet interdictions, seabed infrastructure
damage, Kaliningrad electronic warfare, NATO posture. It is a **manually
curated prose ledger** — 31 dated entries from December 2024 to August 2026 —
with no table, no map and no incident API. Their
[editorial methodology](https://www.grosswald.org/editorial-methodology/) states
that nothing is published on an AI system's assertion alone. An open data layer
exists but serves article metadata only, and their terms forbid building
derivative databases. What they have that we do not: human verification before
publication, a named editorial standard, and narrative continuity on a single
incident across months.

**tutka** — <https://tutka.fly.dev/> · <https://github.com/kurkista/tutka>

The most important finding in the research. A live, MIT-licensed, one-person
civic dashboard for Finland and the Nordic-Baltic region publishing **six
domain indices — military tension, hybrid/grey-zone, information environment,
critical infrastructure, social stability, environmental — each as a 0–100
baseline-deviation index over 24h/7d/30d**, plus live AIS and flight maps.
Sources overlap ours heavily: GDELT, AISStream, NASA FIRMS, NCSC-FI, ENISA,
CERT-EU, Meteoalarm. Node + SQLite + MapLibre on a 256MB instance.

This is functionally the same product idea, one country north, already shipping
the explainable index we treat as distinctive. It is the single strongest
argument against our own novelty claims.

Their README defines the index as: every domain answers how far it is from its
own recent normal, scored 0–100 as a two-sided robust deviation against the
metric's own trailing 30 days — the same robust-deviation approach we use for
SAR anomalies, applied to the whole dashboard.

One real distinction survives, and it is the honest way to position against
them. **tutka's indices are signal-derived**: they measure how unusual the
telemetry is (news volume, AIS, fires, grid status) against its own recent
history. **Our posture is event-derived**: individual items are classified,
given a direction and a severity, and counted. Theirs answers "how abnormal is
the data right now"; ours answers "what happened, and which way did it cut".
Those are different questions, and a high tutka reading can be driven purely by
news volume about an event that was favourable to the region. That difference
is worth stating plainly rather than claiming the index idea itself.

**Liveuamap Baltic edition** — <https://baltics.liveuamap.com/>

Human-in-the-loop, but the Baltic edition updates weekly-to-monthly against
6–23 hours for their Ukraine map — thin and effectively unmaintained as a
regional product. Their API is a commercial tier starting at $150/month with a
deliberately degraded free tier.

## Projects sharing our architecture

| Project | Licence | State | What it has that we don't |
|---|---|---|---|
| [pharos-ai](https://github.com/Juliusolsson05/pharos-ai) | AGPL-3.0, 180★ | Active | Postgres + LLM classification + DeckGL; 30 RSS feeds each labelled by **bias and tier**; actor dossiers; per-incident source citations. Ingest layer deliberately closed. |
| [IRONSIGHT](https://github.com/NoblerWorks-HQ/IRONSIGHT) | MIT, 626★ | Active | 50+ feeds, Telegram scraping, adsb.lol, FIRMS. **Keyword filtering only, no LLM classification** — we are ahead here. |
| [nordic-baltic-grayzone-monitor](https://github.com/Irizatto/nordic-baltic-grayzone-monitor) | MIT | Renders mock data | Nothing yet, but aimed at our exact region. |
| [shadow-fleet-tracker-light](https://github.com/FormerLab/shadow-fleet-tracker-light) | MIT, 42★ | Active | 1200+ vessels with cable proximity <10 km, AIS blackout ≥60 min, loitering <0.5 kn ≥20 min — near-identical rules to our sea layer, independently derived. |

## Structured data sources worth ingesting

| Source | Cadence | Access | Notes |
|---|---|---|---|
| [Shadowserver Dashboard](https://dashboard.shadowserver.org/) | Daily, rolling 2 years | Undocumented `?json=1` on every stats page, no auth | `data_set=count_per_population` inverts the country ranking usefully. **Their terms bar scraping — email them first.** |
| [CERT.PL Warning List](https://hole.cert.pl/domains/v2/domains.json) | Continuous | 8 formats, no key, 140k entries | Records carry insert and delete dates, so a daily series back to March 2020 is reconstructible from one download. The best-engineered open feed in the region. |
| [Global Fishing Watch](https://globalfishingwatch.org/our-apis/documentation) | Continuous | Free, 50k req/day, CC BY-NC | AIS gaps, loitering, encounters and Sentinel-1 `sar-presence` are first-class event types. |
| [Finnish Digitraffic AIS](https://meri.digitraffic.fi/api/ais/v1/locations) | Live | **CC BY 4.0, no key, commercial use allowed** | Verified serving live Baltic GeoJSON. |
| [OpenSanctions maritime](https://www.opensanctions.org/datasets/maritime/) | Daily | CSV + JSON | 20,406 vessels from 23 sources. |
| [EUvsDisinfo](https://euvsdisinfo.eu/disinformation-cases/) | Weekly | No public API, but **an MCP server exists by request** | 18,000+ cases with Baltic-language filters. |
| [ACLED Weekly Conflict Index](https://acleddata.com/platform/weekly-conflict-index) | Weekly | Free account | **Publishes its weight vector: 35/25/20/20.** Kinetic violence only. Their content terms bar building a "functional substitute". |
| [gpsjam.org](https://gpsjam.org/) | Daily since 2022-02-14 | Raw CSV at a predictable path | **H3 resolution 4 only** (~22 km edge) — too coarse to attribute within a Baltic state. **No licence statement anywhere on the site.** |

**No machine-readable national cyber statistic exists for the Baltics.**
CERT.LV publishes numbers inside PNG alt-text; NKSC publishes an annual PDF
only. That is why Shadowserver is the right cyber layer rather than any
national CERT feed.

## Institutional monitors publish PDFs, not data

Hybrid CoE, NATO StratCom COE, CSIS and the Wilson Center all publish reports
with no dashboard, database, RSS or API. NATO Baltic Sentry has one press
release and no public dashboard [NF]. **The only free structured Baltic incident
table anywhere is [Wikipedia's](https://en.wikipedia.org/wiki/Baltic_Sea_underwater_infrastructure_incidents)** —
11 rows, CC BY-SA, full MediaWiki API.

Debunk.org in Vilnius, our nearest geographic peer, is a Wix site with no
dashboard, database or API, and has published four posts in all of 2026.

## Corrections to things a reasonable person would assume are live

- **Bellingcat's Ukraine TimeMap is dead data** — 2,517 incidents, last event
  2025-07-09, even though the repo still receives pushes. A historical corpus
  and a UI reference, never a live map.
- **Hamilton 2.0 is down** since roughly 13 August 2026, and had been cut to a
  rolling 30-day window since April 2025.
- **Stanford Internet Observatory is gone.**
- **Conflict Intelligence Team's website is frozen at February 2021** — the live
  operation is Telegram. Note `cit.team` is an unrelated Rostov software firm.
- **MilitaryLand's deployment map died January 2025**, but the site is alive and
  is **CC BY-SA 4.0** — the only cleanly licensed source in the whole
  conflict-mapping cluster.
- **ISW is legally off-limits.** Their fair-use policy forbids incorporating
  their material into other datasets or mapping platforms and bars bulk or API
  redistribution. Cite and link only.
- **Microsoft's free IOC tier was retired 1 August 2026.** Flashpoint's RSS is
  three years stale. Janes and Dataminr give away nothing usable.
- **All six live "cyber attack maps"** (Check Point, Kaspersky, Radware,
  Fortinet, Bitdefender, Digital Attack Map) publish no methodology, sampling
  rate, denominator or definition of "attack". Unusable on a dashboard that
  claims auditability.

## Where we are genuinely ahead

**Automated recurring Sentinel-1 change detection over a fixed adversary
watchlist, with published results. [NF]** Bellingcat's Radar Interference
Tracker is the closest well-known thing and is manual by design. Texty.org.ua
did exactly this over 26 Russian bases in January 2022 — but as a one-off
investigation, not a standing monitor. A GitHub search for Sentinel-1 change
detection returns 84 repos, all environmental or academic. **This is the moat**,
and it is why `docs/sar-detection.md` deserves the effort it gets.

**Explicit implemented refusals.** "Baseline still building", "conditions
changed — comparison suppressed", "not enough history yet". Across the whole
sweep there was one analogue: a single `upstreamUnavailable` flag meaning treat
as no data, not no risk. **Nobody else distinguishes *quiet* from *blind*.**
This is the most credibility-preserving thing in the codebase.

**A public per-source health table.** No comparator shows its own pipeline
failures — not Großwald, not Liveuamap, not tutka, not ACLED.

**Linking national civil-preparedness guidance from a threat dashboard.**
Unique in this set. Everyone else reports the threat and stops.

**Cross-domain coverage in one index.** ACLED is kinetic-only, EuRepoC
cyber-only, EUvsDisinfo narrative-only, Shadowserver telemetry-only. Nobody
joins them — though see the caveat below on what that does and does not make us.

## Claims that do not survive contact with the evidence

These are corrections to our own copy, and they matter more than the wins.

**"Tone separate from severity" is not novel.** GDELT has shipped document tone
since 2015 — the GKG 2.1 codebook defines `V2Tone` on a −100 to +100 scale,
free, keyless, 65 languages, back to 2017. What survives is narrower and should
be stated that way: GDELT's tone is document sentiment with no severity pairing,
no event curation and no downstream index. Ours is **a direction-for-regional-security
judgement, assigned per classified event alongside an independent severity, and
wired into a published reading it can pull downward.** Claim the construction,
not the concept.

**"Explainable posture index" has direct incumbents, and one is more transparent
than we are.** ACLED publishes its weight vector; we publish our counts but not
a weight vector, which is arguably a step behind on auditability. tutka already
ships six baseline-deviation indices for our region. Our real differentiators
are **domain breadth**, **the down-step rule**, **the severity floor at levels
4–5**, and **the refusals** — not the existence of an explainable index.

**"Source-credibility exclusion from our own metric" is rare, not unique.**
[Ground News Blindspot](https://ground.news/rating-system) gates a computed
classification on a factuality label; the
[Iffy Quotient](https://csmr.umich.edu/projects/iffy-quotient/) excludes unrated
sites from its formula entirely and has published a white paper. Neither
excludes *state-controlled* sources specifically. Critically, **NewsGuard, MBFC,
Ad Fontes and the Iffy Index have essentially zero .lt/.lv/.ee/.pl coverage** —
so the mechanism has precedent but the region is genuinely unoccupied, which
means we construct those labels ourselves and carry the reputational risk alone.

The cautionary precedent is the 2018 EUvsDisinfo Dutch mislabelling incident,
which produced legal action and a parliamentary defunding motion. If we keep
this feature, publish the per-source score, the rubric, the date rated and an
appeals route, and make the metric recomputable with the exclusion switched off.

## Since addressed

Five of the gaps below have been closed since this research was done. They are
left in place so the reasoning stays readable, each marked **[fixed]**:
event clustering, the dead `confidence` column, URL state, data export with
CORS, and a published weight vector with a named recovery state. See
[methodology.md](methodology.md) for how each works now.

### Sources added, and the gap this research missed

Eight feeds were added. Three came from the research (`therecord.media`, and
BECID and FACT hub as the regional EDMO hubs). The other five came from
auditing our own feed list against our own taxonomy, which this research did
not do because it was comparing dashboards rather than checking coverage:

**Every one of our 79 sources sat on the southern or eastern shore of the
Baltic.** No Finnish, Swedish, Danish or Norwegian press at all — even though
`undersea-infrastructure` is one of our ten categories, and the cables that
matter run Finland–Estonia (Balticconnector, EstLink) and Sweden–Baltic. Those
incidents are routinely broken by Finnish and Swedish newsrooms hours before
the Baltic press. Added Yle News and SVT, plus Swedish and Finnish terms in the
pre-filter — without those a Swedish-language cable report failed the region
test and was dropped before ever reaching classification.

Also added the Moscow Times, Novaya Gazeta Europe and Meduza's English feed as
exiled independent Russian press, all classified `independent`.

Two things worth recording about the additions:

- **Yle permits RSS reuse for headlines that link back to the original but
  forbids copying article text**, so it runs headlines-only (`HeadlinesOnly()`
  on the fetcher). Classification still works from a headline.
- **BECID's feed is dormant** — newest post 10 June 2026 — so it currently
  yields nothing. Kept so it resumes on its own.

**Shadowserver was not added**, despite being the best open cyber signal for
the region: their terms forbid scraping, and that is their permission to give.

## Where we are behind, worst first

**1. One article equals one incident row. [fixed]** `incidents.raw_item_id` is UNIQUE
against `raw_items`, and de-duplication is URL plus normalised-title hash in a
7-day window. That will not collapse one railway incident reported by LRT,
Delfi, ERR, LSM, Notes from Poland, Reuters and TASS in six languages. **The
posture count therefore tracks media volume, not event count**, and a single
well-covered incident can move the regional reading. For scale: Bellingcat's
TimeMap averages 3.04 sources per incident. Until incidents are clustered,
"12 adverse developments this week" is not a defensible number.

**2. `confidence` is a dead column. [fixed]** `incidents.confidence REAL NOT NULL
DEFAULT 0` exists in `001_init.sql` and is written by `InsertIncident`, but
nothing ever sets it — there is no `Confidence` field on the enrichment verdict,
so every incident is stored as 0.0. Meanwhile
[Airwars](https://airwars.org/about/methodology/) grades every incident on two
orthogonal axes with mechanical thresholds (Confirmed / Fair / Weak / Contested
/ Discounted), and states the grade is a snapshot revised on new information.
[OpenCTI](https://docs.opencti.io/latest/usage/reliability-confidence/)
implements the same idea with NATO Admiralty A–F on the source and 0–100 on the
claim, lowest-confidence-wins.

**3. No historical archive.** We store incidents but never snapshot beliefs, and
there is no revision history. "What did we say on 12 January, and did we retract
it?" is unanswerable. DeepStateMap exposes any of 1,740 snapshots back to
2022-04-03; [UAControlMapBackups](https://github.com/owlmaps/UAControlMapBackups)
commits a daily KMZ complete since 2022.

**4. No timeline scrubbing.** `Timeline.tsx` is a static stacked chart with
7/30/90-day presets — no brush, no click-to-filter, no linkage to map or feed.

**5. No URL state at all. [fixed]** No `pushState`, `useSearchParams` or `location`
references anywhere in `web/src`. No shareable filtered view, no per-incident
permalink, no citable ID.

**6. No data out, and no CORS header on `/api/*` [fixed]** — so the endpoints are public
but unusable from any browser application. We consume gpsjam, FIRMS, OpenSky,
Copernicus, GDELT and aisstream and return nothing.

**7. No LICENSE file.** For a project presented as open source this is a
straightforward defect: nobody can safely fork or contribute. **This is the
owner's decision, not an implementation detail** — it determines whether the
classification work can be reused commercially.

**8. No internationalisation.** English-only across four countries. kriis.ee
serves EN/ET/RU/UK; BECID serves four languages. A preparedness dashboard a
Lithuanian pensioner cannot read does not serve the stated goal.

**9. No evidence archiving.** Telegram, Bluesky and news links rot fastest and
are hardest to re-source.
[bellingcat/auto-archiver](https://github.com/bellingcat/auto-archiver) exists
for exactly this. CIT's practice is to cite the primary source and an archive
capture side by side.

**10. No entity extraction or actor registry.** Joining AIS against
OpenSanctions maritime would upgrade "a vessel loitered" to "a listed
shadow-fleet tanker loitered over a cable".

**11. No public submission or moderation queue.** The prefilled GitHub-issue
link is a good instinct but requires an account and is not per-item.
[Ushahidi](https://github.com/ushahidi/platform) and
[Meedan check-api](https://github.com/meedan/check-api) are the live open
implementations of report → unpublished → reviewer promotes.

**12. No per-entity staleness decay.** UAControlMap states plainly that a unit
was last *sighted* in an area; MilitaryLand's rule is that no information for
three months returns a unit to barracks. Our feed presents an article as equally
true forever.

**13. Maritime depth.** aisstream.io is realtime-only with no history and we
archive nothing, so we can never recompute a gap or reconstruct a track. **Every
day not archiving is permanently lost.**

**14. gpsjam is coarser than it looks and richer than we use.** Res-4 cells are
too coarse to attribute interference within a Baltic state — but the raw CSVs go
back to 2022-02-14 and four years of baseline could be backfilled.

## Two external design ideas worth stealing

**The US Drought Monitor** publishes six classes with numeric percentile
thresholds shown side by side, and defines its lowest class explicitly as "going
into **or coming out of** drought" — a **named recovery state**. That is the
cleanest solution anywhere to the ratchet problem our posture is designed
against. The US Homeland Security Advisory System died of exactly that ratchet:
the 2009 review found the new baseline had become "guarded", and the two lowest
levels were never used once in nine years.

**US NTAS** went further — it is not a standing level at all, but time-boxed
bulletins with expiry dates, currently reading that there are no current
advisories, and empty from May 2023 to June 2025. The default is silence;
staying elevated requires an affirmative act.

**The Dutch DTN** (NCTV) is the closest European analogue to our posture: a
published level always accompanied by a narrative assessment explaining its
inputs.

**External calibration worth publishing: Lloyd's Joint War Committee has not
listed the Baltic Sea or the Gulf of Finland.** The insurance market, which
prices this risk with real money, does not treat the region as a war-risk area.
That is exactly the kind of fact a dashboard designed against alarmism should
surface.

## What we should not build

- **A GPS jamming map** — gpsjam.org exists, is free and is daily since 2022.
  Ingest and link out.
- **A Ukraine frontline map** — DeepStateMap and UAControlMap both do it better;
  ISW does it daily but is legally un-ingestible.
- **A disinformation case database** — EUvsDisinfo has 18,000+ human-curated
  cases with Baltic-language filters.
- **Cyber-incident coding** — EuRepoC codes 60 variables per incident with
  expert review. Link it, and note fairly that its bulk data is stale to March
  2025.
- **Any live "cyber attack map"** — numbers whose provenance cannot be cited are
  a liability on a politically-read dashboard.
- **A sanctioned-vessel list** — OpenSanctions maritime is 20,406 vessels from
  23 sources, updated daily. Join to it.
- **Submarine cable geometry** — submarinecablemap.com serves live GeoJSON with
  no key.
- **A general-purpose media credibility rating** — scope our labels to the
  Baltic/Polish gap that MBFC and Iffy do not cover, and no wider.
- **Anything derived from ISW or Großwald** — both licences forbid it.
- **A national alerting system** — kriis.ee, LT72, sargs.lv and RCB do this with
  legal authority. The disclaimer in `Preparedness.tsx` is exactly right.

## Open questions to resolve before building

- **ACLED's current access and content terms.** Material: the README already
  carries `ACLED_EMAIL`/`ACLED_PASSWORD` and a phase-2 fetcher, and their terms
  bar building a "functional substitute".
- **gpsjam.org's licensing.** We already depend on it and the site documents no
  terms of use anywhere.
- **Shadowserver's position on automated collection** — ask before wiring it up.
- **Global Fishing Watch's Baltic tanker gap coverage** — get a token and run one
  bbox query before designing around it.

## Not researched

**The EU preparedness and national-alerting cluster** — krisinformation.se,
72tuntia.fi, the German NINA public-warning API, MeteoAlarm CAP feeds, and
LT72/kriis.ee assessed as products rather than as links. This is the benchmark
most directly aimed at "inform without alarming", and **its absence is a gap in
this analysis, not a finding.**
