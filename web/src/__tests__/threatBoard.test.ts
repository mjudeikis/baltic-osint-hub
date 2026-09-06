import { describe, expect, it } from "vitest";
import { level } from "../components/ThreatBoard";

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
