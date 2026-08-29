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
export function cssColor(varName: string): string {
  return getComputedStyle(document.documentElement).getPropertyValue(varName).trim();
}

export const SEVERITY_LABELS: Record<number, string> = {
  1: "Analysis",
  2: "Minor",
  3: "Notable",
  4: "Serious",
  5: "Critical",
};

export const severityColor = (sev: number): string =>
  cssColor(`--seq-${Math.min(Math.max(sev, 1), 5)}`);
