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
      <a
        className="banner-repo"
        href="https://github.com/mjudeikis/baltic-osint-hub"
        target="_blank"
        rel="noopener noreferrer"
        title="Source code, methodology and the full source list"
      >
        <svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true" fill="currentColor">
          <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.4 7.4 0 0 1 2-.27c.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z" />
        </svg>
        source
      </a>
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
