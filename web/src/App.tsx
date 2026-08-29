import { useEffect, useState } from "react";
import {
  fetchIncidents,
  fetchSources,
  fetchSummary,
  fetchTimeline,
  Incident,
  SourceStatus,
  SummaryCell,
  TimelineBucket,
} from "./api";
import { CATEGORIES, COUNTRIES, COUNTRY_NAMES, categoryLabel } from "./taxonomy";
import ThreatBoard from "./components/ThreatBoard";
import Timeline from "./components/Timeline";
import IncidentMap from "./components/IncidentMap";
import Feed from "./components/Feed";

const DAY_PRESETS = [7, 30, 90] as const;

export default function App() {
  const [days, setDays] = useState<number>(30);
  const [country, setCountry] = useState("");
  const [category, setCategory] = useState("");
  const [summary, setSummary] = useState<SummaryCell[]>([]);
  const [timeline, setTimeline] = useState<TimelineBucket[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [sources, setSources] = useState<SourceStatus[]>([]);
  const [error, setError] = useState("");

  useEffect(() => {
    fetchSummary().then(setSummary).catch((e) => setError(String(e)));
    fetchSources().then(setSources).catch(() => {});
  }, []);

  useEffect(() => {
    fetchTimeline(days, country || undefined)
      .then(setTimeline)
      .catch((e) => setError(String(e)));
  }, [days, country]);

  useEffect(() => {
    fetchIncidents({ days, country: country || undefined, category: category || undefined })
      .then(setIncidents)
      .catch((e) => setError(String(e)));
  }, [days, country, category]);

  return (
    <div className="container">
      <header className="site">
        <h1>Baltic OSINT Hub</h1>
        <p>
          Open-source tracking of hybrid-threat activity affecting Lithuania, Latvia,
          Estonia, and Poland — aggregated from public news, CERT, and research feeds.
        </p>
      </header>

      {error && (
        <div className="card" role="alert" style={{ color: "var(--status-critical)" }}>
          Failed to load data: {error}
        </div>
      )}

      <section className="card" aria-label="Threat board">
        <h2>Last 7 days by country</h2>
        <ThreatBoard cells={summary} />
      </section>

      <div className="filters" role="group" aria-label="Filters">
        {DAY_PRESETS.map((d) => (
          <button key={d} aria-pressed={days === d} onClick={() => setDays(d)}>
            {d} days
          </button>
        ))}
        <select value={country} onChange={(e) => setCountry(e.target.value)} aria-label="Country">
          <option value="">All countries</option>
          {COUNTRIES.map((c) => (
            <option key={c} value={c}>
              {COUNTRY_NAMES[c]}
            </option>
          ))}
        </select>
        <select value={category} onChange={(e) => setCategory(e.target.value)} aria-label="Category">
          <option value="">All categories</option>
          {CATEGORIES.map((c) => (
            <option key={c.key} value={c.key}>
              {c.label}
            </option>
          ))}
        </select>
      </div>

      <section className="card" aria-label="Trend">
        <h2>Incidents per day{country ? ` — ${COUNTRY_NAMES[country as never]}` : ""}</h2>
        <Timeline buckets={timeline} />
      </section>

      <section className="card" aria-label="Map">
        <h2>Located incidents</h2>
        <IncidentMap incidents={incidents} />
      </section>

      <section className="card" aria-label="Incident feed">
        <h2>
          Incident feed
          {category ? ` — ${categoryLabel(category)}` : ""} ({incidents.length})
        </h2>
        <Feed incidents={incidents} />
      </section>

      <footer className="site">
        <div className="card">
          <h2>Sources &amp; methodology</h2>
          <p>
            Items are collected every 30 minutes from public RSS feeds (LRT, ERR, LSM,
            Notes from Poland, EUvsDisinfo, CERT.PL, CEPA, Jamestown, ICDS, Warsaw
            Institute, Lithuanian MoD) and the GDELT news index, then automatically
            classified and summarized by a language model. Classification is automated
            and may contain errors — always verify against the linked original source.
            This dashboard aggregates <em>publicly reported</em> events; it is not an
            official assessment.
          </p>
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
              {sources.map((s) => (
                <tr key={s.source}>
                  <td>{s.source}</td>
                  <td>{new Date(s.last_run).toLocaleString("en-GB")}</td>
                  <td>{s.items_found}</td>
                  <td>{s.items_new}</td>
                  <td style={{ color: s.error ? "var(--status-critical)" : undefined }}>
                    {s.error ? "error" : "ok"}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </footer>
    </div>
  );
}
