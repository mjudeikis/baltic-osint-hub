import { useEffect, useState } from "react";
import { SourceStatus } from "../api";
import { cssColor } from "../taxonomy";

// The collector runs hourly, so anything much past that means collection has
// stopped rather than simply being between runs.
const STALE_AFTER_MS = 2 * 60 * 60 * 1000;
const CRITICAL_AFTER_MS = 6 * 60 * 60 * 1000;

export default function StatusBanner({ sources }: { sources: SourceStatus[] }) {
  // Re-render on a timer so "14 min ago" stays honest on a dashboard that is
  // left open; the data itself is refetched by App.
  const [, tick] = useState(0);
  useEffect(() => {
    const id = setInterval(() => tick((n) => n + 1), 30_000);
    return () => clearInterval(id);
  }, []);

  if (sources.length === 0) {
    return (
      <div className="banner" role="status">
        <span style={{ color: "var(--text-muted)" }}>Loading collection status…</span>
      </div>
    );
  }

  const lastRun = sources.reduce<Date | null>((latest, s) => {
    const t = new Date(s.last_run);
    return !latest || t > latest ? t : latest;
  }, null);
  const failing = sources.filter((s) => s.error);
  const age = lastRun ? Date.now() - lastRun.getTime() : Infinity;

  const status =
    age > CRITICAL_AFTER_MS
      ? { label: "Collection stalled", symbol: "▲", cssVar: "--status-critical" }
      : age > STALE_AFTER_MS
        ? { label: "Data may be stale", symbol: "△", cssVar: "--status-warning" }
        : { label: "Live", symbol: "●", cssVar: "--status-good" };

  return (
    <div className="banner" role="status">
      <span className="level" style={{ color: cssColor(status.cssVar) }}>
        <span className="dot" style={{ background: cssColor(status.cssVar) }} />
        {status.symbol} {status.label}
      </span>
      <span>
        Last sync{" "}
        <strong title={lastRun?.toLocaleString("en-GB")}>{relative(age)}</strong>
      </span>
      <span className="banner-sep">·</span>
      <span>
        {sources.length - failing.length}/{sources.length} sources OK
        {failing.length > 0 && (
          <span title={failing.map((f) => f.source).join(", ")}>
            {" "}
            ({failing.length} failing)
          </span>
        )}
      </span>
      <span className="banner-sep">·</span>
      <span style={{ color: "var(--text-muted)" }}>collector runs hourly</span>
    </div>
  );
}

function relative(ms: number): string {
  if (!isFinite(ms)) return "unknown";
  const min = Math.floor(ms / 60000);
  if (min < 1) return "just now";
  if (min < 60) return `${min} min ago`;
  const hours = Math.floor(min / 60);
  if (hours < 24) return `${hours} h ago`;
  const days = Math.floor(hours / 24);
  return `${days} day${days === 1 ? "" : "s"} ago`;
}
