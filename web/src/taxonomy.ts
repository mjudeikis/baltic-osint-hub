// Category taxonomy — must stay in sync with internal/enrich (also served at
// /api/meta). Chart color slots are FIXED per category, never assigned by rank.

export const COUNTRIES = ["LT", "LV", "EE", "PL"] as const;
export type Country = (typeof COUNTRIES)[number];

export const COUNTRY_NAMES: Record<Country, string> = {
  LT: "Lithuania",
  LV: "Latvia",
  EE: "Estonia",
  PL: "Poland",
};

// Codes arrive from URLs and API responses as plain strings; fall back to the
// code itself rather than forcing casts at every call site.
export const countryName = (code: string): string =>
  (COUNTRY_NAMES as Record<string, string>)[code] ?? code;

export interface CategoryDef {
  key: string;
  label: string;
  // CSS variable carrying the categorical slot color; "other" categories share
  // the muted slot and are folded together in charts.
  cssVar: string;
  folded?: boolean;
}

export const CATEGORIES: CategoryDef[] = [
  { key: "cyber", label: "Cyber", cssVar: "--series-1" },
  { key: "sabotage", label: "Sabotage", cssVar: "--series-2" },
  { key: "gps-jamming", label: "GPS jamming", cssVar: "--series-3" },
  { key: "airspace-border", label: "Airspace & border", cssVar: "--series-4" },
  { key: "disinformation", label: "Disinformation", cssVar: "--series-5" },
  { key: "military", label: "Military", cssVar: "--series-6" },
  { key: "espionage", label: "Espionage", cssVar: "--series-7" },
  { key: "undersea-infrastructure", label: "Undersea infra", cssVar: "--series-8" },
  { key: "energy", label: "Energy", cssVar: "--series-other", folded: true },
  { key: "political", label: "Political", cssVar: "--series-other", folded: true },
];

export const categoryLabel = (key: string): string =>
  CATEGORIES.find((c) => c.key === key)?.label ?? key;

export const categoryColor = (key: string): string => {
  const def = CATEGORIES.find((c) => c.key === key);
  return cssColor(def?.cssVar ?? "--series-other");
};

// Resolve a CSS custom property to its current computed value (recharts and
// maplibre need concrete colors, not var() references).
//
// Cached: a 200-row feed render resolves tokens ~800 times, and each
// getComputedStyle call is a style-recalc hazard on the phones least able to
// afford one. The cache empties when the theme changes, so post-flip renders
// pick up the new values.
const colorCache = new Map<string, string>();

export function cssColor(varName: string): string {
  let v = colorCache.get(varName);
  if (v === undefined) {
    v = getComputedStyle(document.documentElement).getPropertyValue(varName).trim();
    colorCache.set(varName, v);
  }
  return v;
}

if (typeof window !== "undefined") {
  window
    .matchMedia("(prefers-color-scheme: dark)")
    .addEventListener("change", () => colorCache.clear());
  new MutationObserver(() => colorCache.clear()).observe(
    document.documentElement,
    { attributes: true, attributeFilter: ["data-theme"] },
  );
}

// The status hues fail contrast as text on light surfaces; every place a
// status colour becomes text resolves it through here, so dots and fills keep
// the canonical hue while text gets the AA variant of the same hue.
const STATUS_TEXT: Record<string, string> = {
  "--status-good": "--status-good-text",
  "--status-warning": "--status-warning-text",
  "--status-serious": "--status-serious-text",
  "--status-critical": "--status-critical-text",
};

export const textVar = (varName: string): string => STATUS_TEXT[varName] ?? varName;

export const textColor = (varName: string): string => cssColor(textVar(varName));

// Tone — the direction of an item for regional security. Always rendered as
// colour + symbol + word so it never depends on colour alone.
export interface ToneDef {
  key: string;
  label: string;
  symbol: string;
  cssVar: string;
}

export const TONES: Record<string, ToneDef> = {
  positive: { key: "positive", label: "Favourable", symbol: "▲", cssVar: "--status-good" },
  neutral: { key: "neutral", label: "Neutral", symbol: "●", cssVar: "--text-muted" },
  negative: { key: "negative", label: "Adverse", symbol: "▼", cssVar: "--status-critical" },
};

export const toneDef = (key: string): ToneDef => TONES[key] ?? TONES.neutral;

// Posture ladder, ascending: 1 calmest, 5 worst. Deliberately not DEFCON
// numbering (which counts down) — the word carries the meaning.
export const POSTURE_LEVELS: { level: number; cssVar: string }[] = [
  { level: 1, cssVar: "--status-good" },
  { level: 2, cssVar: "--status-good" },
  { level: 3, cssVar: "--status-warning" },
  { level: 4, cssVar: "--status-serious" },
  { level: 5, cssVar: "--status-critical" },
];

export const postureColor = (level: number): string =>
  cssColor(POSTURE_LEVELS.find((p) => p.level === level)?.cssVar ?? "--text-muted");

// For the level rendered as text (the posture word): same hue, AA lightness.
export const postureTextColor = (level: number): string =>
  textColor(POSTURE_LEVELS.find((p) => p.level === level)?.cssVar ?? "--text-muted");

// Source credibility. State-controlled outlets are ingested deliberately so
// the narrative aimed at the region is visible, but they must never be
// presented like national broadcasting.
export const CREDIBILITY: Record<string, { label: string; short: string; cssVar: string }> = {
  institutional: { label: "Official or public-service source", short: "official", cssVar: "--text-muted" },
  independent: { label: "Independent reporting", short: "independent", cssVar: "--text-muted" },
  "state-controlled": {
    label: "Russian or Belarusian state-controlled outlet — this is adversary messaging, not verified reporting",
    short: "state media",
    cssVar: "--status-warning",
  },
};

export const SEVERITY_LABELS: Record<number, string> = {
  1: "Analysis",
  2: "Minor",
  3: "Notable",
  4: "Serious",
  5: "Critical",
};

export const severityColor = (sev: number): string =>
  cssColor(`--seq-${Math.min(Math.max(sev, 1), 5)}`);

// The low steps of the severity ramp are light; white text on them fails
// contrast, so the badge text flips to dark below step 3.
export const severityTextColor = (sev: number): string =>
  sev <= 2 ? "#0b0b0b" : "#ffffff";
