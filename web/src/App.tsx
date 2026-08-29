import { useEffect, useState } from "react";
import {
  fetchIncidents,
  fetchLayers,
  fetchMeta,
  fetchPosture,
  Meta,
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
import {
  CATEGORIES,
  COUNTRIES,
  COUNTRY_NAMES,
  categoryLabel,
} from "./taxonomy";
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
import Preparedness from "./components/Preparedness";

import {
  DAY_PRESETS,
  exportURL,
  readFilters,
  syncURL,
} from "./urlState";

const NAV_ITEMS: NavItem[] = [
  { id: "board", label: "By country" },
  { id: "trend", label: "Trend" },
  { id: "map", label: "Situation map" },
  { id: "satellite", label: "Satellite" },
  { id: "feed", label: "Incident feed" },
  { id: "prepare", label: "How to prepare" },
  { id: "sources", label: "Sources" },
];

export default function App() {
  // Seeded from the URL so a shared link opens on the same view it was copied
  // from, rather than resetting the reader to the default dashboard.
  const initial = readFilters();
  const [days, setDays] = useState<number>(initial.days);
  const [country, setCountry] = useState(initial.country);
  const [category, setCategory] = useState(initial.category);
  const [tone, setTone] = useState(initial.tone);
  const [summary, setSummary] = useState<SummaryCell[]>([]);
  const [timeline, setTimeline] = useState<TimelineBucket[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [sources, setSources] = useState<SourceStatus[]>([]);
  const [layers, setLayers] = useState<Layers | null>(null);
  const [posture, setPosture] = useState<Posture | null>(null);
  const [meta, setMeta] = useState<Meta | null>(null);
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

  const filters = { days, country, category, tone };
  useEffect(() => {
    syncURL({ days, country, category, tone });
  }, [days, country, category, tone]);

  useEffect(() => {
    fetchSummary()
      .then(setSummary)
      .catch((e) => setError(String(e)));
    fetchSources()
      .then(setSources)
      .catch(() => {});
    fetchLayers()
      .then(setLayers)
      .catch(() => {});
  }, [refreshKey]);

  // The taxonomy and the posture rules are static per deploy; fetched once.
  useEffect(() => {
    fetchMeta()
      .then(setMeta)
      .catch(() => {});
  }, []);

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
    fetchIncidents({
      days,
      country: country || undefined,
      category: category || undefined,
      tone: tone || undefined,
    })
      .then(setIncidents)
      .catch((e) => setError(String(e)));
  }, [days, country, category, tone, refreshKey]);

  return (
    <div className="container">
      <StatusBanner sources={sources} />

      <header className="site">
        <h1>Baltic OSINT Hub</h1>
        <p>
          Open-source tracking of hybrid-threat activity affecting Lithuania,
          Latvia, Estonia, and Poland — aggregated from public news, CERT, and
          research feeds.
        </p>
      </header>

      {error && (
        <div
          className="card"
          role="alert"
          style={{ color: "var(--status-critical)" }}
        >
          Failed to load data: {error}
        </div>
      )}

      <div className="layout">
        <SideNav items={NAV_ITEMS} />

        <main>
          <PostureBanner
            posture={posture}
            scope={country ? COUNTRY_NAMES[country as never] : ""}
            meta={meta}
          />

          <Section id="board" title="Last 7 days by country">
            <ThreatBoard
              cells={summary}
              onSelect={(cc, cat) => {
                setCountry(cc);
                setCategory(cat);
                // The tile counts adverse items, so the feed must match or the
                // number the reader clicked would not be the number they get.
                setTone("negative");
                // Seven days, because the board's numbers are a 7-day count.
                setDays(7);
                document
                  .getElementById("feed")
                  ?.scrollIntoView({ block: "start" });
              }}
            />
          </Section>

          <div className="filters" role="group" aria-label="Filters">
            {DAY_PRESETS.map((d) => (
              <button
                key={d}
                aria-pressed={days === d}
                onClick={() => setDays(d)}
              >
                {d} days
              </button>
            ))}
            <select
              value={country}
              onChange={(e) => setCountry(e.target.value)}
              aria-label="Country"
            >
              <option value="">All countries</option>
              {COUNTRIES.map((c) => (
                <option key={c} value={c}>
                  {COUNTRY_NAMES[c]}
                </option>
              ))}
            </select>
            <select
              value={tone}
              onChange={(e) => setTone(e.target.value)}
              aria-label="Direction"
            >
              <option value="">All directions</option>
              <option value="negative">▼ Adverse only</option>
              <option value="positive">▲ Favourable only</option>
              <option value="neutral">● Neutral only</option>
            </select>
            <select
              value={category}
              onChange={(e) => setCategory(e.target.value)}
              aria-label="Category"
            >
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

          <Section
            id="satellite"
            title="Satellite change detection — monitored sites"
          >
            <SarPanel
              aois={layers?.sar ?? []}
              focused={focusedSite}
              onFocus={(key) => {
                setFocusedSite(key);
                document
                  .getElementById("map")
                  ?.scrollIntoView({ block: "start" });
              }}
            />
          </Section>

          <Section
            id="feed"
            title="Incident feed"
            aside={`${incidents.length} shown${category ? ` · ${categoryLabel(category)}` : ""}`}
          >
            {/* Both links carry the filters currently on screen, so what a
                reader downloads is what they were looking at. */}
            <p className="export-links">
              Download this view:{" "}
              <a href={exportURL("csv", filters)}>CSV</a>
              {" · "}
              <a href={exportURL("geojson", filters)}>GeoJSON</a>
              <span style={{ color: "var(--text-muted)" }}>
                {" "}
                — GeoJSON covers located incidents only.
              </span>
            </p>
            <Feed incidents={incidents} />
          </Section>

          <Section id="prepare" title="How to prepare">
            <Preparedness posture={posture} />
          </Section>

          <Section
            id="sources"
            title="Sources &amp; methodology"
            defaultOpen={false}
          >
            <SourcesPanel sources={sources} />
          </Section>
        </main>
      </div>
    </div>
  );
}
