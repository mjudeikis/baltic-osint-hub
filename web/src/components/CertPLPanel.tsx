import { useMemo } from "react";
import { CertPLDay } from "../api";
import { cssColor } from "../taxonomy";

// CERT.PL warning-list churn, as a rate: how many malicious domains Poland's
// national CERT added this week against a typical week. It is a machine
// signal for the cyber category and a Poland-only one — no equivalent open
// feed exists for Lithuania, Latvia or Estonia, and the panel says so rather
// than letting a Polish number stand in for the region. Counts only: the
// domains themselves are never surfaced here.

const WEEKS = 12;
const DAY = 24 * 60 * 60 * 1000;

export interface CertPLWeek {
  // ISO date of the week's first day.
  start: string;
  added: number;
}

// Sum daily additions into trailing 7-day weeks, most recent last. Pure so it
// can be tested; `now` is injected for the same reason.
export function weeklyAdded(days: CertPLDay[], now: Date = new Date(), weeks = WEEKS): CertPLWeek[] {
  const end = new Date(now);
  end.setUTCHours(0, 0, 0, 0);
  const out: CertPLWeek[] = [];
  for (let w = weeks - 1; w >= 0; w--) {
    const stop = end.getTime() - w * 7 * DAY + DAY; // exclusive
    const start = stop - 7 * DAY;
    const added = days.reduce((n, d) => {
      const t = new Date(d.day).getTime();
      return t >= start && t < stop ? n + d.added : n;
    }, 0);
    out.push({ start: new Date(start).toISOString().slice(0, 10), added });
  }
  return out;
}

export function median(xs: number[]): number {
  if (xs.length === 0) return 0;
  const s = [...xs].sort((a, b) => a - b);
  const mid = Math.floor(s.length / 2);
  return s.length % 2 ? s[mid]! : (s[mid - 1]! + s[mid]!) / 2;
}

export default function CertPLPanel({
  days,
  error,
}: {
  // null while loading; [] when the feed has never been collected.
  days: CertPLDay[] | null;
  error?: string;
}) {
  const weeks = useMemo(() => (days ? weeklyAdded(days) : []), [days]);

  if (error) {
    return (
      <p className="certpl" style={{ color: "var(--text-muted)" }}>
        <strong>CERT.PL warning list:</strong> unavailable right now — the feed did not
        load, so no rate can be shown. (Poland only.)
      </p>
    );
  }
  if (days === null) {
    return (
      <p className="certpl" style={{ color: "var(--text-muted)" }} aria-busy="true">
        Loading the CERT.PL warning-list rate…
      </p>
    );
  }
  if (days.length === 0) {
    return (
      <p className="certpl" style={{ color: "var(--text-muted)" }}>
        <strong>CERT.PL warning list:</strong> no data collected yet — not a quiet week,
        an empty feed. (Poland only.)
      </p>
    );
  }

  const thisWeek = weeks[weeks.length - 1]!;
  const prior = weeks.slice(0, -1).map((w) => w.added);
  const typical = Math.round(median(prior));
  const max = Math.max(1, ...weeks.map((w) => w.added));
  const fill = cssColor("--series-1");

  return (
    <div className="certpl">
      <p>
        <strong>CERT.PL warning list:</strong> {thisWeek.added} malicious domains added
        this week
        {prior.some((n) => n > 0)
          ? ` · a typical week adds ${typical}`
          : " · not enough history yet to say what is typical"}
        .{" "}
        <span style={{ color: "var(--text-muted)" }}>
          Poland only — no equivalent open feed exists for Lithuania, Latvia or
          Estonia. Counts only; domains are never shown.
        </span>
      </p>
      {/* Twelve trailing weeks as a bar strip: one series, one hue, thin
          marks with a surface gap, the current week marked. Weekly totals in
          the table below carry the same numbers for assistive tech. */}
      <svg
        className="certpl-strip"
        viewBox={`0 0 ${WEEKS * 14} 28`}
        width={WEEKS * 14}
        height={28}
        role="img"
        aria-label={`Domains added per week, last ${WEEKS} weeks: ${weeks
          .map((w) => w.added)
          .join(", ")}`}
      >
        {weeks.map((w, i) => {
          const h = Math.max(1, Math.round((w.added / max) * 26));
          return (
            <rect
              key={w.start}
              x={i * 14 + 2}
              y={28 - h}
              width={10}
              height={h}
              rx={i === weeks.length - 1 ? 2 : 1}
              fill={fill}
              opacity={i === weeks.length - 1 ? 1 : 0.45}
            >
              <title>
                week of {w.start}: {w.added} added
              </title>
            </rect>
          );
        })}
      </svg>
      <table className="sr-only">
        <caption>CERT.PL domains added per week</caption>
        <tbody>
          {weeks.map((w) => (
            <tr key={w.start}>
              <th scope="row">week of {w.start}</th>
              <td>{w.added}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
