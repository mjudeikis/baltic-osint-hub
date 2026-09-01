import { useMemo, useState } from "react";
import {
  BarChart,
  Bar,
  Brush,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  ResponsiveContainer,
} from "recharts";
import { TimelineBucket } from "../api";
import { CATEGORIES, categoryLabel, cssColor } from "../taxonomy";

// Stacked daily counts. The 8 chart categories use their fixed slots; folded
// categories (energy, political) merge into a single muted "Other" series so
// the stack never exceeds the validated 8-slot palette.
//
// The chart is also the date control: drag the brush to narrow the window, or
// click a bar to pull the feed to that single day. Until now it was a picture
// you could only look at, while every comparable dashboard lets you scrub.
export default function Timeline({
  buckets,
  days,
  onSelectDay,
}: {
  buckets: TimelineBucket[];
  // The requested window length, so quiet days can be drawn as quiet days.
  days: number;
  onSelectDay?: (day: string) => void;
}) {
  // Brush indices, or null for "the whole window". Held here rather than
  // lifted, because narrowing the view is a reading gesture, not a filter the
  // API needs to know about.
  const [range, setRange] = useState<[number, number] | null>(null);
  const { data, series } = useMemo(() => {
    const byDay = new Map<string, Record<string, number | string>>();
    for (const b of buckets) {
      const day = b.day.slice(0, 10);
      const def = CATEGORIES.find((c) => c.key === b.category);
      const key = def?.folded ? "other" : b.category;
      const row = byDay.get(day) ?? { day };
      row[key] = ((row[key] as number) ?? 0) + b.count;
      byDay.set(day, row);
    }
    // Densify: a categorical axis renders only the days present, so a
    // six-day calm gap would collapse into two adjacent bars and the chart
    // would manufacture escalation. Quiet days are the answer to "is this
    // week unusual?" and must occupy their real width. The window starts at
    // the first day with data (a fresh database has no meaningful zeros
    // before collection began) and runs to today with every day present.
    if (byDay.size > 0) {
      const first = [...byDay.keys()].sort()[0];
      const start = new Date(`${first}T00:00:00Z`);
      const windowStart = new Date();
      windowStart.setUTCDate(windowStart.getUTCDate() - (days - 1));
      const from = start > windowStart ? start : windowStart;
      for (const d = new Date(from); d <= new Date(); d.setUTCDate(d.getUTCDate() + 1)) {
        const key = d.toISOString().slice(0, 10);
        if (!byDay.has(key)) byDay.set(key, { day: key });
      }
    }
    const data = [...byDay.values()].sort((a, b) =>
      String(a.day).localeCompare(String(b.day)),
    );
    const present = new Set(buckets.map((b) => b.category));
    const series = CATEGORIES.filter((c) => !c.folded && present.has(c.key)).map(
      (c) => ({ key: c.key, label: c.label, color: cssColor(c.cssVar) }),
    );
    if (CATEGORIES.some((c) => c.folded && present.has(c.key))) {
      series.push({ key: "other", label: "Other", color: cssColor("--series-other") });
    }
    // Recharts stacks break on missing keys — make every row dense.
    for (const row of data) {
      for (const s of series) {
        if (row[s.key] === undefined) row[s.key] = 0;
      }
    }
    return { data, series };
  }, [buckets]);

  if (data.length === 0) {
    return <p style={{ color: "var(--text-muted)" }}>No incidents in this window yet.</p>;
  }

  const [from, to] = range ?? [0, data.length - 1];
  const shown = data.slice(from, to + 1);
  const total = shown.reduce(
    (sum, row) =>
      sum + series.reduce((n, s) => n + ((row[s.key] as number) ?? 0), 0),
    0,
  );
  const narrowed = range !== null && shown.length < data.length;

  return (
    <>
      <ResponsiveContainer width="100%" height={260}>
        <BarChart
          data={data}
          barCategoryGap={1}
          onClick={(e: { activeLabel?: string }) => {
            if (onSelectDay && e?.activeLabel) onSelectDay(e.activeLabel);
          }}
          style={onSelectDay ? { cursor: "pointer" } : undefined}
        >
          <CartesianGrid stroke={cssColor("--grid")} vertical={false} />
          <XAxis
            dataKey="day"
            tick={{ fill: cssColor("--text-muted"), fontSize: 11 }}
            tickLine={false}
            axisLine={{ stroke: cssColor("--baseline") }}
            tickFormatter={(d: string) => d.slice(5)}
          />
          <YAxis
            allowDecimals={false}
            tick={{ fill: cssColor("--text-muted"), fontSize: 11 }}
            tickLine={false}
            axisLine={false}
            width={28}
          />
          <Tooltip
            cursor={{ fill: cssColor("--grid"), opacity: 0.5 }}
            contentStyle={{
              background: cssColor("--surface-1"),
              border: `1px solid ${cssColor("--baseline")}`,
              borderRadius: 6,
              fontSize: 12,
            }}
            labelStyle={{ color: cssColor("--text-primary") }}
            formatter={(value: number, name: string) => [value, categoryLabel(name)]}
          />
          {series.map((s, i) => (
            <Bar
              key={s.key}
              dataKey={s.key}
              stackId="a"
              fill={s.color}
              stroke={cssColor("--surface-1")}
              strokeWidth={1}
              isAnimationActive={false}
              radius={i === series.length - 1 ? [3, 3, 0, 0] : undefined}
            />
          ))}
          <Brush
            dataKey="day"
            height={22}
            travellerWidth={10}
            stroke={cssColor("--baseline")}
            fill={cssColor("--surface-1")}
            tickFormatter={(d: string) => String(d).slice(5)}
            onChange={(r: { startIndex?: number; endIndex?: number }) => {
              if (r?.startIndex === undefined || r?.endIndex === undefined) return;
              const whole = r.startIndex === 0 && r.endIndex === data.length - 1;
              setRange(whole ? null : [r.startIndex, r.endIndex]);
            }}
          />
        </BarChart>
      </ResponsiveContainer>
      {narrowed && (
        <p className="brush-note">
          {shown[0].day as string} to {shown[shown.length - 1].day as string} —{" "}
          <strong>{total}</strong> {total === 1 ? "event" : "events"} in view.{" "}
          <button className="linklike" onClick={() => setRange(null)}>
            reset
          </button>
        </p>
      )}
      {onSelectDay && (
        <p className="brush-hint">
          Drag the strip below the chart to narrow the range; click a bar to
          show that day in the feed.
        </p>
      )}
      {/* The bar-click drill-through is pointer-only; this is its keyboard
          path. Off-screen until focused, then a plain labelled select. */}
      {onSelectDay && (
        <div className="sr-day-nav">
          <label htmlFor="timeline-day-picker">Show a single day in the feed:</label>
          <select
            id="timeline-day-picker"
            value=""
            onChange={(e) => {
              if (e.target.value) onSelectDay(e.target.value);
            }}
          >
            <option value="">Choose a day…</option>
            {data.map((row) => {
              const dayTotal = series.reduce(
                (n, s) => n + ((row[s.key] as number) ?? 0),
                0,
              );
              return (
                <option key={String(row.day)} value={String(row.day)}>
                  {String(row.day)} — {dayTotal} {dayTotal === 1 ? "event" : "events"}
                </option>
              );
            })}
          </select>
        </div>
      )}
      <div className="legend">
        {series.map((s) => (
          <span className="key" key={s.key}>
            <span className="swatch" style={{ background: s.color }} />
            {s.label}
          </span>
        ))}
      </div>
      {/* The counting rule, stated where the numbers are: commentary is not
          an event, so it never inflates a chart titled "incidents". */}
      <p className="brush-hint">
        Counts events of severity 2 and above. Severity-1 analysis and
        commentary stay in the incident feed but are not counted here.
      </p>
    </>
  );
}
