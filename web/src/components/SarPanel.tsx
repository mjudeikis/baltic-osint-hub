import { SarAOI } from "../api";
import { cssColor } from "../taxonomy";

// Sentinel-1 change detection: one card per monitored site. Status is carried
// by icon + label + colour together, never colour alone.
export default function SarPanel({
  aois,
  onFocus,
  focused,
}: {
  aois: SarAOI[];
  onFocus: (key: string) => void;
  focused: string | null;
}) {
  if (aois.length === 0) {
    return <p style={{ color: "var(--text-muted)" }}>No monitored sites configured.</p>;
  }
  const measured = aois.filter((a) => a.series.length > 0);
  if (measured.length === 0) {
    return (
      <p style={{ color: "var(--text-muted)" }}>
        No radar measurements yet — add Copernicus credentials and run the
        collector. The first run backfills 180 days of Sentinel-1 history, so
        baselines are usable immediately.
      </p>
    );
  }
  return (
    <>
      <div className="board">
        {aois.map((a) => (
          <SarCard key={a.key} aoi={a} onFocus={onFocus} focused={focused === a.key} />
        ))}
      </div>
      <p className="legend" style={{ color: "var(--text-muted)", display: "block" }}>
        Sites are watched on <strong>Russian and Belarusian territory</strong>, ordered
        by how far they sit behind the border — equipment already visible at a NATO
        crossing carries no warning, so the value is in the depth. Bright-pixel
        fraction from Sentinel-1 VV backscatter (descending passes, 20 m) is a proxy
        for metallic scatterers such as vehicles, aircraft and rolling stock. A
        flagged site means the latest pass sits far above its own 180-day baseline;
        it is a prompt to look at the imagery, not a confirmed deployment. Weather,
        farm machinery and construction all move this number.
      </p>
    </>
  );
}

function SarCard({
  aoi,
  onFocus,
  focused,
}: {
  aoi: SarAOI;
  onFocus: (key: string) => void;
  focused: boolean;
}) {
  const enough = aoi.baseline >= 8;
  const status = !enough
    ? { label: "Baselining", symbol: "○", cssVar: "--text-muted" }
    : aoi.scene_shifted
      ? // Conditions unlike anything in the reference period: no honest
        // comparison is possible, so say so rather than raise or clear.
        { label: "Conditions changed", symbol: "◌", cssVar: "--status-warning" }
      : aoi.anomaly
        ? { label: "Change detected", symbol: "▲", cssVar: "--status-serious" }
        : { label: "Nominal", symbol: "○", cssVar: "--status-good" };

  return (
    <div className="tile" data-focused={focused || undefined}>
      <div className="country">
        <span>{aoi.label}</span>
        <span className="level" style={{ color: cssColor(status.cssVar) }}>
          <span className="dot" style={{ background: cssColor(status.cssVar) }} />
          {status.symbol} {status.label}
        </span>
      </div>
      <div className="delta" style={{ marginTop: 2 }}>
        {aoi.country} · {aoi.kind}
        {aoi.side === "adversary" && aoi.depth_km > 0 && (
          <> · ~{aoi.depth_km} km behind the border</>
        )}
        {aoi.side === "friendly" && <> · NATO side</>}
      </div>

      {aoi.series.length > 0 ? (
        <>
          <div className="count" style={{ fontSize: 22 }}>
            {(aoi.latest * 100).toFixed(1)}%
          </div>
          <div className="delta">
            bright pixels · baseline {(aoi.median * 100).toFixed(1)}%
            {enough && aoi.anomaly && aoi.zscore > 0 && ` · ${aoi.zscore.toFixed(1)}σ above`}
          </div>
          <Sparkline series={aoi.series} anomaly={aoi.anomaly && enough && !aoi.scene_shifted} />
          {aoi.scene_shifted && (
            <div className="delta" style={{ marginTop: 4 }}>
              scene-wide backscatter shifted (weather, harvest or sea state) —
              comparison suppressed
            </div>
          )}
          <div className="delta">
            {aoi.series.length} passes · latest{" "}
            {new Date(aoi.series[aoi.series.length - 1].start).toLocaleDateString("en-GB", {
              day: "numeric",
              month: "short",
            })}
          </div>
        </>
      ) : (
        <div className="delta" style={{ marginTop: 8 }}>
          no measurements yet
        </div>
      )}

      {aoi.note && <p className="site-note">{aoi.note}</p>}

      <div className="site-actions">
        <button className="linklike" onClick={() => onFocus(aoi.key)}>
          ◎ show on map
        </button>
        <a href={aoi.browser_url} target="_blank" rel="noopener noreferrer">
          🛰 inspect imagery ↗
        </a>
      </div>
    </div>
  );
}

// Inline sparkline: a chart this small earns no axes — the value above it is
// the label, and the shape carries the trend.
function Sparkline({
  series,
  anomaly,
}: {
  series: { bright_fraction: number }[];
  anomaly: boolean;
}) {
  const w = 200;
  const h = 34;
  const pad = 3;
  const values = series.map((s) => s.bright_fraction);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const stroke = cssColor(anomaly ? "--status-serious" : "--series-1");

  const x = (i: number) =>
    values.length === 1 ? w / 2 : pad + (i / (values.length - 1)) * (w - 2 * pad);
  const y = (v: number) => h - pad - ((v - min) / span) * (h - 2 * pad);

  const d = values.map((v, i) => `${i === 0 ? "M" : "L"} ${x(i).toFixed(1)} ${y(v).toFixed(1)}`).join(" ");
  const lastX = x(values.length - 1);
  const lastY = y(values[values.length - 1]);

  return (
    <svg
      width="100%"
      viewBox={`0 0 ${w} ${h}`}
      preserveAspectRatio="none"
      style={{ display: "block", marginTop: 8 }}
      role="img"
      aria-label={`Trend over ${values.length} radar passes`}
    >
      <path d={d} fill="none" stroke={stroke} strokeWidth={2} vectorEffect="non-scaling-stroke" />
      <circle cx={lastX} cy={lastY} r={3} fill={stroke} />
    </svg>
  );
}
