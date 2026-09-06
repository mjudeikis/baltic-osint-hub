import { describe, expect, it } from "vitest";
import { Incident } from "../api";
import { groupByDay } from "../components/Feed";

const inc = (id: number, at: string, credibility = "independent"): Incident => ({
  id,
  category: "cyber",
  countries: ["LT"],
  severity: 2,
  tone: "negative",
  place: "",
  credibility,
  summary: "",
  occurred_at: at,
  source: "s",
  url: "",
  title: "t",
  reports: 1,
  confidence: 0,
});

describe("groupByDay", () => {
  it("groups consecutive items by calendar day", () => {
    const g = groupByDay([
      inc(1, "2026-09-06T10:00:00Z"),
      inc(2, "2026-09-06T08:00:00Z"),
      inc(3, "2026-09-05T23:00:00Z"),
    ]);
    expect(g.map((x) => x.day)).toEqual(["2026-09-06", "2026-09-05"]);
    expect(g[0]!.items.map((i) => i.id)).toEqual([1, 2]);
  });

  it("moves state-controlled items after reporting within a day", () => {
    const g = groupByDay([
      inc(1, "2026-09-06T10:00:00Z", "state-controlled"),
      inc(2, "2026-09-06T09:00:00Z"),
      inc(3, "2026-09-06T08:00:00Z", "institutional"),
    ]);
    expect(g[0]!.items.map((i) => i.id)).toEqual([2, 3, 1]);
  });

  it("returns nothing for nothing", () => {
    expect(groupByDay([])).toEqual([]);
  });
});
