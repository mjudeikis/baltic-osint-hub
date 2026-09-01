import { Incident, SummaryCell } from "../api";
import {
  COUNTRIES,
  COUNTRY_NAMES,
  categoryLabel,
  cssColor,
  severityColor,
  severityTextColor,
  textColor,
  TONES,
} from "../taxonomy";

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
// Status glyphs are the ◆/◇/○ family; ▲/▼ belong exclusively to tone
// direction, so one glyph never means "favourable" and "alarm" on one page.
function level(
  adverse: number,
  maxSev: number,
  favourable: number,
  pendingSevere = false,
): Level {
  if (maxSev >= 5) return { label: "Critical", cssVar: "--status-critical", symbol: "◆" };
  if (maxSev >= 4) return { label: "Serious", cssVar: "--status-serious", symbol: "◆" };
  // A serious report still awaiting a second source holds here rather than
  // being softened away, exactly as the posture ladder holds it at Elevated.
  // Without this the banner would read Elevated while every country tile read
  // Watchful, and the page would contradict itself.
  if (pendingSevere)
    return { label: "Unconfirmed", cssVar: "--status-warning", symbol: "◇" };
  if (adverse >= 3 && favourable <= adverse)
    return { label: "Elevated", cssVar: "--status-warning", symbol: "◇" };
  if (adverse > 0) return { label: "Watchful", cssVar: "--status-good", symbol: "○" };
  return { label: "Quiet", cssVar: "--status-good", symbol: "○" };
}

// How many adverse headlines a tile shows. Enough to name what is behind the
// count without turning the board into a second feed.
const TOP_EVENTS = 3;

// The worst events behind a tile's count, one headline per clustered event —
// an event carried by five outlets is one thing that happened, not five.
function topEvents(incidents: Incident[], cc: string): Incident[] {
  const seen = new Set<number>();
  return incidents
    .filter((i) => i.countries.includes(cc))
    .sort(
      (a, b) =>
        b.severity - a.severity || b.occurred_at.localeCompare(a.occurred_at),
    )
    .filter((i) => {
      if (i.event_id == null) return true;
      if (seen.has(i.event_id)) return false;
      seen.add(i.event_id);
      return true;
    })
    .slice(0, TOP_EVENTS);
}

export default function ThreatBoard({
  cells,
  incidents,
  onSelect,
}: {
  // null while the first response is in flight. Rendering an empty array
  // there would paint four "Quiet / 0 adverse" tiles — a fabricated all-clear
  // on every slow connection.
  cells: SummaryCell[] | null;
  // Adverse incidents over the same 7-day window the tiles count, so the
  // headlines shown are the events behind the number above them.
  incidents: Incident[];
  // Drill-through: a country (and optionally a category) filters the incident
  // feed, so the numbers on this board lead to the items behind them.
  onSelect: (country: string, category: string) => void;
}) {
  if (cells === null) {
    return (
      <p style={{ color: "var(--text-muted)" }} aria-busy="true">
        Loading the last 7 days…
      </p>
    );
  }
  return (
    <>
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
        // Corroboration gates the top bands only, exactly as the posture
        // ladder does — it must not erase the label altogether.
        //
        // One uncorroborated report, classified as affecting all four
        // countries, was marking the whole region "Serious" while the banner
        // above correctly said the event was still awaiting corroboration.
        // An uncorroborated severity 4 or 5 is therefore capped at the tier
        // below; everything at severity 3 or under is unaffected.
        const rawSev = Math.max(0, ...rows.map((r) => r.max_severity ?? 0));
        const corrSev = Math.max(0, ...rows.map((r) => r.max_severity_corroborated ?? 0));
        const maxSev = Math.max(corrSev, Math.min(rawSev, 3));
        const lv = level(adverse, maxSev, favourable, rawSev >= 4 && corrSev < 4);

        // Only compare against the baseline once there is enough of one.
        const comparable = samples >= MIN_BASELINE_SAMPLES && baseline > 0;
        const deltaPct = comparable
          ? Math.round(((adverse - baseline) / baseline) * 100)
          : null;

        const top = rows
          .filter((r) => r.recent_adverse > 0)
          .sort((a, b) => b.recent_adverse - a.recent_adverse)
          .slice(0, 3);

        const events = topEvents(incidents, cc);

        return (
          <div className="tile" role="listitem" key={cc}>
            <div className="country">
              <span>{COUNTRY_NAMES[cc]}</span>
              <span className="level" style={{ color: textColor(lv.cssVar) }}>
                <span className="dot" style={{ background: cssColor(lv.cssVar) }} />
                {lv.symbol} {lv.label}
              </span>
            </div>

            {/* Both directions as one side-by-side pair, each number stated
                exactly once. Adverse is the board's subject and stays the
                click-through to the country's feed. */}
            <div className="tile-pair">
              <button
                className="tile-main"
                onClick={() => onSelect(cc, "")}
                title={`Show all adverse incidents in ${COUNTRY_NAMES[cc]}`}
              >
                <div className="count" style={{ color: textColor(TONES.negative.cssVar) }}>
                  {TONES.negative.symbol} {adverse}
                </div>
                <div className="delta">adverse</div>
              </button>
              <div>
                <div className="count" style={{ color: textColor(TONES.positive.cssVar) }}>
                  {TONES.positive.symbol} {favourable}
                </div>
                <div className="delta">favourable</div>
              </div>
            </div>
            <div className="delta">
              in 7 days
              {deltaPct !== null
                ? ` · ${deltaPct >= 0 ? "+" : ""}${deltaPct}% adverse vs prior 4-week avg`
                : " · baseline still building"}
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

            {/* The events themselves, worst first — so the reader sees what
                the count is made of without leaving the board. Headlines link
                to the source article; the count above drills to the feed. */}
            {events.length > 0 && (
              <ul className="tile-events" aria-label={`Top adverse events in ${COUNTRY_NAMES[cc]}`}>
                {events.map((e) => (
                  <li key={e.id}>
                    <span
                      className="tile-event-sev"
                      style={{
                        background: severityColor(e.severity),
                        color: severityTextColor(e.severity),
                      }}
                      title={`Severity ${e.severity}`}
                    >
                      S{e.severity}
                    </span>
                    {/* The enriched summary, not the headline: titles arrive in
                        the source language, and the summary is always English. */}
                    <a
                      href={e.url}
                      target="_blank"
                      rel="noopener noreferrer"
                      title={e.summary || e.title}
                    >
                      {e.summary || e.title}
                    </a>
                  </li>
                ))}
              </ul>
            )}
          </div>
        );
      })}
    </div>

    {/* The tile labels are a country's own 7-day reading, not the regional
        posture above — and "Unconfirmed" in particular must be defined where
        it appears, because under a tense news cycle an unexplained
        "unconfirmed" reads as "they know something they cannot say". */}
    <details className="posture-rules">
      <summary>What these labels mean</summary>
      <p>
        Each tile is that country's own last 7 days, set by the severity of
        corroborated adverse events — not by how many articles were written.
        It is a narrower reading than the regional posture banner above, so
        the two use different words. Counts cover events of severity 2 and
        above; severity-1 analysis and commentary stay in the feed but are
        not counted as events.
      </p>
      <ul>
        <li>
          <strong>Quiet / Watchful</strong> — no adverse events, or only minor
          ones.
        </li>
        <li>
          <strong>Elevated</strong> — several adverse events and little
          favourable news to offset them.
        </li>
        <li>
          <strong>Unconfirmed</strong> — a serious report exists but only one
          independent source carries it so far. It is held here rather than
          escalated: not dismissed, not yet confirmed.
        </li>
        <li>
          <strong>Serious / Critical</strong> — at least one corroborated
          severity&nbsp;4 or 5 event this week.
        </li>
      </ul>
    </details>
    </>
  );
}
