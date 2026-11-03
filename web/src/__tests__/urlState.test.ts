import { describe, expect, it } from "vitest";
import { DEFAULT_FILTERS, exportURL, feedURL, readFilters, toQuery } from "../urlState";

describe("readFilters", () => {
  it("returns defaults for an empty query", () => {
    expect(readFilters("")).toEqual(DEFAULT_FILTERS);
  });

  it("accepts known values and rejects unknown ones", () => {
    const f = readFilters("?days=7&country=LV&category=cyber&tone=negative&day=2026-09-01&sev=4");
    expect(f).toEqual({ days: 7, country: "LV", category: "cyber", tone: "negative", day: "2026-09-01", sev: 4 });
    const bad = readFilters("?days=5&country=XX&category=nope&tone=loud&day=yesterday&sev=9");
    expect(bad).toEqual(DEFAULT_FILTERS);
  });

  it("round-trips sev=0 as a deliberate opt-out", () => {
    expect(readFilters("?sev=0").sev).toBe(0);
    expect(toQuery({ ...DEFAULT_FILTERS, sev: 0 })).toBe("?sev=0");
  });
});

describe("toQuery", () => {
  it("omits defaults so the bare URL stays clean", () => {
    expect(toQuery(DEFAULT_FILTERS)).toBe("");
    expect(toQuery({ ...DEFAULT_FILTERS, country: "EE", days: 90 })).toBe("?days=90&country=EE");
  });

  it("round-trips through readFilters", () => {
    const f = { days: 90, country: "PL", category: "sabotage", tone: "positive", day: "2026-08-30", sev: 3 };
    expect(readFilters(toQuery(f))).toEqual(f);
  });
});

describe("exportURL", () => {
  it("carries the view's filters and a 500 cap", () => {
    const u = exportURL("csv", { ...DEFAULT_FILTERS, country: "LT" });
    expect(u).toBe("/api/incidents.csv?days=30&country=LT&severity=2&limit=500");
    expect(exportURL("geojson", { ...DEFAULT_FILTERS, sev: 0 })).toBe("/api/incidents.geojson?days=30&limit=500");
  });
});

describe("feedURL", () => {
  it("drops the day and limit, and always states severity", () => {
    const u = feedURL("rss", { ...DEFAULT_FILTERS, country: "LT", day: "2026-11-01" });
    expect(u).toBe("/api/incidents.rss?days=30&country=LT&severity=2");
    expect(feedURL("atom", { ...DEFAULT_FILTERS, sev: 0 })).toBe("/api/incidents.atom?days=30&severity=0");
  });
});
