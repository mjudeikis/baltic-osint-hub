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
import { Freshness, staleNote } from "../freshness";
import { buildTimelineData, rowTotal } from "../timelineData";
import { categoryLabel, cssColor } from "../taxonomy";

// Stacked daily counts; the data shaping lives in ../timelineData so it can
// be tested without a DOM.
//
// The chart is also the date control: drag the brush to narrow the window, or
// click a bar to pull the feed to that single day. Until now it was a picture
// you could only look at, while every comparable dashboard lets you scrub.
export default function Timeline({
  buckets,
  days,
  fresh,
  onSelectDay,
}: {
  buckets: TimelineBucket[];
  // The requested window length, so quiet days can be drawn as quiet days.
  days: number;
  // Collector freshness: an empty chart over a stalled pipeline is "no
  // data", and must say so rather than "no incidents".
  fresh: Freshness;
  onSelectDay?: (day: string) => void;
}) {
  // Brush indices, or null for "the whole window". Held here rather than
  // lifted, because narrowing the view is a reading gesture, not a filter the
  // API needs to know about. The indices are positions in the data for one
  // window length, so the selection remembers which `days` it was made for
  // and is dropped the moment the window changes — a 90-day brush applied
  // to 7 days of data would slice past the end.
  const [brush, setBrush] = useState<{ days: number; range: [number, number] } | null>(null);
  const range = brush !== null && brush.days === days ? brush.range : null;
  const setRange = (r: [number, number] | null) => setBrush(r ? { days, range: r } : null);
  const { data, series, collectionStart } = useMemo(() => {
    // `new Date()` is read here deliberately: the chart runs to today, and
    // recomputing on each buckets/days change is the right cadence for it.
    const built = buildTimelineData(buckets, days);
    return {
      ...built,
      series: built.series.map((s) => ({ ...s, color: cssColor(s.cssVar) })),
    };
  }, [buckets, days]);

  if (data.length === 0) {
    return (
      <p style={{ color: "var(--text-muted)" }} role={fresh.stale ? "status" : undefined}>
        {fresh.stale
          ? staleNote(fresh)
          : "No incidents in this window yet."}
      </p>
    );
  }

  // A range that no longer fits the data (a refresh that dropped a day, or a
  // stale brush) falls back to the whole window rather than slicing empty.
  const valid = range !== null && range[0] >= 0 && range[1] < data.length && range[0] <= range[1];
  const [from, to] = valid ? range : [0, data.length - 1];
  const shown = data.slice(from, to + 1);
  const first = shown[0];
  const last = shown[shown.length - 1];
  const total = shown.reduce((sum, row) => sum + rowTotal(row, series), 0);
  const narrowed = valid && shown.length < data.length && first !== undefined && last !== undefined;

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
            travellerWidth={12}
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
          {first?.day} to {last?.day} —{" "}
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
      {/* The bar-click drill-through and the brush are pointer-only; these
          are their keyboard paths. Off-screen until focused, then plain
          labelled selects. */}
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
              const dayTotal = rowTotal(row, series);
              return (
                <option key={String(row.day)} value={String(row.day)}>
                  {String(row.day)} — {dayTotal} {dayTotal === 1 ? "event" : "events"}
                </option>
              );
            })}
          </select>
        </div>
      )}
      <div className="sr-day-nav">
        <label htmlFor="timeline-range-from">Narrow the chart range — from:</label>
        <select
          id="timeline-range-from"
          value={from}
          onChange={(e) => {
            const i = Number(e.target.value);
            setRange([Math.min(i, to), Math.max(i, to)]);
          }}
        >
          {data.map((row, i) => (
            <option key={String(row.day)} value={i}>
              {String(row.day)}
            </option>
          ))}
        </select>
        <label htmlFor="timeline-range-to">to:</label>
        <select
          id="timeline-range-to"
          value={to}
          onChange={(e) => {
            const j = Number(e.target.value);
            setRange([Math.min(from, j), Math.max(from, j)]);
          }}
        >
          {data.map((row, i) => (
            <option key={String(row.day)} value={i}>
              {String(row.day)}
            </option>
          ))}
        </select>
      </div>
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
        {collectionStart &&
          ` Collection of counted events begins ${collectionStart} — earlier days are uncollected, not quiet.`}
      </p>
      {/* The chart's data, as a table only assistive tech sees: Recharts
          tooltips are hover-only, so per-category values were unreachable
          without a pointer. */}
      <table className="sr-only">
        <caption>Incidents per day by category</caption>
        <thead>
          <tr>
            <th scope="col">Day</th>
            {series.map((s) => (
              <th key={s.key} scope="col">
                {s.label}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {data.map((row) => (
            <tr key={String(row.day)}>
              <th scope="row">{String(row.day)}</th>
              {series.map((s) => (
                <td key={s.key}>{(row[s.key] as number | undefined) ?? 0}</td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
