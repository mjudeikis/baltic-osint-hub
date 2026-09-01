import { lazy, Suspense, useEffect, useState } from "react";
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
import Feed from "./components/Feed";

// The two heaviest dependencies — Recharts (~400 KB) and MapLibre (~700 KB)
// — load as separate chunks so the posture card and board render from a
// small core bundle. On 3G the five-second read must not wait for a map.
const Timeline = lazy(() => import("./components/Timeline"));
const IncidentMap = lazy(() => import("./components/IncidentMap"));
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
  const { days, country, category, tone, day, sev } = filters;
  const patch = (p: Partial<FilterState>) => setFilters((f) => ({ ...f, ...p }));
  // A drill-through overwrites several filters at once; "clear all" honestly
  // resets to defaults, but the reader's pre-drill view deserves its own way
  // back. Snapshot taken at each drill, offered beside the chips.
  const [preDrill, setPreDrill] = useState<FilterState | null>(null);

  // Data states start as null, not [] — before the first response the page
  // must read as "loading", never as a fabricated quiet week. Rendering an
  // empty board or feed during load violates "no data is no data".
  const [summary, setSummary] = useState<SummaryCell[] | null>(null);
  // The board's own adverse list, fixed to the 7-day window the tiles count.
  // It cannot share `incidents`: that list follows the feed filters, and a
  // reader narrowing the feed must not empty the headlines on the board.
  const [boardIncidents, setBoardIncidents] = useState<Incident[]>([]);
  const [timeline, setTimeline] = useState<TimelineBucket[] | null>(null);
  const [incidents, setIncidents] = useState<Incident[] | null>(null);
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
  // Plain words, not endpoint paths: the reader needs to know which part of
  // the page is affected and that it will fix itself, not the status code.
  const FAILED_LABEL: Record<string, string> = {
    summary: "the country board",
    timeline: "the trend chart",
    incidents: "the incident feed",
    posture: "the posture reading",
    sources: "the collection status",
    board: "the board headlines",
    layers: "the map signal layers",
  };
  const failedParts = Object.entries(errors)
    .filter(([, msg]) => Boolean(msg))
    .map(([key]) => FAILED_LABEL[key] ?? key);

  // Periodic refresh so a dashboard left open doesn't drift — and so the
  // status banner's "last sync" can't claim freshness the rest of the page
  // doesn't have. Matches the API's 5-minute Cache-Control.
  const [refreshKey, setRefreshKey] = useState(0);
  useEffect(() => {
    const id = setInterval(() => setRefreshKey((k) => k + 1), 5 * 60 * 1000);
    return () => clearInterval(id);
  }, []);

  // Re-render on an OS theme flip so canvas-rendered colors (Recharts) pick
  // up the retinted tokens — taxonomy's cache clears on the same event, and
  // it registered first, so a post-flip render always reads fresh values.
  // The map's basemap stays init-time; its data colors refresh on the next
  // layer update.
  const [, setThemeTick] = useState(0);
  useEffect(() => {
    const mq = window.matchMedia("(prefers-color-scheme: dark)");
    const onChange = () => setThemeTick((t) => t + 1);
    mq.addEventListener("change", onChange);
    return () => mq.removeEventListener("change", onChange);
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
    // severity 2+, matching the tile counts: the headlines shown must be the
    // events behind the numbers, and analysis pieces are not counted there.
    fetchIncidents({ days: 7, tone: "negative", severity: 2, limit: 200 })
      .then((d) => {
        setBoardIncidents(d);
        noteError("board", "");
      })
      .catch((e) => noteError("board", String(e)));
    fetchSources()
      .then((d) => {
        setSources(d);
        noteError("sources", "");
      })
      .catch((e) => noteError("sources", String(e)));
    fetchLayers()
      .then((d) => {
        setLayers(d);
        noteError("layers", "");
      })
      .catch((e) => noteError("layers", String(e)));
  }, [refreshKey]);

  // The taxonomy and the posture rules are static per deploy; fetched once.
  useEffect(() => {
    fetchMeta()
      .then(setMeta)
      .catch(() => {});
  }, []);

  // Refetch-in-flight flags: stale rows dim rather than posing as current
  // while a filter change is loading (see .refetching).
  const [timelineBusy, setTimelineBusy] = useState(false);
  const [feedBusy, setFeedBusy] = useState(false);

  useEffect(() => {
    setTimelineBusy(true);
    fetchTimeline(days, country || undefined)
      .then((d) => {
        setTimeline(d);
        noteError("timeline", "");
      })
      .catch((e) => noteError("timeline", String(e)))
      .finally(() => setTimelineBusy(false));
    // Posture follows the country filter so it reads for whatever is on
    // screen. Its failure must never be silent: this is the one element the
    // visitor came for, and a swallowed error left "Reading regional
    // posture…" on screen forever.
    fetchPosture(country || undefined)
      .then((d) => {
        setPosture(d);
        noteError("posture", "");
      })
      .catch((e) => noteError("posture", String(e)));
  }, [days, country, refreshKey]);

  useEffect(() => {
    setFeedBusy(true);
    fetchIncidents({
      days,
      day: day || undefined,
      country: country || undefined,
      category: category || undefined,
      tone: tone || undefined,
      severity: sev || undefined,
    })
      .then((d) => {
        setIncidents(d);
        noteError("incidents", "");
      })
      .catch((e) => noteError("incidents", String(e)))
      .finally(() => setFeedBusy(false));
  }, [days, country, category, tone, day, sev, refreshKey]);

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
  if (day)
    chips.push({
      // Same date voice as the feed's day headers, not raw ISO.
      label: new Date(day).toLocaleDateString("en-GB", {
        weekday: "short",
        day: "numeric",
        month: "short",
      }),
      clear: () => patch({ day: "" }),
    });
  if (sev)
    chips.push({
      label: `Events only (severity ≥${sev})`,
      clear: () => patch({ sev: 0 }),
    });

  return (
    <div className="container">
      <a className="skip-link" href="#posture">
        Skip to the posture reading
      </a>
      <StatusBanner sources={sources} />

      <header className="site">
        <h1>Baltic OSINT Hub</h1>
        <p>
          Open-source tracking of hybrid-threat activity affecting Lithuania,
          Latvia, Estonia, and Poland — aggregated from public news, national
          cyber-security teams (CERTs), and research feeds.
        </p>
      </header>

      {failedParts.length > 0 && (
        <div
          className="card"
          role="alert"
          style={{ color: "var(--status-critical-text)" }}
        >
          Part of the page could not be loaded: {failedParts.join(", ")}.
          Everything else is current, and the dashboard retries automatically
          every few minutes.
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
                // sev: 2 mirrors the tile counts (analysis excluded), so the
                // number clicked equals the number of rows shown.
                setPreDrill(filters);
                patch({
                  country: cc,
                  category: cat,
                  tone: "negative",
                  days: 7,
                  day: "",
                  sev: 2,
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
            {timeline === null ? (
              <p style={{ color: "var(--text-muted)" }} aria-busy="true">
                Loading the incident trend…
              </p>
            ) : (
              <div className={timelineBusy ? "refetching" : undefined} aria-busy={timelineBusy}>
                <Suspense
                  fallback={
                    <p style={{ color: "var(--text-muted)" }} aria-busy="true">
                      Loading the incident trend…
                    </p>
                  }
                >
                <Timeline
                  buckets={timeline}
                  days={days}
                  onSelectDay={(d) => {
                    setPreDrill(filters);
                    patch({ day: d });
                    revealSection("feed");
                  }}
                />
                </Suspense>
              </div>
            )}
          </Section>

          <Section id="map" title="Situation map">
            <Suspense
              fallback={
                <p style={{ color: "var(--text-muted)" }} aria-busy="true">
                  Loading the map…
                </p>
              }
            >
              <IncidentMap
                incidents={incidents ?? []}
                layers={layers}
                focusedSite={focusedSite}
                onFocusHandled={() => setFocusedSite(null)}
              />
            </Suspense>
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
            aside={
              // aria-live so a drill-through from the board or timeline is
              // announced — the view changes, and focus alone can't say how.
              <span aria-live="polite">
                {incidents === null
                  ? "loading…"
                  : `${incidents.length} shown${category ? ` · ${categoryLabel(category)}` : ""}`}
              </span>
            }
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
                {preDrill && (
                  <button
                    className="linklike"
                    onClick={() => {
                      setFilters(preDrill);
                      setPreDrill(null);
                    }}
                  >
                    ← back to previous view
                  </button>
                )}
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
            {incidents === null ? (
              <p style={{ color: "var(--text-muted)" }} aria-busy="true">
                Loading incidents…
              </p>
            ) : (
              <div className={feedBusy ? "refetching" : undefined} aria-busy={feedBusy}>
                <Feed incidents={incidents} />
              </div>
            )}
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
