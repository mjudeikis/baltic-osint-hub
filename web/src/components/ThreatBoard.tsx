import { SummaryCell } from "../api";
import { COUNTRIES, COUNTRY_NAMES, categoryLabel, cssColor, TONES } from "../taxonomy";

interface Level {
  label: string;
  cssVar: string;
  symbol: string;
}

// Minimum adverse items in the prior 28 days before a trend comparison means
// anything. Below this the baseline is mostly an artefact of how long the
// collector has been running, and a "+7500%" reading is noise dressed as alarm.
const MIN_BASELINE_SAMPLES = 8;

// Mirrors the regional posture rules so the board can never contradict the
// banner above it: level comes from adverse severity, never from raw volume.
function level(adverse: number, maxSev: number, favourable: number): Level {
  if (maxSev >= 5) return { label: "Critical", cssVar: "--status-critical", symbol: "▲" };
  if (maxSev >= 4) return { label: "Serious", cssVar: "--status-serious", symbol: "▲" };
  if (adverse >= 3 && favourable <= adverse)
    return { label: "Elevated", cssVar: "--status-warning", symbol: "△" };
  if (adverse > 0) return { label: "Watchful", cssVar: "--status-good", symbol: "○" };
  return { label: "Quiet", cssVar: "--status-good", symbol: "○" };
}

export default function ThreatBoard({
  cells,
  onSelect,
}: {
  cells: SummaryCell[];
  // Drill-through: a country/category pair filters the incident feed, so the
  // numbers on this board lead to the items behind them.
  onSelect: (country: string, category: string) => void;
}) {
  return (
    <div className="board" role="list" aria-label="Adverse activity by country">
      {COUNTRIES.map((cc) => {
        const rows = cells.filter((c) => c.country === cc);
        // Coalesce every field: right after a deploy the fresh bundle can be
        // served a still-cached response from the previous schema, and a bare
        // sum would render NaN across the board.
        const sum = (pick: (r: SummaryCell) => number | undefined) =>
          rows.reduce((s, r) => s + (pick(r) ?? 0), 0);

        const adverse = sum((r) => r.recent_adverse);
        const favourable = sum((r) => r.recent_favourable);
        const baseline = sum((r) => r.baseline);
        const samples = sum((r) => r.baseline_samples);
        const maxSev = Math.max(0, ...rows.map((r) => r.max_severity ?? 0));
        const lv = level(adverse, maxSev, favourable);

        // Only compare against the baseline once there is enough of one.
        const comparable = samples >= MIN_BASELINE_SAMPLES && baseline > 0;
        const deltaPct = comparable
          ? Math.round(((adverse - baseline) / baseline) * 100)
          : null;

        const top = rows
          .filter((r) => r.recent_adverse > 0)
          .sort((a, b) => b.recent_adverse - a.recent_adverse)
          .slice(0, 3);

        return (
          <div className="tile" role="listitem" key={cc}>
            <div className="country">
              <span>{COUNTRY_NAMES[cc]}</span>
              <span className="level" style={{ color: cssColor(lv.cssVar) }}>
                <span className="dot" style={{ background: cssColor(lv.cssVar) }} />
                {lv.symbol} {lv.label}
              </span>
            </div>

            <div className="count">{adverse}</div>
            <div className="delta">
              adverse in 7 days
              {deltaPct !== null
                ? ` · ${deltaPct >= 0 ? "+" : ""}${deltaPct}% vs prior 4-week avg`
                : " · baseline still building"}
            </div>

            <div className="delta" style={{ marginTop: 4, color: cssColor(TONES.positive.cssVar) }}>
              {TONES.positive.symbol} {favourable} favourable
            </div>

            <ul>
              {top.map((r) => (
                <li key={r.category}>
                  <button
                    className="tile-drill"
                    onClick={() => onSelect(cc, r.category)}
                    title={`Show ${categoryLabel(r.category)} incidents in ${COUNTRY_NAMES[cc]}`}
                  >
                    <span>{categoryLabel(r.category)}</span>
                    <span>{r.recent_adverse}</span>
                  </button>
                </li>
              ))}
            </ul>
          </div>
        );
      })}
    </div>
  );
}
