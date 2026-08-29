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

**De-duplication** is by URL and then by normalised-title hash within a 7-day
window, because agencies syndicate the same headline widely.

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

## Signal layers

Machine measurements, shown on the map and never passed through the LLM:

| Layer | Source | Cadence |
|---|---|---|
| GPS jamming | gpsjam.org daily H3 cells | 6 h |
| Thermal anomalies | NASA FIRMS VIIRS, filtered to approach sectors | 2 h |
| Air activity | OpenSky snapshots of border boxes | 30 min |
| Sea activity | aisstream.io AIS in cable corridors | continuous |
| Satellite change | Sentinel-1 SAR over the watchlist | 20 h |

These are **detections, not verified events**, and the UI says so.

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
