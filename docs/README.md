# Documentation

How this project decides what to watch, what to say, and what it refuses to
claim — and how to run it.

| Document | What it covers |
|---|---|
| [methodology.md](methodology.md) | The pipeline end to end: sources and cadence, classification, tone, posture, and the rules that keep it from crying wolf |
| [self-hosting.md](self-hosting.md) | Operations: the Helm chart, Cloudflare Tunnel via cfgate, rollouts and pinning, backups and restore, health endpoints |
| [ukraine-2021-22.md](ukraine-2021-22.md) | What the Russian build-up before February 2022 actually looked like in open sources, and what it teaches about indicator design |
| [watchlist.md](watchlist.md) | Why each site is watched, the empty/occupied/hollow classification, and the distance bands |
| [sar-detection.md](sar-detection.md) | What Sentinel-1 can and cannot see, with the numbers, and how the collector runs it |
| [competitive-landscape.md](competitive-landscape.md) | **Internal working notes, not methodology.** A dated survey of who else does this, which novelty claims survive scrutiny, and what is missing. Findings are "not found", not "verified absent"; read the caveats at its top before quoting it. |

Root-level: [../CONTRIBUTING.md](../CONTRIBUTING.md) for running, testing and
the PR flow; [../SECURITY.md](../SECURITY.md) for reporting a vulnerability.

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
