// Day strings (YYYY-MM-DD) are calendar dates, not instants. `new Date("2026-09-06")`
// parses as UTC midnight, and formatting that in a local zone west of UTC
// prints the day before — so a Lithuanian reader in New York saw every feed
// header off by one. Building the Date from parts makes it local midnight,
// which formats as the day it names in every zone.

export function parseDay(day: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(day);
  if (!m) return null;
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
}

export function formatDay(
  day: string,
  opts: Intl.DateTimeFormatOptions = { weekday: "short", day: "numeric", month: "short" },
): string {
  const d = parseDay(day);
  return d ? d.toLocaleDateString("en-GB", opts) : day;
}

// Full form for feed headers: "Sat 6 Sep 2026".
export const formatDayLong = (day: string): string =>
  formatDay(day, { weekday: "short", day: "numeric", month: "short", year: "numeric" });
