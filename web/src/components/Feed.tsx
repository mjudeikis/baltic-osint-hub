import { Incident } from "../api";
import { categoryColor, categoryLabel, severityColor, SEVERITY_LABELS } from "../taxonomy";

export default function Feed({ incidents }: { incidents: Incident[] }) {
  if (incidents.length === 0) {
    return <p style={{ color: "var(--text-muted)" }}>No incidents match the filters.</p>;
  }
  return (
    <div>
      {incidents.map((inc) => (
        <article className="feed-item" key={inc.id}>
          <div className="head">
            <span
              className="sev"
              style={{ background: severityColor(inc.severity) }}
              title={`Severity ${inc.severity}`}
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
