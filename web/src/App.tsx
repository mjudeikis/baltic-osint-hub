import { useEffect, useState } from "react";
import {
  fetchIncidents,
  fetchLayers,
  fetchPosture,
  Posture,
  fetchSources,
  fetchSummary,
  fetchTimeline,
  Incident,
  Layers,
  SourceStatus,
  SummaryCell,
  TimelineBucket,
} from "./api";
import { CATEGORIES, COUNTRIES, COUNTRY_NAMES, categoryLabel } from "./taxonomy";
import ThreatBoard from "./components/ThreatBoard";
import Timeline from "./components/Timeline";
import IncidentMap from "./components/IncidentMap";
import Feed from "./components/Feed";
import SarPanel from "./components/SarPanel";
import StatusBanner from "./components/StatusBanner";
import PostureBanner from "./components/PostureBanner";
import Section from "./components/Section";
import SideNav, { NavItem } from "./components/SideNav";
import SourcesPanel from "./components/SourcesPanel";

const DAY_PRESETS = [7, 30, 90] as const;

const NAV_ITEMS: NavItem[] = [
  { id: "board", label: "By country" },
  { id: "trend", label: "Trend" },
  { id: "map", label: "Situation map" },
  { id: "satellite", label: "Satellite" },
  { id: "feed", label: "Incident feed" },
  { id: "sources", label: "Sources" },
];

export default function App() {
  const [days, setDays] = useState<number>(30);
  const [country, setCountry] = useState("");
  const [category, setCategory] = useState("");
  const [summary, setSummary] = useState<SummaryCell[]>([]);
  const [timeline, setTimeline] = useState<TimelineBucket[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [sources, setSources] = useState<SourceStatus[]>([]);
  const [layers, setLayers] = useState<Layers | null>(null);
  const [posture, setPosture] = useState<Posture | null>(null);
  const [focusedSite, setFocusedSite] = useState<string | null>(null);
  const [error, setError] = useState("");

  // Periodic refresh so a dashboard left open doesn't drift — and so the
  // status banner's "last sync" can't claim freshness the rest of the page
  // doesn't have. Matches the API's 5-minute Cache-Control.
  const [refreshKey, setRefreshKey] = useState(0);
  useEffect(() => {
    const id = setInterval(() => setRefreshKey((k) => k + 1), 5 * 60 * 1000);
    return () => clearInterval(id);
  }, []);

  useEffect(() => {
    fetchSummary().then(setSummary).catch((e) => setError(String(e)));
    fetchSources().then(setSources).catch(() => {});
    fetchLayers().then(setLayers).catch(() => {});
  }, [refreshKey]);

  useEffect(() => {
    fetchTimeline(days, country || undefined)
      .then(setTimeline)
      .catch((e) => setError(String(e)));
    // Posture follows the country filter so it reads for whatever is on screen.
    fetchPosture(country || undefined)
      .then(setPosture)
      .catch(() => {});
  }, [days, country, refreshKey]);

  useEffect(() => {
    fetchIncidents({ days, country: country || undefined, category: category || undefined })
      .then(setIncidents)
      .catch((e) => setError(String(e)));
  }, [days, country, category, refreshKey]);

  return (
    <div className="container">
      <StatusBanner sources={sources} />

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

      <div className="layout">
        <SideNav items={NAV_ITEMS} />

        <main>
          <PostureBanner
            posture={posture}
            scope={country ? COUNTRY_NAMES[country as never] : ""}
          />

          <Section id="board" title="Last 7 days by country">
            <ThreatBoard cells={summary} />
          </Section>

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

          <Section
            id="trend"
            title={`Incidents per day${country ? ` — ${COUNTRY_NAMES[country as never]}` : ""}`}
          >
            <Timeline buckets={timeline} />
          </Section>

          <Section id="map" title="Situation map">
            <IncidentMap
              incidents={incidents}
              layers={layers}
              focusedSite={focusedSite}
              onFocusHandled={() => setFocusedSite(null)}
            />
          </Section>

          <Section id="satellite" title="Satellite change detection — monitored sites">
            <SarPanel
              aois={layers?.sar ?? []}
              focused={focusedSite}
              onFocus={(key) => {
                setFocusedSite(key);
                document.getElementById("map")?.scrollIntoView({ block: "start" });
              }}
            />
          </Section>

          <Section
            id="feed"
            title="Incident feed"
            aside={`${incidents.length} shown${category ? ` · ${categoryLabel(category)}` : ""}`}
          >
            <Feed incidents={incidents} />
          </Section>

          <Section id="sources" title="Sources &amp; methodology" defaultOpen={false}>
            <SourcesPanel sources={sources} />
          </Section>
        </main>
      </div>

    </div>
  );
}
