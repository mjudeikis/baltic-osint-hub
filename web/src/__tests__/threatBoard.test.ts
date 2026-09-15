import { describe, expect, it } from "vitest";
import { Incident } from "../api";
import { level, topEvents } from "../components/ThreatBoard";

const inc = (
  id: number,
  occurred_at: string,
  severity: number,
  event_id?: number,
  countries = ["LT"],
): Incident => ({
  id,
  occurred_at,
  severity,
  event_id,
  countries,
  category: "airspace-border",
  tone: "negative",
  place: "",
  credibility: "independent",
  summary: `incident ${id}`,
  source: "lrt-en",
  url: "",
  title: "",
  reports: 1,
  confidence: 0.5,
});

describe("ThreatBoard topEvents()", () => {
  it("puts last night's event above an older, more severe one", () => {
    const got = topEvents(
      [
        inc(1, "2026-09-10T12:00:00Z", 4),
        inc(2, "2026-09-15T02:36:00Z", 3),
        inc(3, "2026-09-13T07:21:00Z", 3),
      ],
      "LT",
    );
    expect(got.map((i) => i.id)).toEqual([2, 3, 1]);
  });

  it("shows one line per event and only the tile's country", () => {
    const got = topEvents(
      [
        inc(1, "2026-09-15T03:00:00Z", 4, 7),
        inc(2, "2026-09-15T04:00:00Z", 4, 7),
        inc(3, "2026-09-15T05:00:00Z", 3, undefined, ["PL"]),
        inc(4, "2026-09-14T05:00:00Z", 2, 8),
      ],
      "LT",
    );
    expect(got.map((i) => i.id)).toEqual([2, 4]);
  });
});

describe("ThreatBoard level()", () => {
  it("is set by corroborated severity before volume", () => {
    expect(level(0, 5, 0).label).toBe("Critical");
    expect(level(0, 4, 0).label).toBe("Serious");
    expect(level(12, 3, 0).label).toBe("Elevated");
  });

  it("holds an uncorroborated serious report at Unconfirmed", () => {
    expect(level(1, 3, 0, true).label).toBe("Unconfirmed");
  });

  it("weighs favourable news against adverse volume", () => {
    expect(level(3, 2, 3).label).toBe("Elevated");
    expect(level(3, 2, 4).label).toBe("Watchful");
    expect(level(1, 2, 0).label).toBe("Watchful");
  });

  it("never reads Quiet over a stale collector", () => {
    expect(level(0, 0, 0).label).toBe("Quiet");
    const stale = level(0, 0, 0, false, true);
    expect(stale.label).toBe("No recent data");
    expect(stale.cssVar).toBe("--status-warning");
    // A real reading is still a reading even when stale.
    expect(level(2, 3, 0, false, true).label).toBe("Watchful");
  });
});
