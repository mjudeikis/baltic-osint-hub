import { Incident } from "../api";
import {
  categoryColor,
  categoryLabel,
  severityColor,
  severityTextColor,
  SEVERITY_LABELS,
  toneDef,
  CREDIBILITY,
} from "../taxonomy";

// Items are grouped under day headers: at up to 200 rows the date is the main
// scanning axis, and a header carries it once instead of once per row. Items
// arrive date-ordered from the API, so grouping is a single pass.
function groupByDay(incidents: Incident[]): { day: string; items: Incident[] }[] {
  const groups: { day: string; items: Incident[] }[] = [];
  for (const inc of incidents) {
    const day = inc.occurred_at.slice(0, 10);
    const last = groups[groups.length - 1];
    if (last && last.day === day) last.items.push(inc);
    else groups.push({ day, items: [inc] });
  }
  return groups;
}

const dayLabel = (day: string): string =>
  new Date(day).toLocaleDateString("en-GB", {
    weekday: "short",
    day: "numeric",
    month: "short",
    year: "numeric",
  });

export default function Feed({ incidents }: { incidents: Incident[] }) {
  if (incidents.length === 0) {
    return <p style={{ color: "var(--text-muted)" }}>No incidents match the filters.</p>;
  }
  return (
    <div>
      {groupByDay(incidents).map(({ day, items }) => (
        <section key={day} aria-label={dayLabel(day)}>
          <h3 className="feed-day">
            {dayLabel(day)}
            <span> · {items.length === 1 ? "1 incident" : `${items.length} incidents`}</span>
          </h3>
          {items.map((inc) => (
            <article
              className="feed-item"
              key={inc.id}
              // Stable anchor so a single incident can be linked to directly.
              id={`incident-${inc.id}`}
              data-state-media={inc.credibility === "state-controlled" || undefined}
            >
              {/* Title first: it is the thing a reader scans; the metadata
                  qualifies it and reads second. */}
              <div className="title">
                <a href={inc.url} target="_blank" rel="noopener noreferrer">
                  {inc.title}
                </a>
                <a
                  className="permalink"
                  href={`#incident-${inc.id}`}
                  title="Link to this incident"
                  aria-label="Link to this incident"
                >
                  #
                </a>
              </div>
              <div className="head">
                {/* Tone and severity share one badge: the arrow carries the
                    direction, the colour how consequential. */}
                <span
                  className="sev"
                  style={{
                    background: severityColor(inc.severity),
                    color: severityTextColor(inc.severity),
                  }}
                  title={`${toneDef(inc.tone).label} · severity ${inc.severity} — how consequential, independent of direction`}
                  aria-label={`${toneDef(inc.tone).label}, severity ${inc.severity}, ${SEVERITY_LABELS[inc.severity]}`}
                >
                  {toneDef(inc.tone).symbol} S{inc.severity} {SEVERITY_LABELS[inc.severity]}
                </span>
                <span
                  className="chip cat"
                  style={{ borderLeftColor: categoryColor(inc.category) }}
                >
                  {categoryLabel(inc.category)}
                </span>
                <span className="chip">{inc.countries.join(" · ")}</span>
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
                {inc.credibility === "state-controlled" && (
                  <span
                    className="cred-warn"
                    title={CREDIBILITY["state-controlled"].label}
                  >
                    ⚠ state media — adversary messaging, not verified reporting
                  </span>
                )}
              </div>
              {inc.summary && <p className="summary">{inc.summary}</p>}
            </article>
          ))}
        </section>
      ))}
    </div>
  );
}
