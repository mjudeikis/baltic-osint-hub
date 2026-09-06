import { TimelineBucket } from "./api";
import { CATEGORIES } from "./taxonomy";

export type TimelineRow = Record<string, number | string> & { day: string };

export interface TimelineSeries {
  key: string;
  label: string;
  cssVar: string;
}

export interface TimelineData {
  data: TimelineRow[];
  series: TimelineSeries[];
  // First day with data when the collector is younger than the window, else
  // null. Rendered as a caveat so a short history inside a long window does
  // not read as a left-to-right escalation ramp.
  collectionStart: string | null;
}

// Stacked daily counts, densified. The 8 chart categories use their fixed
// slots; folded categories (energy, political) merge into a single muted
// "Other" series so the stack never exceeds the validated 8-slot palette.
//
// Densify: a categorical axis renders only the days present, so a six-day
// calm gap would collapse into two adjacent bars and the chart would
// manufacture escalation. Quiet days are the answer to "is this week
// unusual?" and must occupy their real width. The window starts at the first
// day with data (a fresh database has no meaningful zeros before collection
// began) and runs to `now` with every day present.
//
// Pure: colours are resolved by the caller, so this runs under test without
// a DOM.
export function buildTimelineData(
  buckets: TimelineBucket[],
  days: number,
  now: Date = new Date(),
): TimelineData {
  const byDay = new Map<string, TimelineRow>();
  for (const b of buckets) {
    const day = b.day.slice(0, 10);
    const def = CATEGORIES.find((c) => c.key === b.category);
    const key = def?.folded ? "other" : b.category;
    const row = byDay.get(day) ?? { day };
    row[key] = ((row[key] as number | undefined) ?? 0) + b.count;
    byDay.set(day, row);
  }
  let collectionStart: string | null = null;
  if (byDay.size > 0) {
    const first = [...byDay.keys()].sort()[0]!;
    const start = new Date(`${first}T00:00:00Z`);
    const windowStart = new Date(now);
    windowStart.setUTCHours(0, 0, 0, 0);
    windowStart.setUTCDate(windowStart.getUTCDate() - (days - 1));
    const from = start > windowStart ? start : windowStart;
    if (start > windowStart) collectionStart = first;
    for (const d = new Date(from); d <= now; d.setUTCDate(d.getUTCDate() + 1)) {
      const key = d.toISOString().slice(0, 10);
      if (!byDay.has(key)) byDay.set(key, { day: key });
    }
  }
  const data = [...byDay.values()].sort((a, b) => a.day.localeCompare(b.day));
  const present = new Set(buckets.map((b) => b.category));
  const series: TimelineSeries[] = CATEGORIES.filter(
    (c) => !c.folded && present.has(c.key),
  ).map((c) => ({ key: c.key, label: c.label, cssVar: c.cssVar }));
  if (CATEGORIES.some((c) => c.folded && present.has(c.key))) {
    series.push({ key: "other", label: "Other", cssVar: "--series-other" });
  }
  // Recharts stacks break on missing keys — make every row dense.
  for (const row of data) {
    for (const s of series) {
      if (row[s.key] === undefined) row[s.key] = 0;
    }
  }
  return { data, series, collectionStart };
}

export const rowTotal = (row: TimelineRow, series: TimelineSeries[]): number =>
  series.reduce((n, s) => n + ((row[s.key] as number | undefined) ?? 0), 0);
