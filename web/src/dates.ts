// Day strings (YYYY-MM-DD) are calendar dates, not instants. `new Date("2026-09-06")`
// parses as UTC midnight, and formatting that in a local zone west of UTC
// prints the day before — so a Lithuanian reader in New York saw every feed
// header off by one. Building the Date from parts makes it local midnight,
// which formats as the day it names in every zone.
//
// Formatting is hand-rolled rather than Intl: the UI is English-only, and
// ICU builds disagree on details ("Sept" vs "Sep"), which would give the axis
// and the feed headers two voices depending on the reader's browser.

const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];

export function parseDay(day: string): Date | null {
  const m = /^(\d{4})-(\d{2})-(\d{2})/.exec(day);
  if (!m) return null;
  return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]));
}

// "6 Sep" — axes, chips, captions.
export function formatDay(day: string): string {
  const d = parseDay(day);
  if (!d) return day;
  return `${d.getDate()} ${MONTHS[d.getMonth()]}`;
}

// "Sat 6 Sep" — feed day headers. The year is only spelled out when it is not
// the current one, so the common case stays short.
export function formatDayLong(day: string, now: Date = new Date()): string {
  const d = parseDay(day);
  if (!d) return day;
  const year = d.getFullYear() === now.getFullYear() ? "" : ` ${d.getFullYear()}`;
  return `${WEEKDAYS[d.getDay()]} ${d.getDate()} ${MONTHS[d.getMonth()]}${year}`;
}
