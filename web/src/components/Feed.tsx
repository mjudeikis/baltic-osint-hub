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
          // Stable anchor so a single incident can be linked to directly.
          id={`incident-${inc.id}`}
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
            {/* Corroboration: how many independent outlets carried this, not
                how many articles exist. Absent while unclustered, because an
                incident nobody has checked is not the same as one that failed
                the check. */}
            {inc.confidence_label && (
              <span
                className="corrob"
                data-level={inc.confidence_label.replace(/ /g, "-")}
                title={
                  inc.sources?.length
                    ? `Reported by: ${inc.sources.join(", ")}`
                    : undefined
                }
              >
                {inc.confidence_label}
                {inc.reports > 1 && ` · ${inc.reports} reports`}
              </span>
            )}
            <a
              className="permalink"
              href={`#incident-${inc.id}`}
              title="Link to this incident"
              aria-label="Link to this incident"
            >
              #
            </a>
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
