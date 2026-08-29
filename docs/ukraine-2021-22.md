# What the 2021–22 Russian build-up looked like in open sources

Research compiled 2026-08-29 to calibrate this project's watchlist and
detector. Distances below were computed point-to-nearest-border-segment
against Natural Earth 10m / geoBoundaries / OSM geometry, not quoted from
reporting; where reporting gave its own figure the two agree within a few
kilometres (Kursk computed 106.2 vs Maxar "~110"; Klintsy 42.4 vs "~40";
Mazyr 36.2 vs "<40").

## The three findings that matter most

### 1. Object detection would have cried wolf

Armour parks, tent cities, troop counts and **field hospitals all fired
identically in April 2021 and February 2022**. CSIS documented a field hospital
at Pogonovo on 10 April 2021 — ten months before the invasion, during a
build-up that dispersed. Anything that merely counts objects at a military site
cannot separate the two.

What discriminated was **state transitions and relational anomalies**:
equipment moving *out* of storage rather than in; a pontoon bridge that appears
and then disappears; an announced naval "exercise" closure box containing no
warships; a rear airfield swapping armour for attack helicopters; units from
the wrong military district; vehicles dispersed along tree lines instead of
parked in rows.

This is why the watchlist carries a [baseline class](watchlist.md) rather than
treating every site alike.

### 2. Distance from the border was a poor indicator until the final weeks

The autumn 2021 concentrations were on average **further** from Ukraine than
the spring 2021 ones. Yelnya, at 248 km, was the deepest major site of the
entire crisis — which is exactly why ISW read it on 2 November 2021 as routine
winter redeployment.

Occupancy moved through bands over time:

| Period | Where the force sat |
|---|---|
| Autumn 2021 | 160–250 km — deep ranges and storage sites |
| Dec 2021 – Jan 2022 | a 20–50 km belt |
| 15–23 Feb 2022 | 11–34 km, dispersed onto farmland and tree lines |

The band below 20 km was **empty until 20 December**. The US DoD description on
23 February — forward positions "as little as five kilometres, all the way out
to 50", ~80% of forces forward, "today, they are uncoiled" — matches exactly.

The operational consequence: **the transition between bands is the signal, not
occupancy of any single band.**

### 3. There is no Yelnya equivalent in the Baltic theatre

Yelnya gave 115 days of warning at 248 km. Toward Lithuania the deepest
Belarusian site is Borisovsky at 146 km; toward Estonia, Luzhsky at 104 km. The
equivalent depth is Russian territory 400–700 km away, reachable only by rail.

**Warning here will be shorter than Ukraine's, and rail throughput matters more
than site occupancy.** A motor rifle battalion needs 78–80 flatcars; the
Jan–Feb 2022 Belarus movement used >200 trains, roughly 10,000 railcars, some
originating 7,000–9,500 km away. Everything from the Moscow district or further
east must transit a small number of loading ramps.

## Indicators ranked by lead time

First unambiguous public signal, days before 24 February 2022:

| Lead | Indicator |
|---|---|
| 4–10 months | Equipment left in place after the April 2021 "withdrawal"; Zapad-2021 units not returning home, stockpiles left in Belarus |
| 115 d | Vehicle parks at Yelnya (Maxar via Politico, 1 Nov) |
| 99–77 d | Long-haul unit identification — 41st CAA from Novosibirsk, 55th Bde from Tuva (4,000 km) |
| 50–38 d | Russian embassy in Kyiv quietly thinning |
| 41 d | Eastern district rail movement west, destination concealed for four days |
| 38 d | Amphibious flotilla leaves the Baltic |
| 30 d | Divisional EW battalion attached at Valuyki |
| 27 d | Blood supplies moved forward |
| 17–15 d | Landing ships through the Bosphorus |
| 16 d | Unprecedented Black Sea / Azov closure regime |
| 15 d | Field hospitals appear in imagery |
| 12 d | Aviation war-risk insurance withdrawn |
| 11–10 d | Camps emptying; congregation at assembly areas |
| 9 d | Pontoon bridge over the Pripyat |
| 5 d | "Z" markings first appear publicly |
| 2 d | Capella SAR: road-march columns, **no tents** |
| 1 d | Sub-unit packets on farmland and tree lines within 16 km |

Lead time and discriminating power are **different orderings**. The
highest-confidence indicators were: amphibious ships through the Turkish
straits (irreversible, no exercise rationale); the transition from consolidated
storage to dispersed terrain-masked packets; a pontoon bridge built and struck
with no exercise precedent; invasion markings unseen in eight years of
monitoring; and the "empty exercise box" — BlackSeaNews found no significant
warship presence inside the announced closure zones on 13 February, meaning the
closures were cover for movement rather than training.

## Indicators that do not survive scrutiny

Worth recording, because several are widely repeated and would waste effort:

- **Fuel bladders and POL depots** — no public imagery evidence exists in the
  entire period. Not a viable satellite indicator.
- **Ammunition dumps** — no imagery-identified dump in the public record. The
  useful version is the Zapad residue (units and stockpiles left behind), which
  is a policy observation, not change detection.
- **Insignia removal** — unverified; the only hard evidence is unreadable
  licence plates in one video.
- **GPS jamming did not exist as an indicator.** gpsjam.org was created in July
  2022, five months *after* the invasion.
- **NOTAMs lagged rather than led.** Airspace closures landed on 24 February,
  D-0; EASA had explicitly declined to recommend a ban on 13 February. The real
  aviation signal was **war-risk insurance withdrawal at D-12** — reinsurers
  priced invasion while the regulator was still declining to advise.
- **Cyber gave essentially no strategic warning.** WhisperGate at D-42 produced
  a five-week false calm; the long-lead artefacts were only reconstructed
  afterwards.
- **The Google Maps traffic story is mis-told.** The trigger was a Capella SAR
  image on 22 February showing road-march columns with no tents; Google traffic
  confirmed an existing hypothesis, and the mechanism was civilians stuck at
  roadblocks, not soldiers' phones.

## Where the evidence is thin

Recorded so later work does not treat these as settled:

- **"~700 vehicles at Yelnya" is single-origin** — Janes, 9 Dec 2021.
  Everything else repeats it.
- **Soloti has two competing readings** — new deployment versus routine
  post-Zapad return plus re-equipment. Maxar's own analyst hedged.
- **"~125 BTGs" is a myth.** The official figure on 23 Feb was "north of 120".
- **The Pentagon published no briefing transcript between 9 and 23 February
  2022**, so every troop figure from the decisive fortnight rests on wire
  reporting quoting anonymous officials rather than a primary document.
- ISW made a documented early mis-call: on 2 Nov 2021 it judged reports of 1st
  Guards Tank Army deploying "likely inaccurate". Janes and CIT said the
  opposite and were vindicated.

## Two theatre-specific warnings

**Zapad-2025 was deliberately distributed and sub-threshold** — 13,000 declared
against the 200,000 claimed for Zapad-2021, staged ~300 km from Poland, and
modularised to stay under the Vienna Document's 13,000-troop observation
threshold. **A single large Yelnya-style concentration may not repeat.** A
detector must catch several simultaneous medium concentrations.

**Belaruski Hajun was compromised on 5 February 2025** and ceased operations on
7 February, followed by roughly 1,500 arrests. The dominant human-reporting
channel for Belarusian military movement no longer exists — which is the
strongest single argument for automated satellite change detection in this
theatre.

## Further reading

- Gustafson et al., "Intelligence warning in the Ukraine war", *Intelligence
  and National Security* 39:3 (2024) — open access, the first serious academic
  treatment.
- The Estonian Foreign Intelligence Service annual report signed 31 January
  2022, which stated in writing before the event that Russian forces were
  "ready to embark on a full-scale military operation against Ukraine from the
  second half of February… only a political decision is required." That is the
  model for what this dashboard's output should aspire to: a dated, published,
  falsifiable readiness judgement with the political decision named as pending.
