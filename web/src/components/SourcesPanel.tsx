import { SourceStatus } from "../api";

export default function SourcesPanel({ sources }: { sources: SourceStatus[] }) {
  const failing = sources.filter((s) => s.error);
  return (
    <div className="sources-panel">
      <p>
        Items are collected hourly from public RSS feeds (national broadcasters and
        dailies in English, Lithuanian, Latvian, Estonian and Polish; defence
        ministries; national CERTs and security services; EU agencies; research
        institutes), the GDELT news index, targeted Google News queries, public
        Telegram channels — including Russian and Belarusian state and pro-war
        channels, monitored as primary sources of adversary messaging rather than as
        trusted reporting — regional subreddits, and Bluesky keyword searches. Each
        item is then classified and summarised by a language model.
      </p>
      <p>
        Every item carries a <strong>tone</strong> as well as a severity:
        favourable, neutral or adverse for the security of the region. Defensive
        deployments, arrests and interdictions are favourable even though their
        subject is a threat — the dashboard is deliberately built not to read as
        uniformly dire.
      </p>
      <p>
        Map layers add machine measurements: GPS-jamming cells, NASA FIRMS thermal
        anomalies, aircraft and vessel activity, and Sentinel-1 radar change
        detection over monitored sites. These are <em>detections, not verified
        events</em>. Classification is automated and may contain errors — always
        verify against the linked original source. This dashboard aggregates{" "}
        <em>publicly reported</em> events; it is not an official assessment.
      </p>

      {failing.length > 0 && (
        <p style={{ color: "var(--text-muted)" }}>
          {failing.length} of {sources.length} sources are currently failing; those
          items simply retry on the next run.
        </p>
      )}

      <div style={{ overflowX: "auto" }}>
        <table>
          <thead>
            <tr>
              <th>Source</th>
              <th>Last fetch</th>
              <th>Items</th>
              <th>New</th>
              <th>Status</th>
            </tr>
          </thead>
          <tbody>
            {[...sources]
              .sort((a, b) => a.source.localeCompare(b.source))
              .map((s) => (
                <tr key={s.source}>
                  <td>{s.source}</td>
                  <td>{new Date(s.last_run).toLocaleString("en-GB")}</td>
                  <td>{s.items_found}</td>
                  <td>{s.items_new}</td>
                  <td
                    style={{ color: s.error ? "var(--status-critical)" : undefined }}
                    title={s.error || undefined}
                  >
                    {s.error ? "error" : "ok"}
                  </td>
                </tr>
              ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
