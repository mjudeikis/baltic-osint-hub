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
  countryName,
  toneDef,
} from "./taxonomy";
import ThreatBoard from "./components/ThreatBoard";
import Timeline from "./components/Timeline";
import IncidentMap from "./components/IncidentMap";
import Feed from "./components/Feed";
import SarPanel from "./components/SarPanel";
import StatusBanner from "./components/StatusBanner";
import PostureBanner from "./components/PostureBanner";
import Section, { revealSection } from "./components/Section";
import SideNav, { NavItem } from "./components/SideNav";
import SourcesPanel from "./components/SourcesPanel";
import Preparedness from "./components/Preparedness";

import {
  DAY_PRESETS,
  DEFAULT_FILTERS,
  FilterState,
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
  // One filter object rather than five parallel states: it is set from four
  // places (the strip, board drill-through, timeline clicks, the URL), and a
  // single value is what makes "clear all" and the chips row trivially honest.
  // Seeded from the URL so a shared link opens on the same view it was copied
  // from, rather than resetting the reader to the default dashboard.
  const [filters, setFilters] = useState<FilterState>(readFilters);
  const { days, country, category, tone, day } = filters;
  const patch = (p: Partial<FilterState>) => setFilters((f) => ({ ...f, ...p }));

  const [summary, setSummary] = useState<SummaryCell[]>([]);
  // The board's own adverse list, fixed to the 7-day window the tiles count.
  // It cannot share `incidents`: that list follows the feed filters, and a
  // reader narrowing the feed must not empty the headlines on the board.
  const [boardIncidents, setBoardIncidents] = useState<Incident[]>([]);
  const [timeline, setTimeline] = useState<TimelineBucket[]>([]);
  const [incidents, setIncidents] = useState<Incident[]>([]);
  const [sources, setSources] = useState<SourceStatus[]>([]);
  const [layers, setLayers] = useState<Layers | null>(null);
  const [posture, setPosture] = useState<Posture | null>(null);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [focusedSite, setFocusedSite] = useState<string | null>(null);

  // Errors are tracked per endpoint so a later success clears exactly the
  // failure it retried — a single transient blip during a background refresh
  // must not leave a permanent red banner over a page that has recovered.
  const [errors, setErrors] = useState<Record<string, string>>({});
  const noteError = (key: string, message: string) =>
    setErrors((e) => (e[key] === message ? e : { ...e, [key]: message }));
  const error = Object.values(errors).find(Boolean) ?? "";

  // Periodic refresh so a dashboard left open doesn't drift — and so the
  // status banner's "last sync" can't claim freshness the rest of the page
  // doesn't have. Matches the API's 5-minute Cache-Control.
  const [refreshKey, setRefreshKey] = useState(0);
  useEffect(() => {
    const id = setInterval(() => setRefreshKey((k) => k + 1), 5 * 60 * 1000);
    return () => clearInterval(id);
  }, []);

  useEffect(() => {
    syncURL(filters);
  }, [filters]);

  useEffect(() => {
    fetchSummary()
      .then((d) => {
        setSummary(d);
        noteError("summary", "");
      })
      .catch((e) => noteError("summary", String(e)));
    fetchIncidents({ days: 7, tone: "negative", limit: 200 })
      .then(setBoardIncidents)
      .catch(() => {});
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
      .then((d) => {
        setTimeline(d);
        noteError("timeline", "");
      })
      .catch((e) => noteError("timeline", String(e)));
    // Posture follows the country filter so it reads for whatever is on screen.
    fetchPosture(country || undefined)
      .then(setPosture)
      .catch(() => {});
  }, [days, country, refreshKey]);

  useEffect(() => {
    fetchIncidents({
      days,
      day: day || undefined,
      country: country || undefined,
      category: category || undefined,
      tone: tone || undefined,
    })
      .then((d) => {
        setIncidents(d);
        noteError("incidents", "");
      })
      .catch((e) => noteError("incidents", String(e)));
  }, [days, country, category, tone, day, refreshKey]);

  // Active filters, rendered as removable chips above the feed. Filters are
  // set from several places — some silently, like the board drill-through
  // setting tone — so their current state has to be visible where the results
  // are read, with one gesture to undo any of it.
  const chips: { label: string; clear: () => void }[] = [];
  if (country)
    chips.push({ label: countryName(country), clear: () => patch({ country: "" }) });
  if (category)
    chips.push({ label: categoryLabel(category), clear: () => patch({ category: "" }) });
  if (tone)
    chips.push({
      label: `${toneDef(tone).symbol} ${toneDef(tone).label} only`,
      clear: () => patch({ tone: "" }),
    });
  if (day) chips.push({ label: day, clear: () => patch({ day: "" }) });

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
            scope={country ? countryName(country) : ""}
            meta={meta}
          />

          <Section id="board" title="Last 7 days by country">
            <ThreatBoard
              cells={summary}
              incidents={boardIncidents}
              onSelect={(cc, cat) => {
                // The tile counts adverse items over 7 days, so the feed must
                // match or the number the reader clicked would not be the
                // number they get. A stale single-day selection would likewise
                // leave the feed showing fewer items than the tile.
                patch({
                  country: cc,
                  category: cat,
                  tone: "negative",
                  days: 7,
                  day: "",
                });
                revealSection("feed");
              }}
            />
          </Section>

          <div className="filters" role="group" aria-label="Filters">
            {DAY_PRESETS.map((d) => (
              <button
                key={d}
                aria-pressed={days === d}
                onClick={() => patch({ days: d })}
              >
                {d} days
              </button>
            ))}
            <select
              value={country}
              onChange={(e) => patch({ country: e.target.value })}
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
              onChange={(e) => patch({ tone: e.target.value })}
              aria-label="Direction"
            >
              <option value="">All directions</option>
              <option value="negative">▼ Adverse only</option>
              <option value="positive">▲ Favourable only</option>
              <option value="neutral">● Neutral only</option>
            </select>
            <select
              value={category}
              onChange={(e) => patch({ category: e.target.value })}
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
            title={`Incidents per day${country ? ` — ${countryName(country)}` : ""}`}
          >
            <Timeline
              buckets={timeline}
              onSelectDay={(d) => {
                patch({ day: d });
                revealSection("feed");
              }}
            />
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
                revealSection("map");
              }}
            />
          </Section>

          <Section
            id="feed"
            title="Incident feed"
            aside={`${incidents.length} shown${category ? ` · ${categoryLabel(category)}` : ""}`}
          >
            {chips.length > 0 && (
              <p className="filter-chips" role="group" aria-label="Active filters">
                <span>Filtered:</span>
                {chips.map((c) => (
                  <button
                    key={c.label}
                    className="chip-clear"
                    onClick={c.clear}
                    title="Remove this filter"
                  >
                    {c.label} ✕
                  </button>
                ))}
                <button
                  className="linklike"
                  onClick={() => setFilters({ ...DEFAULT_FILTERS })}
                >
                  clear all
                </button>
              </p>
            )}
            {/* Both links carry the filters currently on screen, so what a
                reader downloads is what they were looking at. */}
            <p className="export-links">
              Download these filters (up to 500 items):{" "}
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
