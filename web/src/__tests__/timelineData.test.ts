import { describe, expect, it } from "vitest";
import { buildTimelineData, rowTotal } from "../timelineData";

const now = new Date("2026-09-06T12:00:00Z");

describe("buildTimelineData", () => {
  it("returns an empty window for no buckets", () => {
    const t = buildTimelineData([], 7, now);
    expect(t.data).toEqual([]);
    expect(t.series).toEqual([]);
    expect(t.collectionStart).toBeNull();
  });

  it("densifies quiet days so gaps keep their width", () => {
    const t = buildTimelineData(
      [
        { day: "2026-09-01T00:00:00Z", category: "cyber", count: 2 },
        { day: "2026-09-06T00:00:00Z", category: "cyber", count: 1 },
      ],
      7,
      now,
    );
    // Starts at the first day with data — not at the window edge — and says
    // so, since the day before collection began is uncollected, not quiet.
    expect(t.data.map((r) => r.day)).toEqual([
      "2026-09-01", "2026-09-02", "2026-09-03", "2026-09-04", "2026-09-05", "2026-09-06",
    ]);
    expect(t.data.map((r) => r.cyber)).toEqual([2, 0, 0, 0, 0, 1]);
    expect(t.collectionStart).toBe("2026-09-01");
  });

  it("covers the whole window when data reaches its start", () => {
    const t = buildTimelineData(
      [
        { day: "2026-08-31T00:00:00Z", category: "sabotage", count: 1 },
        { day: "2026-09-04T00:00:00Z", category: "sabotage", count: 1 },
      ],
      7,
      now,
    );
    expect(t.data[0]!.day).toBe("2026-08-31");
    expect(t.data).toHaveLength(7);
    expect(t.collectionStart).toBeNull();
  });

  it("folds minor categories into Other and keeps rows dense", () => {
    const t = buildTimelineData(
      [
        { day: "2026-09-06T00:00:00Z", category: "energy", count: 1 },
        { day: "2026-09-06T00:00:00Z", category: "political", count: 2 },
        { day: "2026-09-05T00:00:00Z", category: "cyber", count: 1 },
      ],
      2,
      now,
    );
    expect(t.series.map((s) => s.key)).toEqual(["cyber", "other"]);
    expect(t.data.find((r) => r.day === "2026-09-06")).toMatchObject({ other: 3, cyber: 0 });
    expect(rowTotal(t.data[1]!, t.series)).toBe(3);
  });
});
