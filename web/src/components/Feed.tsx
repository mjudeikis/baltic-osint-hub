import { Incident } from "../api";
import {
  categoryColor,
  categoryLabel,
  cssColor,
  severityColor,
  SEVERITY_LABELS,
  toneDef,
  CREDIBILITY,
} from "../taxonomy";

export default function Feed({ incidents }: { incidents: Incident[] }) {
  if (incidents.length === 0) {
    return <p style={{ color: "var(--text-muted)" }}>No incidents match the filters.</p>;
  }
  return (
    <div>
      {incidents.map((inc) => (
        <article
          className="feed-item"
          key={inc.id}
          data-state-media={inc.credibility === "state-controlled" || undefined}
        >
          <div className="head">
            <span
              className="tone"
              style={{ color: cssColor(toneDef(inc.tone).cssVar) }}
              title="Direction for regional security"
            >
              {toneDef(inc.tone).symbol} {toneDef(inc.tone).label}
            </span>
            <span
              className="sev"
              style={{ background: severityColor(inc.severity) }}
              title={`Severity ${inc.severity} — how consequential, independent of direction`}
            >
              S{inc.severity} {SEVERITY_LABELS[inc.severity]}
            </span>
            <span
              className="chip cat"
              style={{ borderLeftColor: categoryColor(inc.category) }}
            >
              {categoryLabel(inc.category)}
            </span>
            <span className="chip">{inc.countries.join(" · ")}</span>
            <time dateTime={inc.occurred_at}>
              {new Date(inc.occurred_at).toLocaleDateString("en-GB", {
                day: "numeric",
                month: "short",
                year: "numeric",
              })}
            </time>
            <span className="src">{inc.source}</span>
            {inc.credibility === "state-controlled" && (
              <span
                className="cred-warn"
                title={CREDIBILITY["state-controlled"].label}
              >
                ⚠ state media — adversary messaging, not verified reporting
              </span>
            )}
          </div>
          <div>
            <a href={inc.url} target="_blank" rel="noopener noreferrer">
              {inc.title}
            </a>
          </div>
          {inc.summary && <p className="summary">{inc.summary}</p>}
        </article>
      ))}
    </div>
  );
}
