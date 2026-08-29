# The watchlist: what is monitored and why

Defined in `internal/layers/aoi.go`. Every site is a publicly known
installation reported in open sources; the boxes are deliberately coarse
(5–15 km), because the layer reports "activity at this site changed", not
object-level detections.

## Two rules, both learned the hard way

**Watch outward.** Early warning comes from the adversary's own ground. By the
time equipment is visible at a NATO-side crossing the warning value is spent.
An early version of this watchlist had 9 of 18 sites on NATO territory; that
was a design error, and a test now fails the build if friendly-side sites
creep back in.

**Weight by baseline class, not by importance.** This is the finding from
[the 2021–22 build-up](ukraine-2021-22.md): object detection at permanently
occupied garrisons fired identically in a quiet spring and a pre-invasion
winter.

## Baseline classes

| Class | Meaning | Value |
|---|---|---|
| **Empty** | Ranges and rail ramps unused most of the year | **Highest.** Anything present is signal |
| **Occupied** | Permanent garrisons and air bases | Low. Presence means nothing; only density change counts, and that is a weak delta |
| **Hollow** | Garrisons that look normal but whose units are committed to Ukraine | **The trap.** Refilling produces almost no signature until vehicle parks refill — never read "unchanged" as "quiet" |

The hollow set is documented: the 76th Air Assault Division at Pskov, the 336th
Naval Infantry at Baltiysk, and the whole 11th Army Corps in Kaliningrad —
assessed at 71% of authorised strength by August 2022, with sub-units as low
as 23%.

## Distance bands

Calibrated to the Baltic theatre, which is **much shallower than Ukraine's**.
There is no Yelnya equivalent here: warning is shorter and comes from rail.

| Band | Range | Warning | What it tells you |
|---|---|---|---|
| 0 — Commitment | 0–25 km | Hours to days | Confirmation, not warning. In Ukraine this band was empty until 20 December and only filled in the final week |
| 1 — Forward assembly | 25–75 km | Days to ~2 weeks | The **emptying** is the signal, not the filling |
| 2 — Operational staging | 75–160 km | Weeks to months | **The highest-value band.** Where Kursk (106 km) and Pogonovo (160 km) sat. Longest dwell time, so most detections per image |
| 3 — Generation | 160–400 km | Months, via rail | Thin on the ground here — monitor rail throughput rather than site occupancy |
| 4 — Beyond 400 km | — | — | Do not watch sites; watch the loading ramps everything must transit |

Suggested collection weighting: ~45–55% on Band 2, ~25% Band 1, ~15% on Band 3
rail nodes, ~10% Band 0. Weight empty-class sites roughly 3:1 over occupied
ones within every band.

## The Suwałki gap

The pincer is short and both jaws are awkward to observe:

| Site | km to gap | Class |
|---|---|---|
| Gozha range | 33 | **Empty** — highly observable |
| Gusev (79th Gds MR Regt, 11th Tank Regt) | 40 | **Hollow** — hardest thing on the list to see |
| Grodno rail station | 44 | Empty |
| Chernyakhovsk (152nd Iskander Bde) | 65 | Occupied |

The eastern jaw is the empty one and the western jaw is the hollow one. And
the 152nd Iskander Brigade covers the entire corridor from its permanent
garrison at 65 km — **no repositioning required, therefore no repositioning
signal.** That is a genuine blind spot, not something the detector can fix.

Practical reading: watch Obuz-Lesnovsky and Borisovsky for build-up; watch
Gozha and Grodno for commitment.

## Rail loading ramps

Documented Russian unloading points, not inferred ones. Rail is how these
formations actually move, and a ramp is empty by default — which makes it the
cleanest signal class available.

Brest-Tsentralny is the structural chokepoint: the break of gauge means
everything moving west by rail is transloaded there.

## Coordinate quality

Distances are computed to the nearest NATO border segment. An earlier version
measured Kaliningrad sites against the Belarus–Poland line rather than the
exclave's own frontier, overstating them by a factor of two or more; those are
corrected.

Known gaps: **OSM carries essentially no military tagging for Kaliningrad**, so
the Pravdinsky and Dobrovolsky ranges — both on the Zapad-2021 list, Dobrovolsk
15 km from Lithuania — are unmapped and would need manual digitising. That is
the largest remaining geolocation gap.
