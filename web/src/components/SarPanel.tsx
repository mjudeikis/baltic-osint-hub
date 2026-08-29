import { useMemo, useState } from "react";
import { SarAOI } from "../api";
import { cssColor } from "../taxonomy";

// A compact, filterable table rather than one card per site: at ~50 sites the
// card layout buried the two rows that actually needed attention.

type StatusKey = "flagged" | "shifted" | "baselining" | "nominal";

// Derived once so sorting, filtering and display can never disagree.
function statusOf(a: SarAOI): StatusKey {
  if (a.baseline < 8) return "baselining";
  if (a.scene_shifted) return "shifted";
  if (a.anomaly) return "flagged";
  return "nominal";
}

const STATUS: Record<StatusKey, { label: string; symbol: string; cssVar: string }> = {
  flagged: { label: "Change detected", symbol: "▲", cssVar: "--status-serious" },
  shifted: { label: "Conditions changed", symbol: "◌", cssVar: "--status-warning" },
  baselining: { label: "Baselining", symbol: "○", cssVar: "--text-muted" },
  nominal: { label: "Nominal", symbol: "○", cssVar: "--status-good" },
};

const CLASS_LABEL: Record<string, string> = {
  empty: "Empty",
  occupied: "Occupied",
  hollow: "Hollow",
};

// Anything needing attention first; within that, deeper sites lead because
// they buy more warning time.
const RANK: Record<StatusKey, number> = {
  flagged: 0,
  shifted: 1,
  baselining: 2,
  nominal: 3,
};

export default function SarPanel({
  aois,
  onFocus,
  focused,
}: {
  aois: SarAOI[];
  onFocus: (key: string) => void;
  focused: string | null;
}) {
  const [classFilter, setClassFilter] = useState("");
  const [statusFilter, setStatusFilter] = useState<StatusKey | "">("");
  const [countryFilter, setCountryFilter] = useState("");

  const rows = useMemo(
    () =>
      aois
        .map((a) => ({ aoi: a, status: statusOf(a) }))
        .sort(
          (x, y) => RANK[x.status] - RANK[y.status] || y.aoi.depth_km - x.aoi.depth_km,
        ),
    [aois],
  );

  const counts = useMemo(() => {
    const c: Partial<Record<StatusKey, number>> = {};
    for (const r of rows) c[r.status] = (c[r.status] ?? 0) + 1;
    return c;
  }, [rows]);

  const countries = useMemo(() => [...new Set(aois.map((a) => a.country))].sort(), [aois]);

  if (aois.length === 0) {
    return <p style={{ color: "var(--text-muted)" }}>No monitored sites configured.</p>;
  }
  if (rows.every((r) => r.aoi.series.length === 0)) {
    return (
      <p style={{ color: "var(--text-muted)" }}>
        No radar measurements yet — add Copernicus credentials and run the collector.
        The first run backfills 180 days of history, so baselines are usable immediately.
      </p>
    );
  }

  const shown = rows.filter(
    (r) =>
      (!classFilter || r.aoi.class === classFilter) &&
      (!statusFilter || r.status === statusFilter) &&
      (!countryFilter || r.aoi.country === countryFilter),
  );
  const filtered = Boolean(classFilter || statusFilter || countryFilter);

  return (
    <>
      {/* The counts double as filters — clicking "change detected" is the
          fastest path to the only rows that usually matter. */}
      <div className="sar-summary">
        <strong>{aois.length} sites</strong>
        {(Object.keys(RANK) as StatusKey[]).map((k) =>
          counts[k] ? (
            <button
              key={k}
              className="sar-count"
              aria-pressed={statusFilter === k}
              onClick={() => setStatusFilter(statusFilter === k ? "" : k)}
            >
              <span style={{ color: cssColor(STATUS[k].cssVar) }}>{STATUS[k].symbol}</span>{" "}
              {counts[k]} {STATUS[k].label.toLowerCase()}
            </button>
          ) : null,
        )}
      </div>

      <div className="filters" style={{ marginTop: 8 }}>
        <select
          value={classFilter}
          onChange={(e) => setClassFilter(e.target.value)}
          aria-label="Baseline class"
        >
          <option value="">All baselines</option>
          <option value="empty">Empty — cleanest indicator</option>
          <option value="occupied">Permanently occupied</option>
          <option value="hollow">Hollow — unit committed elsewhere</option>
        </select>
        <select
          value={countryFilter}
          onChange={(e) => setCountryFilter(e.target.value)}
          aria-label="Country"
        >
          <option value="">All countries</option>
          {countries.map((c) => (
            <option key={c} value={c}>
              {c}
            </option>
          ))}
        </select>
        {filtered && (
          <button
            onClick={() => {
              setClassFilter("");
              setStatusFilter("");
              setCountryFilter("");
            }}
          >
            Clear filters
          </button>
        )}
      </div>

      <div className="sar-table-wrap">
        <table className="sar-table">
          <thead>
            <tr>
              <th>Site</th>
              <th>Baseline</th>
              <th className="num">Depth</th>
              <th className="num">Now</th>
              <th className="num">Typical</th>
              <th>Trend</th>
              <th>Status</th>
              <th aria-label="Imagery" />
            </tr>
          </thead>
          <tbody>
            {shown.map(({ aoi, status }) => (
              <tr key={aoi.key} data-focused={focused === aoi.key || undefined}>
                <td>
                  <button
                    className="linklike"
                    onClick={() => onFocus(aoi.key)}
                    title={aoi.note || "Show on map"}
                  >
                    {aoi.label}
                  </button>
                  <div className="sar-sub">
                    {aoi.country} · {aoi.kind}
                  </div>
                </td>
                <td>
                  <span className="site-class" data-class={aoi.class}>
                    {CLASS_LABEL[aoi.class] ?? aoi.class}
                  </span>
                </td>
                <td className="num">{aoi.depth_km} km</td>
                <td className="num">
                  {aoi.series.length ? `${(aoi.latest * 100).toFixed(1)}%` : "—"}
                </td>
                <td className="num muted">
                  {aoi.series.length ? `${(aoi.median * 100).toFixed(1)}%` : "—"}
                </td>
                <td>
                  {aoi.series.length > 1 && (
                    <Sparkline series={aoi.series} alert={status === "flagged"} />
                  )}
                </td>
                <td>
                  <span className="level" style={{ color: cssColor(STATUS[status].cssVar) }}>
                    {STATUS[status].symbol} {STATUS[status].label}
                  </span>
                </td>
                <td className="num">
                  <a
                    href={aoi.browser_url}
                    target="_blank"
                    rel="noopener noreferrer"
                    title="Inspect the imagery"
                  >
                    🛰
                  </a>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {shown.length === 0 && (
        <p style={{ color: "var(--text-muted)" }}>No sites match these filters.</p>
      )}

      <p className="legend" style={{ color: "var(--text-muted)", display: "block" }}>
        Sites sit on <strong>Russian and Belarusian territory</strong>, ordered with
        anything needing attention first and then by depth, since deeper sites buy more
        warning. Weight the <strong>empty</strong> baselines most: in the 2021–22
        build-up, detection at permanently occupied garrisons fired identically in a
        quiet spring and a pre-invasion winter, while change at empty ranges and rail
        ramps discriminated. Values are the bright-pixel fraction from Sentinel-1 VV
        backscatter — a proxy for vehicles, aircraft and rolling stock, not a count of
        them. A flagged site is a prompt to open the imagery, not a confirmed
        deployment.
      </p>
    </>
  );
}

// Sized for a table cell: no axes, because the numbers beside it are the label
// and the shape only has to carry the trend.
function Sparkline({
  series,
  alert,
}: {
  series: { bright_fraction: number }[];
  alert: boolean;
}) {
  const w = 74;
  const h = 18;
  const pad = 2;
  const values = series.map((s) => s.bright_fraction);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const stroke = cssColor(alert ? "--status-serious" : "--series-1");
  const x = (i: number) => pad + (i / (values.length - 1)) * (w - 2 * pad);
  const y = (v: number) => h - pad - ((v - min) / span) * (h - 2 * pad);
  const d = values
    .map((v, i) => `${i === 0 ? "M" : "L"} ${x(i).toFixed(1)} ${y(v).toFixed(1)}`)
    .join(" ");

  return (
    <svg
      width={w}
      height={h}
      viewBox={`0 0 ${w} ${h}`}
      role="img"
      aria-label={`Trend over ${values.length} passes`}
      style={{ display: "block" }}
    >
      <path d={d} fill="none" stroke={stroke} strokeWidth={1.5} />
      <circle cx={x(values.length - 1)} cy={y(values[values.length - 1])} r={2} fill={stroke} />
    </svg>
  );
}
