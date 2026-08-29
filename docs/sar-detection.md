# Sentinel-1 SAR: what it can and cannot see

The numbers behind the satellite change-detection layer, and the limits it
must not be claimed to exceed.

## The resolution reality

**Sentinel-1 IW GRDH is 20.4 × 22.5 m true resolution at 10 × 10 m pixel
spacing.** The commonly quoted "10 m" is pixel spacing, not resolution. This
distinction is load-bearing for everything below.

Against the NIIRS interpretability scale and the Johnson criteria, which agree
independently:

| Capability | Required resolution |
|---|---|
| Detect vehicles at a fixed site; detect revetments | 2.5–4.5 m (NIIRS 3) |
| Identify tracked vs wheeled by general type | 1.2–2.5 m (NIIRS 4) |
| Distinguish support vehicles from tanks | 0.40–0.75 m (NIIRS 6) |
| Distinguish a tank from a three-dimensional decoy | 0.20–0.40 m (NIIRS 7) |

A ~3 m tank at 3 m ground sample distance is roughly half a Johnson cycle —
**below individual detection**. You cannot count tanks on free imagery, and at
Sentinel-1 resolution you cannot see berms either.

**What is achievable is aggregate footprint detection.** Texty.org.ua
demonstrated exactly this in January 2022, monitoring 26 Russian bases with
Sentinel-1 and finding build-up at 10 of them between September and December
2021 — including Yelnya and Pogonovo. Their own statement of the limit is the
single most relevant sentence for this project: individual vehicles cannot be
identified, but **"accumulation is seen as a set of brighter dots."**

That is precisely what `internal/layers/sentinel.go` measures.

## Why SAR rather than optical, in this theatre specifically

Winter cloud cover: **Smolensk 78–80% November–February; Minsk 79% overcast in
December with 1.3 sunshine-hours per day; Kyiv 78% in January** — roughly one
usable-sky day in four or five. PlanetScope's median maximum revisit interval
is 9.15 days *before* cloud. Low winter sun degrades even cloud-free days at
these latitudes.

Measured acquisition cadence over this project's own sites, queried from the
Copernicus catalogue:

| Site | Winter 2025-26 | Summer 2026 |
|---|---|---|
| Gozhsky range | 1.5 d/observation | 1.5 d |
| Chernyakhovsk | 1.7 d | 1.6 d |
| Pskov | 3.2 d* | 1.6 d |

\* lower only because Sentinel-1D was not yet operational.

**SAR cadence is season-independent** — identical in December darkness and
cloud as in June sunshine. That is the whole argument, measured rather than
asserted.

## Constellation history that affects historical comparison

- **Sentinel-1B failed 23 December 2021** — two months before the invasion.
  Interferometric revisit halved from 6 to 12 days at the build-up's peak and
  stayed there until May 2025.
- **Sentinel-1A retired 29 June 2026**; C and D restored 6-day repeat from
  early July 2026, on a grid shifted one day from the old A/B pattern.

Any baseline spanning those dates changes character partway through.

## What we deliberately do not attempt

**Coherent change detection (CCD) for vehicle tracks.** Every classic
demonstration is airborne, X-band, spotlight, metre-or-finer, short-baseline.
Temporal decorrelation kills the transfer to Sentinel-1: forest coherence can
fall to near zero after a single 6-day repeat, and grass and farmland decay to
the floor over 10–90 days. Only bare ground, gravel, tarmac and concrete stay
coherent — which is why the published damage-mapping literature works on
buildings and rubble, not open-field dispersal areas.

The best calibration point available: a well-engineered Sentinel-1 amplitude
pipeline (Dietrich et al., *Communications Earth & Environment*, 2025) achieves
F1 ≈ 74.9% with a **minimum detectable object of buildings larger than 50 m²**.
Vehicles are smaller and transient.

Free platforms are amplitude-only by architecture — GRD discards phase at
product generation, and OPERA CSLC excludes Europe. The only free coherence
path is ASF HyP3 burst InSAR.

## The metric this project uses

Bright-pixel fraction: the share of pixels whose VV backscatter exceeds −5 dB.
Bare soil and grass sit near −15 dB, forest around −8 dB; metal vehicles,
aircraft and rolling stock are corner reflectors returning well above −5 dB.

Three guards make it usable rather than noisy:

1. **Scene-mean regression.** Rain, harvest and sea state brighten an entire
   scene, mechanically pushing pixels past a fixed threshold. The bright
   fraction is measured as a residual against the site's own bright-vs-mean
   relationship, so only brightness the scene does not explain counts.
2. **Scene-shift suppression.** If the latest pass was taken under conditions
   unlike anything in the reference period, no comparison is valid and the site
   reports "conditions changed" rather than raising or clearing an alarm.
3. **A reference gap.** The most recent passes are held out of the baseline, so
   a sustained change cannot creep into its own reference and hide itself.

Observed in practice: a region-wide backscatter shift moved *every* monitored
site positively in one window, including a water-dominated box at Baltiysk.
Without guard 2 that would have flagged Pskov at 27.9σ. It is suppressed.

## Known outstanding limitation

C-band backscatter over **wet snow** drops sharply, and freeze–thaw cycles
shift it in both directions — at 54–60°N, in exactly the November–March window
of highest concern. The scene-shift guard should catch region-wide events, but
this has not yet been validated against a full winter series. Until it is,
treat winter readings with extra caution.

## Known defect: relative-orbit mixing (not yet fixed)

The collector filters acquisitions to `orbitDirection: DESCENDING`, with a code
comment claiming this keeps incidence angle comparable. **It does not.**

Querying the Copernicus catalogue over this project's own sites shows each AOI
is covered by several distinct *relative orbits* within each direction:

```
Gozhsky range   139 scenes/180d   ASC relOrbit 58, ASC 131, DESC 80, DESC 153
Pskov           138 scenes/180d   5 distinct relative orbits
Obuz-Lesnovsky  138 scenes/180d   4 distinct relative orbits
```

Each relative orbit is natively 6-day with identical viewing geometry.
Filtering only by direction still merges 2–3 of them into one series —
injecting exactly the geometry-driven variance the filter was meant to remove —
while discarding the ascending half of the data.

**The correct unit is one series per (site × relativeOrbitNumber).** Doing so
would give strictly better comparability *and*, by fusing 4–5 per-orbit
verdicts, roughly 1.5-day effective warning latency instead of 6.

Implementation sketch:

1. Migration: add `relative_orbit` to `layer_sar`; primary key becomes
   `(aoi, interval_start, relative_orbit)`.
2. Change `aggregationInterval` from `P6D` to `P1D` so each observation maps to
   a single acquisition date.
3. Query the Catalog API per AOI
   (`POST /api/v1/catalog/1.0.0/search`, field `properties.sat:relative_orbit`)
   to build a date → orbit map, and tag each stored observation.
   **Note:** some products return `sat:relative_orbit = -1`; those need a
   documented fallback rather than being silently bucketed together.
4. `DetectAnomaly` groups by orbit, evaluates each series independently, and
   fuses: flag when any orbit with a sufficient baseline flags, and report
   which one.

Until this lands, treat the scene-mean regression as partially compensating —
different incidence angles shift the scene mean, so some geometry variance is
already absorbed — but do not assume per-pass comparability.

## Known defect: no backtest yet

The detector has never been run against a labelled positive. There is an
obvious one available and **Yelnya is already in the watchlist**: photographed
vacant on 11 September 2021, holding roughly 700 vehicles by December, and
independently shown to be detectable on Sentinel-1 by Texty.org.ua.

A proper validation would run:

- **Yelnya, Aug 2021 – Feb 2022** — labelled positive.
- **Pogonovo, Mar–May 2021** — second positive.
- **Zapad-2021 ranges, Sep–Oct 2021** — true-positive-but-not-invasion, which
  is the exact confound that fooled analysts in April 2021.

That would give a measured false-alarm rate instead of an assumed one.
