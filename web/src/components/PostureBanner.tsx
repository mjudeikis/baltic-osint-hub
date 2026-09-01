import { Meta, Posture } from "../api";
import { cssColor, postureColor, postureTextColor, textColor, TONES } from "../taxonomy";
import { revealSection } from "./Section";

// The single at-a-glance reading. Its job is as much to prevent false alarm as
// to raise it: the balance bar shows how much of the week was actually
// favourable, which a threat-only feed hides.
export default function PostureBanner({
  posture,
  scope,
  meta,
}: {
  posture: Posture | null;
  scope: string;
  meta: Meta | null;
}) {
  if (!posture) {
    return (
      <div className="posture" id="posture" tabIndex={-1} aria-busy="true">
        <span style={{ color: "var(--text-muted)" }}>Reading regional posture…</span>
      </div>
    );
  }

  const colour = postureColor(posture.level);
  const textColour = postureTextColor(posture.level);
  const { positive, neutral, negative } = posture.counts;
  const total = positive + neutral + negative;

  return (
    // tabIndex -1: the skip link must land keyboard focus here, and a plain
    // section is not otherwise focusable.
    <section className="posture" id="posture" tabIndex={-1} aria-label="Regional posture">
      <div className="posture-head">
        <div>
          <div className="posture-eyebrow">
            Regional posture{scope ? ` — ${scope}` : ""} · last 7 days
          </div>
          <div className="posture-level" style={{ color: textColour }}>
            {posture.level_name}
            <span className="posture-of">{posture.level} of 5</span>
            {/* A scale that can only ratchet up stops being informative — the
                US advisory system settled permanently on "guarded" and never
                once used its two lowest levels. Naming improvement as its own
                state is what keeps the ladder honest in both directions. */}
            {trendLabel(posture.trend) && (
              <span className="posture-trend" data-trend={posture.trend}>
                {trendLabel(posture.trend)}
              </span>
            )}
          </div>
        </div>

        {/* Ascending ladder: filled segments up to the current level. */}
        <div
          className="ladder"
          role="img"
          aria-label={`Level ${posture.level} of 5: ${posture.level_name}`}
        >
          {[1, 2, 3, 4, 5].map((n) => (
            <span
              key={n}
              className="rung"
              style={{
                background: n <= posture.level ? colour : "var(--grid)",
                opacity: n <= posture.level ? 1 : 0.6,
              }}
            />
          ))}
        </div>
      </div>

      <p className="posture-headline">{posture.headline}</p>
      {/* The honesty budget: one word (the level), one basis (the headline
          names the rule that set it), one caveat line. Everything else lives
          one click down in "How this level is decided". The line merges the
          count breakdown with the typicality answer — as data when history
          exists, as an honest "can't say yet" when it doesn't. */}
      <p className="posture-explain">
        {negative} adverse · {positive} favourable · {neutral} neutral this
        week —{" "}
        {posture.typical_week > 0
          ? `a typical week has ${posture.typical_week} adverse.`
          : "not enough history yet to say whether that is typical."}
      </p>

      {total > 0 && (
        <>
          <div className="balance" role="img" aria-label={balanceLabel(positive, neutral, negative)}>
            {(["positive", "neutral", "negative"] as const).map((k) => {
              const n = { positive, neutral, negative }[k];
              if (n === 0) return null;
              return (
                <span
                  key={k}
                  className="balance-seg"
                  style={{
                    width: `${(n / total) * 100}%`,
                    background: cssColor(TONES[k].cssVar),
                  }}
                />
              );
            })}
          </div>
          <div className="balance-key">
            {(["positive", "neutral", "negative"] as const).map((k) => {
              const n = { positive, neutral, negative }[k];
              return (
                <span key={k} className="key">
                  <span style={{ color: textColor(TONES[k].cssVar) }}>
                    {TONES[k].symbol}
                  </span>{" "}
                  {n} {TONES[k].label.toLowerCase()}
                </span>
              );
            })}
            {/* The reading's other half: what to do with it. Readiness is
                routine advice here, so the path to it is always present, not
                only once the level is already high. */}
            <button
              className="linklike"
              onClick={() => revealSection("prepare")}
            >
              How to prepare →
            </button>
          </div>
        </>
      )}

      {/* The exact rules behind the number above. A reading a reader cannot
          check is one they have to take on trust, and this dashboard asks for
          rather less trust than that. */}
      {meta?.posture_rules?.length ? (
        <details className="posture-rules">
          <summary>How this level is decided</summary>
          <p>
            The first matching rule sets the level. Counts cover the last 7
            days.
          </p>
          <ol>
            {meta.posture_rules.map((r, i) => (
              <li key={i}>
                <strong>{r.level_name}</strong> ({r.level} of 5) — {r.condition}
              </li>
            ))}
          </ol>
          <p>Then:</p>
          <ul>
            {meta.posture_adjustments.map((a, i) => (
              <li key={i}>{a}</li>
            ))}
          </ul>
        </details>
      ) : null}
    </section>
  );
}

function balanceLabel(positive: number, neutral: number, negative: number): string {
  return `${positive} favourable, ${neutral} neutral, ${negative} adverse`;
}

// "steady" is deliberately not shown: a badge on every ordinary week is noise,
// and the absence of a badge already reads as unremarkable.
function trendLabel(trend: string): string {
  switch (trend) {
    case "de-escalating":
      return "↓ easing";
    case "escalating":
      return "↑ rising";
    default:
      return "";
  }
}
