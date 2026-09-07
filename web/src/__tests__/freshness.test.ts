import { describe, expect, it } from "vitest";
import { freshness, relative, staleNote } from "../freshness";
import { formatDay, formatDayLong, parseDay } from "../dates";
import { median, weeklyAdded } from "../components/CertPLPanel";

const src = (last_run: string, error = "") => ({ source: "x", last_run, items_found: 0, items_new: 0, error });
const now = Date.parse("2026-09-06T12:00:00Z");

describe("freshness", () => {
  it("is unknown before any status has loaded", () => {
    const f = freshness([], now);
    expect(f.state).toBe("unknown");
    expect(f.stale).toBe(true);
  });

  it("uses the most recent run across sources", () => {
    const f = freshness([src("2026-09-06T05:00:00Z"), src("2026-09-06T11:30:00Z")], now);
    expect(f.state).toBe("live");
    expect(f.stale).toBe(false);
  });

  it("goes stale after two hours and stalled after six", () => {
    expect(freshness([src("2026-09-06T09:00:00Z")], now).state).toBe("stale");
    expect(freshness([src("2026-09-06T05:00:00Z")], now).state).toBe("stalled");
    expect(staleNote(freshness([src("2026-09-06T05:00:00Z")], now))).toMatch(/has not run since/);
  });

  it("formats ages in plain words", () => {
    expect(relative(Infinity)).toBe("unknown");
    expect(relative(30_000)).toBe("just now");
    expect(relative(5 * 60_000)).toBe("5 min ago");
    expect(relative(3 * 3_600_000)).toBe("3 h ago");
    expect(relative(48 * 3_600_000)).toBe("2 days ago");
  });
});

describe("dates", () => {
  it("treats a day string as a local calendar date", () => {
    const d = parseDay("2026-09-06")!;
    expect([d.getFullYear(), d.getMonth(), d.getDate()]).toEqual([2026, 8, 6]);
    expect(formatDay("2026-09-06")).toBe("6 Sep");
    expect(formatDayLong("2026-09-06", new Date(2026, 0, 1))).toBe("Sun 6 Sep");
    expect(formatDayLong("2025-12-31", new Date(2026, 0, 1))).toBe("Wed 31 Dec 2025");
    expect(formatDay("not a day")).toBe("not a day");
  });
});

describe("CERT.PL weekly rate", () => {
  it("sums daily additions into trailing weeks, current week last", () => {
    const days = [
      { day: "2026-09-06T00:00:00Z", added: 3, removed: 0 },
      { day: "2026-09-01T00:00:00Z", added: 4, removed: 1 },
      { day: "2026-08-25T00:00:00Z", added: 10, removed: 0 },
    ];
    const w = weeklyAdded(days, new Date(now), 3);
    expect(w.map((x) => x.added)).toEqual([0, 10, 7]);
    expect(w[2]!.start).toBe("2026-08-31");
  });

  it("computes a median", () => {
    expect(median([])).toBe(0);
    expect(median([5, 1, 3])).toBe(3);
    expect(median([1, 2, 3, 4])).toBe(2.5);
  });
});
