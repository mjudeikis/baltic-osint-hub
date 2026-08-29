# Methodology

How an item becomes a dashboard reading, and the rules that constrain what the
dashboard is allowed to claim.

## Pipeline

```
sources ──► raw_items ──► pre-filter ──► LLM classification ──► incidents ──► posture
                                              │
signal layers ────────────────────────────────┴──► map overlays (never classified)
```

**Collection** (`internal/sources`) runs hourly. Roughly 80 fetchers: national
press in five languages, government and CERT feeds, EU agencies, research
institutes, GDELT, targeted news queries, public Telegram channels, regional
subreddits. Each source enforces its own minimum interval, so the cron schedule
and upstream rate limits stay independent.

**De-duplication** happens in two stages, because they solve different problems.

*Exact* de-duplication is by URL and then by normalised-title hash within a
7-day window, which catches wire copy republished verbatim by a subscriber.

*Event clustering* (`internal/cluster`) catches what the hash cannot: the same
incident written up independently by six newsrooms in five languages. After
classification, each incident's English summary is embedded
(`text-embedding-3-small`, 512 dimensions) and compared by cosine similarity
against incidents in the same category within ±72 hours. Above the threshold —
and only if the two share at least one affected country — they are merged into
one `events` row. **The classifier writing every summary in English regardless
of source language is what makes this possible at all**: an Estonian and a
Russian report of one event arrive at the clustering stage as two English
sentences.

Similarity runs in Go over that small candidate window rather than in the
database, so no vector extension is needed.

**Every count on this dashboard is a count of events, not of articles.** Before
clustering existed, one well-covered sabotage story contributed six adverse
items to the posture reading, which meant the reading tracked press attention
rather than events. An incident that has not been clustered yet counts as its
own event, so the numbers degrade to the old behaviour rather than dropping
while a backfill is in progress.

The threshold (0.70, `CLUSTER_THRESHOLD`) is **measured, not chosen**. Against
`text-embedding-3-small` at 512 dimensions, with the category and country
guards applied, genuine duplicate reports of one event score 0.73–0.86, while
distinct events that clear both guards top out at 0.64. 0.70 sits in that gap.

This is worth stating plainly because the first attempt got it wrong in an
instructive way: the constant was originally set to 0.86 on the reasoning that
a high threshold is the cautious choice. Measurement showed 0.86 would have
merged **nothing at all** — it sat above almost every true duplicate, so the
"cautious" setting was silently a no-op. The calibration test
(`internal/cluster`) now pins this down and must be re-run whenever the
embedding model or dimension count changes.

The two failure modes remain asymmetric: merging too eagerly collapses distinct
adverse events and publishes a **calmer** reading than reality, while merging
too timidly merely leaves us counting articles as before. So the threshold sits
above the midpoint of the gap and knowingly misses the hardest duplicates — a
state outlet's denial of an event an independent outlet reported scores 0.68
and stays a separate row.

**Corroboration** falls out of clustering. `confidence` is set mechanically
from the number of *independent* sources backing an event — state-controlled
outlets are counted as evidence but never as corroboration, because four
Kremlin wires repeating one claim is one claim:

| Independent sources | Score | Shown as |
|---|---|---|
| 3 or more | 0.95 | corroborated |
| 2 | 0.8 | corroborated |
| 1 | 0.5 | single source |
| 0 | 0.15 | state media only |

An incident that has not been clustered yet carries **no** label rather than
"single source" — not-yet-assessed is a different thing from uncorroborated,
and the UI must not conflate them.

This axis is the same one the OASIS **Common Alerting Protocol** calls
`certainty` (Observed / Likely / Possible), which both the German NINA feed and
MeteoAlarm's CAP feeds use. Ours was arrived at independently but maps onto it,
and aligning the export vocabulary is an open improvement.

**Pre-filter** (`internal/enrich/prefilter.go`) is keyword-based across
EN/LT/LV/ET/PL/RU/BE. It exists purely to cut LLM cost: roughly 80% of raw
volume is off-topic. Region-scoped sources (a Belarus monitoring channel, a
national subreddit) satisfy the region test by definition, because posts there
name a town rather than a country.

**Classification** batches ~20 items to an LLM which returns, per item:
relevance, category, affected countries, severity 1–5, **tone**, an English
summary, and coordinates where the text names a place.

## Severity and tone are independent

This separation is the core of the "inform, don't frighten" design.

- **Severity** is how consequential something is, regardless of who benefits.
- **Tone** is the direction *for the security of the region*: favourable,
  neutral, or adverse.

A NATO reinforcement is severity 3–4 and favourable. A successful arson attack
is severity 4 and adverse. **An arrest of a saboteur is favourable even though
its subject is sabotage** — the classifier is instructed explicitly on this,
because the naive reading would mark anything threat-adjacent as bad news and
the dashboard would report an ordinary week as a crisis.

Unknown or missing tone defaults to neutral, never adverse. A backfill must
never invent alarm.

## Source credibility

Russian and Belarusian state outlets are ingested **deliberately** — the
narrative aimed at the region is itself intelligence. They are never presented
as reporting:

| Class | Treatment |
|---|---|
| `institutional` | Governments, CERTs, EU agencies, public broadcasters |
| `independent` | Independent journalism, including **exiled** Russian outlets |
| `state-controlled` | Marked in the feed, **excluded from the posture** |

Exiled outlets (Meduza, Astra, The Insider, Mediazona) are independent, not
state-controlled; that distinction is the point of the field and is under test.
Nominally private Russian outlets operating under wartime censorship are marked
conservatively — over-marking costs a label, under-marking launders a claim.

The exclusion matters concretely: an adversary outlet must not be able to move
this dashboard's own threat gauge.

## Regional posture

A single ascending 1–5 reading — Calm, Watchful, Elevated, High, Severe.
Deliberately **not DEFCON numbering**, which counts down; the word carries the
meaning and the number only shows position.

Rules:

- Levels **4 and 5 are set by absolute adverse severity** and cannot be
  softened by good news. A serious incident stands on its own.
- A week with more favourable than adverse developments **steps the middle of
  the scale down one**, because sustained defensive progress genuinely is a
  different situation from the same adverse count with nothing going right.
- The banner always publishes the counts it was derived from, so the reading is
  auditable rather than a vibe.
- It answers **"is this week unusual?"** against the median of the trailing
  twelve weeks, and says "not enough history yet" when it cannot.
- Counts are **per event, not per article** (see de-duplication above).

The full ladder is published at `/api/meta` and behind "How this level is
decided" in the banner, so a reader can check the reading against the counts
instead of trusting it. ACLED publishes the weight vector behind its conflict
index; this is the equivalent.

### The ladder can go down, and says so

A reading also carries a **trend** — `escalating`, `steady` or
`de-escalating` — against the trailing weekly median. A week clearly below the
norm is badged "↓ easing"; `steady` is deliberately not badged, because a badge
on every ordinary week is noise.

CAP has the same idea as a standard field: `responseType: AllClear`, which
appeared on 34 of 62 Estonian alert blocks when the feed was read on
2026-08-29. Improvement being expressible is not a novelty, it is established
alerting practice.

Worth stating plainly, because it cuts against us: **no national alerting
system publishes a standing threat level.** Sweden's national alert endpoint
returns an empty array as its normal state, Estonia's crisis site has no
permanent gauge at all, and Germany's civil-protection feed carried 13 warnings
all rated `Minor`. Every body with legal authority to alert a population uses
silence by default plus time-boxed alerts. We publish a standing reading
because we are an aggregator rather than an alerting authority — a distinction
the preparedness section states explicitly — but the comparison is a real
argument for keeping "nothing unusual" the visually dominant default.

This exists because a scale that only ratchets upward stops meaning anything.
The US Homeland Security Advisory System is the cautionary case: its 2009
review found the national baseline had settled permanently at "guarded", and
its two lowest levels were never used once in nine years. The US Drought
Monitor avoids the same trap by defining its lowest class to explicitly include
"coming out of" drought — a named recovery state — and the trend field is our
version of that. Improvement has to be visible in its own right, not merely as
the absence of a rise.

## Signal layers

Machine measurements, shown on the map and never passed through the LLM:

| Layer | Source | Cadence |
|---|---|---|
| GPS jamming | gpsjam.org daily H3 cells | 6 h |
| Thermal anomalies | NASA FIRMS VIIRS, filtered to approach sectors | 2 h |
| Air activity | OpenSky snapshots of border boxes | 30 min |
| Sea activity | aisstream.io AIS in cable corridors | continuous |
| AIS archive | Finnish Digitraffic positions in the corridors | 15 min |
| Sanctioned vessels | OpenSanctions maritime, joined by MMSI | daily |
| Cyber rate (PL) | CERT.PL warning-list additions per day | daily |
| Satellite change | Sentinel-1 SAR over the watchlist | 20 h |

These are **detections, not verified events**, and the UI says so.

### What the AIS archive does and does not cover

aisstream.io is realtime-only and keeps no history, so a loitering event could
never be re-examined afterwards. The archive fixes that going forward by
storing every position inside a cable corridor.

Its coverage is the *Finnish* national AIS network, measured 2026-08-29 against
a live response of 1,095 vessels:

| Corridor | Vessels seen | Coverage |
|---|---|---|
| Gulf of Finland | 196 | good |
| Central Baltic | 52 | partial (feed starts at 57.4°N, the box at 56.5°N) |
| NordBalt | 0 | **none** |

That happens to cover Balticconnector and the EstLink cables, where the
highest-profile incidents occurred. But **an empty NordBalt corridor means no
data, not a quiet corridor**, and aisstream remains the only source there.

### What counts as a sea detection

The first week of live data produced **120 sea events, of which 4 involved a
sanctioned vessel**. That is not a detector working — it is a description of a
working seaway, and it buried the four that mattered. Three rules now separate
signal from traffic:

**Vessels that are stationary by declaration are not loitering.** AIS
navigational status 1 (at anchor), 5 (moored) and 6 (aground) are excluded.
Anchor was the significant omission: the corridor boxes span 137,000 km² and
include the Helsinki and Tallinn anchorages, where ships wait for a berth
entirely legitimately.

**Vessels whose job is holding station are excluded from loitering.** AIS ship
types 50–59 — pilot boats, search and rescue, tugs, port tenders, law
enforcement. Measured against the Finnish registry, these are **229 of 1,002
vessels (23%)**. Deliberately *not* excluded: fishing vessels, because trawling
a cable route is itself a known tactic; military; and vessels broadcasting no
type at all, since not-yet-classified is not the same as cleared. An AIS gap on
a pilot boat is still reported — going dark is nobody's job.

**Plain loitering is baseline, not a finding.** Ships stop constantly. Only two
things promote a sea event to the foreground: the vessel is on the sanctions
watchlist, or its transponder went dark. Everything else is still recorded —
it is the baseline that makes an anomaly meaningful — but it is dimmed on the
map and excluded from the layer count.

Applied to that same week: 16 service-craft detections would never have fired,
26 events surface (4 listed, 23 AIS gaps), and 78 remain as dimmed background.

One thing measured and discarded: **filtering by distance to a real cable does
not work.** The Baltic carries 49 cable routes, so 88% of events already fall
within 10 km of one and 64% within 5 km. Proximity is not selective here, and
the geometry would have been wasted effort.

### Naming the vessel

Sea events are joined to the OpenSanctions maritime dataset by MMSI, so the
dashboard can say a *listed shadow-fleet* vessel loitered over a cable rather
than merely that a vessel did. Roughly 6,600 of the dataset's ~20,400 vessels
publish an MMSI; the rest are IMO-only and cannot be matched against an AIS
broadcast. **An unmatched vessel is therefore not a cleared vessel**, and the
UI shows the absence of a flag as exactly that.

## What the dashboard refuses to claim

Each of these is a deliberate refusal, implemented rather than promised:

- **"Baseline still building"** instead of a trend percentage, until there are
  at least 8 adverse items in the prior 28 days. Early on, a near-empty
  baseline produced readings like "+7500%" — statistically true, meaningless,
  and alarming.
- **"Conditions changed — comparison suppressed"** when a SAR pass was taken
  under conditions unlike anything in its reference period. "We cannot tell" is
  the honest answer, not "alarm".
- **"Not enough history yet"** rather than a fabricated normal.
- Every automated detection carries a link to the underlying imagery or source.

## Known limitations

- Classification is automated and will contain errors. Every item links to its
  original source.
- The watchlist is curated, not an exhaustive order of battle.
- SAR detects aggregate footprint change, not objects — see
  [sar-detection.md](sar-detection.md).
- The 2021–22 precedent shows the final movement was into **dispersed farmland
  positions**, which are not installations and therefore not on any fixed
  watchlist. A site-based approach would likely miss that last step.
- This is an open-source monitor, not an alerting system and not an official
  assessment.
- **Clustering is calibrated against ten hand-written pairs, not a real
  labelled sample.** That was enough to catch a threshold that would have done
  nothing, but it is not enough to state a false-merge rate. Nobody has yet
  checked a sample of live merges by hand. The separation measured so far is
  narrow — 0.64 to 0.73 between the hardest distinct pair and the hardest
  duplicate — so some misclassification in both directions should be assumed.
- Clustering compares one-sentence summaries, so it inherits any error the
  classifier made in writing them. A summary that omits the location cannot be
  distinguished from a similar event elsewhere except by the country guard.
- Events are never **un**-merged. If two distinct incidents are wrongly joined,
  nothing currently splits them again short of manual intervention.

## Data out

The API is CORS-enabled and read-only. `/api/incidents.csv` and
`/api/incidents.geojson` accept exactly the same filters as the dashboard, so
an export always matches the view it was taken from. GeoJSON contains only
located incidents — an event that named no place gets no pin rather than a
placeholder on a capital city.

This project consumes gpsjam, FIRMS, OpenSky, Copernicus, GDELT and aisstream;
returning nothing would be poor manners.
