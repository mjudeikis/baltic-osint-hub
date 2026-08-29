# Documentation

How this project decides what to watch, what to say, and what it refuses to
claim.

| Document | What it covers |
|---|---|
| [methodology.md](methodology.md) | The pipeline end to end: collection, classification, tone, posture, and the rules that keep it from crying wolf |
| [ukraine-2021-22.md](ukraine-2021-22.md) | What the Russian build-up before February 2022 actually looked like in open sources, and what it teaches about indicator design |
| [watchlist.md](watchlist.md) | Why each site is watched, the empty/occupied/hollow classification, and the distance bands |
| [sar-detection.md](sar-detection.md) | What Sentinel-1 can and cannot see, with the numbers |
| [competitive-landscape.md](competitive-landscape.md) | Who else does this, which of our novelty claims survive scrutiny, and what we are missing |

## The design goal, stated plainly

The dashboard exists to **inform so people can prepare**, not to frighten. That
is not a tone preference; it drives concrete engineering decisions, and most of
the non-obvious choices in this codebase trace back to it:

- Every item carries a **tone**, so defensive progress is visible rather than
  drowned by threat reporting. A week of arrests and deployments should not
  read like a week of sabotage.
- The posture reading can **go down**, and says why.
- Adversary media is ingested but **marked and excluded** from the reading, so
  a hostile outlet cannot move our own gauge.
- Where the data cannot support a judgement, the dashboard **says so** —
  "baseline still building", "conditions changed — comparison suppressed",
  "not enough history yet" — rather than inventing a number.
- Every automated detection links to the underlying imagery or source so a
  human can check it.

A monitoring tool that overstates is worse than none: it burns the reader's
attention and their trust, and when something real happens they have already
stopped believing it.
