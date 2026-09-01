import { CATEGORIES, COUNTRIES } from "./taxonomy";

// Filters live in the querystring so any view of this dashboard can be linked,
// cited or bookmarked. A public-information site whose "12 adverse events in
// Latvia this week" cannot be pointed at is not much use to anyone trying to
// discuss it.

export interface FilterState {
  days: number;
  country: string;
  category: string;
  tone: string;
  // A single selected day (YYYY-MM-DD) from clicking the timeline, or "".
  // Shareable like every other filter — "what happened on the 12th" is exactly
  // the kind of view someone wants to send to a colleague.
  day: string;
  // Minimum severity, or 0 for all. Board drill-throughs set 2 so the feed
  // count equals the tile count (which excludes severity-1 analysis).
  sev: number;
}

export const DAY_PRESETS = [7, 30, 90] as const;

export const DEFAULT_FILTERS: FilterState = {
  days: 30,
  country: "",
  category: "",
  tone: "",
  day: "",
  // Events-only by default: the feed opens with things that happened, and
  // severity-1 analysis is one toggle away. The public read wins.
  sev: 2,
};

const TONES = ["negative", "positive", "neutral"];

// Values arrive from a URL anyone can edit, so each is checked against the
// taxonomy rather than passed through to the API. An unrecognised value falls
// back to the default instead of producing an empty dashboard with no
// explanation.
function oneOf(value: string | null, allowed: readonly string[]): string {
  return value && allowed.includes(value) ? value : "";
}

export function readFilters(search: string = location.search): FilterState {
  const p = new URLSearchParams(search);
  const days = Number(p.get("days"));
  const sev = Number(p.get("sev"));
  return {
    days: (DAY_PRESETS as readonly number[]).includes(days) ? days : DEFAULT_FILTERS.days,
    country: oneOf(p.get("country"), COUNTRIES),
    category: oneOf(
      p.get("category"),
      CATEGORIES.map((c) => c.key),
    ),
    tone: oneOf(p.get("tone"), TONES),
    day: /^\d{4}-\d{2}-\d{2}$/.test(p.get("day") ?? "") ? p.get("day")! : "",
    // 0 (all items) is a deliberate opt-out and must round-trip through the
    // URL now that the default is 2.
    sev:
      p.get("sev") === "0"
        ? 0
        : Number.isInteger(sev) && sev >= 2 && sev <= 5
          ? sev
          : DEFAULT_FILTERS.sev,
  };
}

// toQuery omits defaults so a shared link carries only what was actually
// chosen, and the bare URL stays clean.
export function toQuery(f: FilterState): string {
  const p = new URLSearchParams();
  if (f.days !== DEFAULT_FILTERS.days) p.set("days", String(f.days));
  if (f.country) p.set("country", f.country);
  if (f.category) p.set("category", f.category);
  if (f.tone) p.set("tone", f.tone);
  if (f.day) p.set("day", f.day);
  if (f.sev !== DEFAULT_FILTERS.sev) p.set("sev", String(f.sev));
  const q = p.toString();
  return q ? `?${q}` : "";
}

// syncURL rewrites the address bar without adding a history entry: changing a
// filter should not mean the back button walks through every intermediate
// selection before leaving the page.
export function syncURL(f: FilterState): void {
  const next = location.pathname + toQuery(f) + location.hash;
  if (next !== location.pathname + location.search + location.hash) {
    history.replaceState(null, "", next);
  }
}

// exportURL builds a download link carrying whatever the reader is looking at,
// so an export always matches the view it was taken from.
export function exportURL(kind: "csv" | "geojson", f: FilterState): string {
  const p = new URLSearchParams();
  p.set("days", String(f.days));
  if (f.country) p.set("country", f.country);
  if (f.category) p.set("category", f.category);
  if (f.tone) p.set("tone", f.tone);
  if (f.day) p.set("day", f.day);
  if (f.sev) p.set("severity", String(f.sev));
  p.set("limit", "500");
  return `/api/incidents.${kind}?${p}`;
}
