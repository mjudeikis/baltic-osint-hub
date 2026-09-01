---
name: Baltic OSINT Hub
description: Calm, flat, instrument-grade public dashboard for hybrid-threat awareness
colors:
  page: "#f9f9f7"
  surface-1: "#fcfcfb"
  text-primary: "#0b0b0b"
  text-secondary: "#52514e"
  text-muted: "#6f6d67"
  grid: "#e1e0d9"
  baseline: "#c3c2b7"
  border: "rgba(11, 11, 11, 0.1)"
  series-1: "#2a78d6"
  series-2: "#eb6834"
  series-3: "#1baf7a"
  series-4: "#eda100"
  series-5: "#e87ba4"
  series-6: "#008300"
  series-7: "#4a3aa7"
  series-8: "#e34948"
  series-other: "#898781"
  status-good: "#0ca30c"
  status-warning: "#fab219"
  status-serious: "#c56a55"
  status-critical: "#d03b3b"
  status-good-text: "#007701"
  status-warning-text: "#805906"
  status-serious-text: "#9e4430"
  status-critical-text: "#b53232"
  layer-jamming: "#557d8d"
  layer-thermal: "#8d5a74"
  layer-air: "#5d6b8c"
  layer-searoutine: "#6f7d6a"
  layer-sites: "#7c7255"
  layer-cable: "#74797c"
  layer-territory: "#8d6b5a"
  seq-1: "#86b6ef"
  seq-2: "#5598e7"
  seq-3: "#2a78d6"
  seq-4: "#1c5cab"
  seq-5: "#104281"
typography:
  display:
    fontFamily: "system-ui, -apple-system, 'Segoe UI', sans-serif"
    fontSize: "28px"
    fontWeight: 650
    lineHeight: 1.15
  headline:
    fontFamily: "system-ui, -apple-system, 'Segoe UI', sans-serif"
    fontSize: "22px"
    fontWeight: 700
  title:
    fontFamily: "system-ui, -apple-system, 'Segoe UI', sans-serif"
    fontSize: "15px"
    fontWeight: 700
  body:
    fontFamily: "system-ui, -apple-system, 'Segoe UI', sans-serif"
    fontSize: "14px"
    fontWeight: 400
    lineHeight: 1.5
  body-small:
    fontFamily: "system-ui, -apple-system, 'Segoe UI', sans-serif"
    fontSize: "12.5px"
    fontWeight: 400
    lineHeight: 1.45
  label:
    fontFamily: "system-ui, -apple-system, 'Segoe UI', sans-serif"
    fontSize: "11.5px"
    fontWeight: 600
    letterSpacing: "0.05em"
rounded:
  mark: "3px"
  badge: "4px"
  control: "6px"
  card: "8px"
  pill: "99px"
spacing:
  xs: "4px"
  sm: "8px"
  md: "12px"
  lg: "16px"
  xl: "24px"
components:
  card:
    backgroundColor: "{colors.surface-1}"
    rounded: "{rounded.card}"
    padding: "16px"
  chip:
    textColor: "{colors.text-secondary}"
    typography: "{typography.label}"
    rounded: "{rounded.pill}"
    padding: "1px 8px"
  severity-badge:
    backgroundColor: "{colors.seq-3}"
    textColor: "#ffffff"
    rounded: "{rounded.badge}"
    padding: "1px 6px"
  filter-button:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.text-primary}"
    rounded: "{rounded.control}"
    padding: "6px 10px"
  filter-button-active:
    backgroundColor: "{colors.surface-1}"
    textColor: "{colors.series-1}"
    rounded: "{rounded.control}"
    padding: "6px 10px"
  chip-clear:
    textColor: "{colors.series-1}"
    rounded: "{rounded.pill}"
    padding: "2px 10px"
---

# Design System: Baltic OSINT Hub

## Overview

**Creative North Star: "The Control Room Wall"**

A public-facing operations display: calm, factual, steady. The surfaces are
quiet warm-neutral paper; every color on screen is a signal with a fixed,
learnable meaning; nothing is decorative. The design never looks urgent — even
when reporting serious events, the wall stays level-voiced and lets severity
scales, tone symbols, and posture words carry the message. Sober authority
without militarism: this is civil situational awareness, not a war room LARP.

The system is dataviz-first. Category hues, severity blues, and status colors
are a fixed vocabulary shared by every chart, map mark, badge, and legend, so
a reader who learns "orange is sabotage" once is never lied to. Interface
chrome is flat, bordered, and desaturated so the data layer owns all chromatic
attention. Components are quiet until it matters: controls recede into
transparent-bordered pills and plain text, and color or weight appears only as
state — an active filter, a focused row, a state-media warning.

**Key Characteristics:**
- Flat, bordered surfaces on warm-neutral paper; zero shadows.
- Color is a fixed semantic vocabulary, never decoration and never cycled.
- Every meaning is triple-encoded: color + symbol/shape + word.
- Dense but humane: 12.5px working text, tabular numerals, generous grouping.
- Fully dual-theme; light is the base, dark redefines the same roles.

## Colors

A restrained warm-gray neutral core with three fixed semantic palettes riding
on top: categorical hues for threat categories, a sequential blue ramp for
severity, and an unthemed status scale for posture and warnings.

### Primary
- **Signal Blue** (`series-1`, #2a78d6): the one interactive accent. Links,
  active filters, focused rows, the current nav item, drill-through hovers —
  and, as data, the *cyber* category. Interaction color and category color are
  deliberately the same voice.

### Neutral
- **Warm Paper** (`page`, #f9f9f7): the page field.
- **Card Stock** (`surface-1`, #fcfcfb): raised panels, tiles, banners, popups.
- **Ink** (`text-primary`, #0b0b0b): headings, numbers, emphasized copy.
- **Graphite** (`text-secondary`, #52514e): working text, summaries, legends.
- **Faded Ink** (`text-muted`, #6f6d67 light / #898781 dark): eyebrows,
  timestamps, table headers, de-emphasized metadata. AA on both surfaces —
  muted never means unreadable, because it carries the "no data" statements
  the product commits to keeping visible. (`series-other` keeps the old
  #898781 as a chart color.)
- **Pencil Grid** (`grid`, #e1e0d9): chart gridlines, row separators.
- **Putty** (`baseline`, #c3c2b7): chart baselines, resting control borders.
- **Hairline** (`border`, rgba(11,11,11,0.1)): 1px card and panel borders.

### Categorical series (fixed slots)
- **Signal Blue** (`series-1`, #2a78d6): cyber.
- **Flare Orange** (`series-2`, #eb6834): sabotage.
- **Sea Green** (`series-3`, #1baf7a): GPS jamming.
- **Beacon Amber** (`series-4`, #eda100): airspace & border.
- **Rose** (`series-5`, #e87ba4): disinformation.
- **Field Green** (`series-6`, #008300): military.
- **Deep Violet** (`series-7`, #4a3aa7): espionage.
- **Coral Red** (`series-8`, #e34948): undersea infrastructure.
- **Faded Ink** (`series-other`, #898781): folded minor categories (energy,
  political).

### Sequential severity ramp
- **Severity Blues** (`seq-1`…`seq-5`, #86b6ef → #104281): severity 1–5
  badges and ordinal encodings. Light steps 1–2 take dark text (#0b0b0b);
  steps 3–5 take white.

### Status scale (hues never themed)
- **All-Clear Green** (`status-good`, #0ca30c): favourable tone, corroborated
  events, posture levels 1–2, de-escalating trend.
- **Watch Amber** (`status-warning`, #fab219): posture level 3, escalating
  trend, state-media marking, single-source caution.
- **Serious Terracotta** (`status-serious`, #c56a55): posture level 4. Muted
  terracotta, deliberately distinct from Flare Orange (#eb6834) — the level-4
  ladder must never wear sabotage's category color.
- **Critical Red** (`status-critical`, #d03b3b): posture level 5, adverse tone.
- **Text variants** (`status-*-text`): the hues above are for dots, rungs,
  fills, and borders; amber and terracotta fail WCAG contrast as text on
  light surfaces. Any status color rendered as *text* resolves through
  `textVar()`/`textColor()` in `taxonomy.ts` — same hue, lightness themed per
  scheme for AA (light: darkened; dark: brightened where needed — serious
  #e08a76, critical #e05252).

### Instrument family (machine-measured map layers)
Muted, low-chroma tokens for the map's machine layers — GPS jamming
(`layer-jamming`), thermal (`layer-thermal`), air (`layer-air`), sea baseline
(`layer-searoutine`), radar sites (`layer-sites`), cables (`layer-cable`),
RU/BY territory (`layer-territory`). Desaturation is the point: a detection
must never wear a category's saturated voice, and "machine, not event" is
perceivable at a glance. Notable sea events keep Watch Amber — being flagged
is a status, not an identity.

### Named Rules
**The Fixed Slot Rule.** Every category owns its series slot permanently
(orange *is* sabotage). Colors are never assigned by rank, frequency, or
cycle order — in charts, maps, chips, and legends alike.

**The Never-Alone Rule.** Color never carries meaning by itself. Tone is
color + symbol + word (▲ Favourable / ● Neutral / ▼ Adverse); posture is
color + level + word; map layers are color + shape. A colorblind reader or a
grayscale printout loses nothing.

**The Unthemed Status Rule.** The four status *hues* are identical in light
and dark themes — a warning must mean exactly the same thing at midnight.
The one sanctioned exception is lightness: the `status-*-text` variants adapt
per theme so the words stay readable, but the hue never changes.

**The Instrument Rule.** Machine-measured layers use the muted `layer-*`
family; the saturated categorical hues belong to reported incidents, charts,
and chips only. A reader who learned "orange is sabotage" must never be lied
to by a jamming cell.

## Typography

**Display/Body Font:** system-ui stack (`system-ui, -apple-system, "Segoe UI",
sans-serif`) — no webfonts by design; the dashboard loads on a weak connection
and looks native everywhere.

**Character:** invisible, factual, fast. The type never performs; hierarchy
comes from a disciplined size-and-weight ladder and uppercase labels, not from
family changes. Numbers are the loudest thing on the wall.

### Hierarchy
- **Display** (650, 28–30px, 1.15): the posture level word and country-tile
  counts — the two readings a visitor came for.
- **Headline** (700, 22px): the site title only.
- **Title** (700, 15px): card and section headings.
- **Body** (400, 14px, 1.5): summaries, explanations, preparedness copy.
- **Body-small** (400, 12.5px, 1.45): the working density — feed metadata,
  legends, tables, filters, banners.
- **Label** (600, 11.5px, 0.04–0.06em, UPPERCASE): eyebrows, day headers,
  table column headers.

### Named Rules
**The Tabular Number Rule.** Any column of numbers sets
`font-variant-numeric: tabular-nums` and right-aligns.

**The Level Voice Rule.** Alarm is never typographic. No bold-red headlines,
no exclamation styling; severity and posture words in their normal ladder
sizes are the maximum volume.

## Layout

A single centered column (max-width 1200px, 16px side padding, 48px bottom)
holding a vertical stack of bordered cards. Above the fold: a status banner
strip, then the posture card — the five-second read. Below, a two-column grid
(172px sticky side navigation + `minmax(0,1fr)` content, 24px gap) walks
through board, timeline, map, satellite, feed, sources, preparedness as
collapsible sections.

Spacing rhythm is a loose 4px scale: 4/8/12/16/24, with 16px as the default
card padding and inter-card gap, 12px for in-card grouping, and 10–12px grid
gaps in dense boards (`repeat(auto-fit, minmax(220px, 1fr))` for country
tiles). Density increases with depth: page-level readings are airy, tables
run at 12.5px with 6px row padding.

Breakpoints are need-based, not systematic: at 900px the sidebar collapses
into a wrapped pill nav; at 700px banner links stop floating right; at 620px
legend definition grids stack. Scroll behavior is smooth with
`scroll-margin-top: 12px` on anchored sections, disabled under
`prefers-reduced-motion`.

## Elevation & Depth

**Flat. No shadows anywhere — this is an invariant, not an omission.** Depth
is conveyed by exactly two devices: a one-step surface lift (Card Stock panels
on the Warm Paper page) and 1px Hairline borders. Overlap legibility on the
map is handled by stroking marks in the surface color, not by drop shadows.

### Named Rules
**The Flat Field Rule.** No `box-shadow`, no gradients, no glows. If a new
element seems to need a shadow to separate, give it a border and the surface
color instead.

## Shapes

Rectangles with small, role-graded radii: 8px for cards and panels, 6px for
controls and the map frame, 4px for severity badges, 3px for swatches and
ladder rungs, and full pills (99px) for chips, trend markers, and the
collapsed mobile nav. Sharp corners do not exist; neither do large friendly
radii — the range is tight and utilitarian.

On the map, shape is a semantic channel: circle = incident, triangle =
aircraft, diamond = vessel, hexagon = jamming cell (real H3 geometry), dashed
line = cable, tinted outlined area = territory. Filled marks demand attention;
hollow marks are context.

### Named Rules
**The One Channel Rule.** One visual channel per variable. Shape says which
layer, fill says how much it matters, color says which kind. Never encode two
variables in one channel or one variable in two.

**The Tone Arrow Rule.** ▲ and ▼ mark tone direction (favourable/adverse) and
nothing else. Status alarms use the diamond family — ◆ for alarm states
(Serious, Critical, Change detected, Collection stalled), ◇ for caution
states (Unconfirmed, Elevated, stale data) — so a grayscale or colorblind
reader never sees one glyph meaning both good and bad on the same page.

## Components

Quiet until it matters: every control rests as plain text or a
transparent-bordered pill, and gains color, border, or weight only through
state (`aria-pressed`, `aria-current`, `data-` attributes, hover, focus).

### Buttons
- **Shape:** gently rounded (6px); pill (99px) for chip-scale actions.
- **Filter buttons:** Card Stock background, Putty 1px border, 6px 10px
  padding. Active (`aria-pressed="true"`): border and text flip to Signal
  Blue, weight 600 — no fill change.
- **Link-like buttons** (`.linklike`): actions that read as links (reset,
  clear) — Signal Blue, underlined, no chrome; hover dims to 80% opacity.
- **Drill-through buttons:** invisible at rest (inherit everything); hover
  shows Signal Blue text plus a 10% Signal Blue tint
  (`color-mix(in srgb, var(--series-1) 10%, transparent)`).

### Chips
- **Base chip:** pill, 1px Putty border, Graphite text, 11.5px, 1px 8px
  padding, `white-space: nowrap`.
- **Category chip** (`.chip.cat`): adds a 5px left border in the category's
  fixed series color — a colored spine, not a colored fill.
- **Clear-filter chip:** Signal Blue text and border over a 10% Signal Blue
  tint; hover deepens the tint to 20%.
- **Corroboration pill:** muted by default (the corroborated case stays
  quiet); `data-level="state-media-only"` flips it to Watch Amber,
  `data-level="corroborated"` to All-Clear Green — border and text only.

### Cards / Containers
- **Corner Style:** 8px.
- **Background:** Card Stock on Warm Paper.
- **Border:** 1px Hairline; no shadow (see Elevation).
- **Internal Padding:** 16px (posture card 16px 18px; banners 8px 14px).
- **Rhythm:** 16px top margin between stacked cards.

### Badges
- **Severity badge:** 4px radius, 1px 6px padding, 11.5px/600; background is
  the severity's Severity Blues step; text is white at steps 3–5, Ink at 1–2.
- **State-media warning:** Watch Amber pill (`.cred-warn`), paired with a 3px
  Watch Amber left border on the feed row itself — adversary messaging is
  marked twice, at the badge and at the row edge.

### Tables
- Dense and scannable: 12.5px, 6px vertical padding, Pencil Grid row rules,
  uppercase Label column headers in Faded Ink, tabular right-aligned
  numerics, `overflow-x: auto` wrapper. Focused rows take a 12% Signal Blue
  tint via `data-focused`.

### Navigation
- **Desktop:** 172px sticky rail; items are 12.5px Graphite links on a 2px
  Pencil Grid left rule. Current section (`aria-current="true"`): Signal Blue
  text and left rule, weight 600. Hover: Ink on Card Stock.
- **Mobile (≤900px):** the same links reflow into wrapped pills with Putty
  borders; the current pill's border goes Signal Blue.

### Disclosures (signature)
- Explanatory legends ("here is how this works") are `<details>` blocks styled
  identically everywhere: 8px-radius bordered box, 600-weight Graphite
  `<summary>`, 12.5px body — collapsed by default. The reading must work
  without them; the definitions are there for whoever needs them.

### Posture Ladder (signature)
- Five ascending rungs (26px wide, 14→30px tall, 3px radius) colored by the
  status scale, beside the level word at Display size. The trend pill appears
  only when the week departs from the norm — absence of decoration is itself
  information.

## Do's and Don'ts

### Do:
- **Do** take colors only from the token vocabulary in `theme.css`; resolve
  them via `var(--token)` (or `cssColor()` for canvas/Recharts/MapLibre), and
  resolve any status color used as text through `textColor()`/`textVar()`.
- **Do** pair every color-coded meaning with a symbol or word (the
  Never-Alone Rule) and keep status colors identical across themes.
- **Do** style both themes for any new element: light base, dark via
  `@media (prefers-color-scheme: dark)` *and* `[data-theme="dark"]`.
- **Do** keep state on semantic attributes (`aria-pressed`, `aria-current`,
  `data-level`, `data-trend`) and style those selectors — never class-toggled
  visual state.
- **Do** respect `prefers-reduced-motion` for any transition; the existing
  ceiling is 150ms ease on micro-state changes.
- **Do** render absence honestly: no data is "no data" in Faded Ink, never a
  zero, never hidden (a durable product commitment).

### Don't:
- **Don't** add shadows, gradients, glows, or webfonts — the Flat Field Rule
  and the system stack are invariants.
- **Don't** cycle, reassign, or rank-order category colors, and don't use
  series hues for interface decoration.
- **Don't** raise the visual alarm volume: no red headlines, no pulsing, no
  urgency styling beyond the status scale in its defined slots.
- **Don't** let state-media content render unmarked — the amber pill and row
  edge are non-negotiable wherever such items appear.
- **Don't** introduce new radii, font sizes, or spacing steps outside the
  established ladders without recording them here.
