import { useMemo } from "react";
import {
  BarChart,
  Bar,
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
export default function Timeline({ buckets }: { buckets: TimelineBucket[] }) {
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

  return (
    <>
      <ResponsiveContainer width="100%" height={260}>
        <BarChart data={data} barCategoryGap={1}>
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
        </BarChart>
      </ResponsiveContainer>
      <div className="legend" aria-hidden={false}>
        {series.map((s) => (
          <span className="key" key={s.key}>
            <span className="swatch" style={{ background: s.color }} />
            {s.label}
          </span>
        ))}
      </div>
    </>
  );
}
