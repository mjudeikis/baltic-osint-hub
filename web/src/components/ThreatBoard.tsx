import { SummaryCell } from "../api";
import { COUNTRIES, COUNTRY_NAMES, categoryLabel, cssColor } from "../taxonomy";

interface Level {
  label: string;
  cssVar: string;
  symbol: string;
}

// Level always ships as color + symbol + label, never color alone.
function level(recent: number, baseline: number, maxSev: number): Level {
  if (maxSev >= 5) return { label: "Critical", cssVar: "--status-critical", symbol: "▲" };
  if (maxSev >= 4 || (recent >= 5 && recent > baseline * 2))
    return { label: "Serious", cssVar: "--status-serious", symbol: "▲" };
  if (recent > baseline * 1.3 && recent >= 2)
    return { label: "Elevated", cssVar: "--status-warning", symbol: "△" };
  return { label: "Steady", cssVar: "--status-good", symbol: "○" };
}

export default function ThreatBoard({ cells }: { cells: SummaryCell[] }) {
  return (
    <div className="board" role="list" aria-label="Threat level by country">
      {COUNTRIES.map((cc) => {
        const rows = cells.filter((c) => c.country === cc);
        const recent = rows.reduce((s, r) => s + r.recent, 0);
        const baseline = rows.reduce((s, r) => s + r.baseline, 0);
        const maxSev = Math.max(0, ...rows.map((r) => r.max_severity));
        const lv = level(recent, baseline, maxSev);
        const top = rows
          .filter((r) => r.recent > 0)
          .sort((a, b) => b.recent - a.recent)
          .slice(0, 3);
        const deltaPct =
          baseline > 0 ? Math.round(((recent - baseline) / baseline) * 100) : null;
        return (
          <div className="tile" role="listitem" key={cc}>
            <div className="country">
              <span>{COUNTRY_NAMES[cc]}</span>
              <span className="level" style={{ color: cssColor(lv.cssVar) }}>
                <span className="dot" style={{ background: cssColor(lv.cssVar) }} />
                {lv.symbol} {lv.label}
              </span>
            </div>
            <div className="count">{recent}</div>
            <div className="delta">
              incidents in 7 days
              {deltaPct !== null &&
                ` · ${deltaPct >= 0 ? "+" : ""}${deltaPct}% vs prior 4-week avg`}
            </div>
            <ul>
              {top.map((r) => (
                <li key={r.category}>
                  <span>{categoryLabel(r.category)}</span>
                  <span>{r.recent}</span>
                </li>
              ))}
            </ul>
          </div>
        );
      })}
    </div>
  );
}
