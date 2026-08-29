import { Posture } from "../api";
import { cssColor, postureColor, TONES } from "../taxonomy";

// The single at-a-glance reading. Its job is as much to prevent false alarm as
// to raise it: the balance bar shows how much of the week was actually
// favourable, which a threat-only feed hides.
export default function PostureBanner({
  posture,
  scope,
}: {
  posture: Posture | null;
  scope: string;
}) {
  if (!posture) {
    return (
      <div className="posture" aria-busy="true">
        <span style={{ color: "var(--text-muted)" }}>Reading regional posture…</span>
      </div>
    );
  }

  const colour = postureColor(posture.level);
  const { positive, neutral, negative } = posture.counts;
  const total = positive + neutral + negative;

  return (
    <section className="posture" aria-label="Regional posture">
      <div className="posture-head">
        <div>
          <div className="posture-eyebrow">
            Regional posture{scope ? ` — ${scope}` : ""} · last 7 days
          </div>
          <div className="posture-level" style={{ color: colour }}>
            {posture.level_name}
            <span className="posture-of">{posture.level} of 5</span>
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
      <p className="posture-explain">{posture.explanation}</p>
      {posture.context && <p className="posture-context">{posture.context}</p>}

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
                <span key={k} className="key" style={{ color: cssColor(TONES[k].cssVar) }}>
                  {TONES[k].symbol} {n} {TONES[k].label.toLowerCase()}
                </span>
              );
            })}
          </div>
        </>
      )}
    </section>
  );
}

function balanceLabel(positive: number, neutral: number, negative: number): string {
  return `${positive} favourable, ${neutral} neutral, ${negative} adverse`;
}
