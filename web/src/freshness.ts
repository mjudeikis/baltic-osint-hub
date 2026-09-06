import { SourceStatus } from "./api";

// One definition of "is the pipeline current?", shared by the status strip,
// the country board and the trend chart. The collector runs hourly, so
// anything much past that means collection has stopped rather than simply
// being between runs. An empty board or chart over a stalled collector is
// "no data", and no data must never be shown as calm.
export const STALE_AFTER_MS = 2 * 60 * 60 * 1000;
export const CRITICAL_AFTER_MS = 6 * 60 * 60 * 1000;

export type FreshnessState = "unknown" | "live" | "stale" | "stalled";

export interface Freshness {
  state: FreshnessState;
  // Most recent successful run across all sources, or null when nothing has
  // loaded yet.
  lastRun: Date | null;
  // Milliseconds since lastRun; Infinity when unknown.
  age: number;
  // True whenever the data cannot be trusted to be current: stale, stalled,
  // or not yet known. Callers that render "quiet" consult this first.
  stale: boolean;
}

export function freshness(sources: SourceStatus[], now: number = Date.now()): Freshness {
  const lastRun = sources.reduce<Date | null>((latest, s) => {
    const t = new Date(s.last_run);
    if (Number.isNaN(t.getTime())) return latest;
    return !latest || t > latest ? t : latest;
  }, null);
  const age = lastRun ? now - lastRun.getTime() : Infinity;
  const state: FreshnessState =
    sources.length === 0
      ? "unknown"
      : age > CRITICAL_AFTER_MS
        ? "stalled"
        : age > STALE_AFTER_MS
          ? "stale"
          : "live";
  return { state, lastRun, age, stale: state !== "live" };
}

export function relative(ms: number): string {
  if (!isFinite(ms)) return "unknown";
  const min = Math.floor(ms / 60000);
  if (min < 1) return "just now";
  if (min < 60) return `${min} min ago`;
  const hours = Math.floor(min / 60);
  if (hours < 24) return `${hours} h ago`;
  const days = Math.floor(hours / 24);
  return `${days} day${days === 1 ? "" : "s"} ago`;
}

// The sentence every "empty" state uses when the pipeline is not current, so
// a quiet-looking panel says why it is empty rather than implying calm.
export function staleNote(f: Freshness): string {
  if (f.state === "unknown") return "No recent data — collection status is not yet known.";
  return `No recent data — the collector has not run since ${
    f.lastRun ? f.lastRun.toLocaleString("en-GB") : "an unknown time"
  } (${relative(f.age)}).`;
}
