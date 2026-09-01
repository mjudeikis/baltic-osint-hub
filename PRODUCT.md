# Product

<!-- impeccable:product-schema 1 -->

## Platform

web

## Users

**Primary: the concerned Baltic public** — residents of Lithuania, Latvia,
Estonia, and Poland checking "is this week unusual, should I care?" They win
every design trade-off. Plain language, readiness framing, no OSINT jargon at
the top of the page.

Secondary: OSINT analysts, journalists, and researchers who verify, cite, and
dig — served by source credibility labels, deep links, the public API, and
CSV/GeoJSON exports, but never at the primary audience's expense.

## Product Purpose

Public dashboard (live at [osintbaltic.com](https://osintbaltic.com)) tracking
open-source intelligence on hybrid threats against Lithuania, Latvia, Estonia,
and Poland: sabotage, GPS jamming, cyberattacks, disinformation, airspace and
border incidents, espionage, and military activity.

The purpose is **calibrated, auditable situational awareness for the general
public — readiness over anxiety**. The analyst machinery (event clustering,
tone scoring, posture math, signal layers) exists to serve a trustworthy
public read, not the other way around. Success is a resident who leaves
knowing whether this week is unusual and what, if anything, to do about it —
not a reader left uniformly alarmed.

## Positioning

Neighboring trackers are manually curated prose ledgers (Großwald) or
composite index dashboards (tutka). This product's mechanism they cannot
truthfully copy:

- **Counts events, not articles.** Clustering merges syndicated reports of the
  same incident, so six outlets carrying one story move the reading once.
- **A posture gauge an adversary cannot move.** State-controlled sources are
  ingested as intelligence but excluded from the posture calculation and never
  counted as corroboration.
- **Machine signal layers alongside classified news** — GPS jamming, thermal
  anomalies, air and sea activity, Sentinel-1 SAR change detection over a
  fixed military watchlist. Phrase the SAR claim as "we are not aware of
  another automated one" — the absolute negative is unverified ([NF] in
  docs/competitive-landscape.md) and must never appear in public copy.
- **Threats paired with what to do about them**: each country's official civil
  preparedness guidance (LT72, 72 stundas, kriis.ee, RCB) is linked from the
  dashboard.

## Operating Context

- Hourly collector (Kubernetes CronJob) → OpenAI classification → Postgres →
  Go server → React dashboard. Self-hosted on k8s.
- The read-only API is public with open CORS; third parties may build on it.
- Data freshness varies by source (30 min to 20 h minimum intervals); the
  dashboard is a rolling picture, not a live wire.
- Docs in `docs/` (methodology, SAR detection, watchlist, competitive
  landscape, Ukraine 2021–22 baseline study) are part of the public product —
  the auditability claim depends on them.

## Capabilities and Constraints

- Every count on the dashboard is a count of **events**, not articles.
- Every item carries category, countries, severity 1–5, **tone**
  (favourable/neutral/adverse for regional security), credibility class
  (institutional / independent / state-controlled), and an English summary.
- Regional posture scale: **Calm → Watchful → Elevated → High → Severe**
  (ascending; deliberately not DEFCON numbering). The banner always shows the
  counts it derives from, and answers "is this week unusual?" against the
  trailing twelve-week median.
- Known coverage gaps are product facts the UI must state, not smooth over:
  AIS history has **no coverage in NordBalt** (empty means no data, not
  quiet); CERT.PL cyber rate is Poland-only; OpenSky/aisstream depend on
  volunteer receiver networks.
- OpenSanctions maritime data is CC BY-NC; CERT.PL surfaces counts only,
  never domains.
- Exports: `/api/incidents.csv`, `/api/incidents.geojson`; URL state is
  shareable (`web/src/urlState.ts`).

## Brand Commitments

*(all confirmed by the user, 2026-08-31)*

- **English-only UI.** No LT/LV/ET/PL localization planned.
- **Sober, institutional tone.** No sensationalism or click-bait framing, in
  UI copy and in classification output alike.
- **Never fabricate certainty.** Coverage gaps, confidence levels, and
  "not found ≠ verified absent" caveats stay visible in the product.
- **Free and open, no accounts.** Public read-only API, open CORS, no login,
  no paywall.
- Name: **Baltic OSINT Hub**; domain **osintbaltic.com**.

## Evidence on Hand

- Live production data at osintbaltic.com — real incidents, real signal
  layers, real posture history. Design work should use real data, never
  invented incidents.
- Public methodology docs: `docs/methodology.md`, `docs/sar-detection.md`,
  `docs/watchlist.md`, `docs/ukraine-2021-22.md`,
  `docs/competitive-landscape.md`.
- Official preparedness resources linked in-product
  (`web/src/components/Preparedness.tsx`).
- **Absent, must not be fabricated:** testimonials, user counts, press
  mentions, institutional endorsements, and any settled negative claims about
  competitors.

## Product Principles

1. **The public read wins.** A resident's five-second "is this week unusual?"
   outranks analyst density; depth stays one click below, never deleted.
2. **Auditable, not a vibe.** Every reading shows the counts it came from;
   every flag deep-links to a source a human can verify.
3. **Readiness over anxiety.** Favourable developments are surfaced with the
   same rigor as adverse ones; threats come paired with preparedness guidance.
4. **Honest about gaps.** No data is displayed as no data — never as calm,
   never hidden.
5. **Adversary-aware by construction.** State-controlled sources are
   intelligence to display, never inputs that can steer the gauge.
