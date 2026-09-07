import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  CertPLDay,
  fetchCertPL,
  fetchIncidents,
  fetchLayers,
  fetchMeta,
  fetchPosture,
  isAbort,
  LAYER_SOURCES,
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
import { formatDay } from "./dates";
import { freshness } from "./freshness";
import ThreatBoard from "./components/ThreatBoard";
import Feed from "./components/Feed";

// The two heaviest dependencies — Recharts (~400 KB) and MapLibre (~700 KB)
// — load as separate chunks so the posture card and board render from a
// small core bundle. On 3G the five-second read must not wait for a map.
// TODO(perf): the map chunk (MapLibre + h3-js) is ~290 KB gzipped and
// Recharts ~120 KB; both are far heavier than what they draw. Swapping them
// is a separate piece of work — for now the map section defaults closed on
// narrow viewports so phones do not fetch it unasked.
const Timeline = lazy(() => import("./components/Timeline"));
const IncidentMap = lazy(() => import("./components/IncidentMap"));
import SarPanel from "./components/SarPanel";
import StatusBanner, { METHODOLOGY_DOCS } from "./components/StatusBanner";
import PostureBanner from "./components/PostureBanner";
import Section, { revealSection } from "./components/Section";
import SideNav, { NavItem } from "./components/SideNav";
import SourcesPanel from "./components/SourcesPanel";
import Preparedness from "./components/Preparedness";
import CertPLPanel from "./components/CertPLPanel";
import ErrorBoundary from "./components/ErrorBoundary";

import {
  DAY_PRESETS,
  DEFAULT_FILTERS,
  FilterState,
  exportURL,
  readFilters,
  syncURL,
} from "./urlState";

const NAV_ITEMS: NavItem[] = [
  { id: "board", label: "By country", primary: true },
  { id: "trend", label: "Trend", primary: true },
  { id: "map", label: "Situation map" },
  { id: "satellite", label: "Satellite" },
  { id: "feed", label: "Incident feed", primary: true, short: "Feed" },
  { id: "prepare", label: "How to prepare", primary: true, short: "Prepare" },
  { id: "sources", label: "Sources" },
];

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
  certpl: "the CERT.PL cyber rate",
  "layer:firms": "the thermal (FIRMS) map layer",
  "layer:gpsjam": "the GPS jamming map layer",
  "layer:air": "the air activity map layer",
  "layer:sea": "the sea activity map layer",
  "layer:sar": "the satellite radar sites",
};

const REFRESH_MS = 5 * 60 * 1000;

// Phones get the map collapsed by default: the section is lazy-mounted, so a
// closed map never downloads its chunk. A reader who opens it once keeps it
// open (Section persists the choice).
const NARROW = typeof window !== "undefined" && window.matchMedia("(max-width: 900px)").matches;

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
  // The current filters, readable from stable callbacks (the board's
  // onSelect is memoised so the board itself can be).
  const filtersRef = useRef(filters);
  useEffect(() => {
    filtersRef.current = filters;
  }, [filters]);

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
  const [certpl, setCertpl] = useState<CertPLDay[] | null>(null);
  const [posture, setPosture] = useState<Posture | null>(null);
  const [meta, setMeta] = useState<Meta | null>(null);
  const [focusedSite, setFocusedSite] = useState<string | null>(null);

  // Errors are tracked per endpoint so a later success clears exactly the
  // failure it retried — a single transient blip during a background refresh
  // must not leave a permanent red banner over a page that has recovered.
  const [errors, setErrors] = useState<Record<string, string>>({});
  const noteError = (key: string, message: string) =>
    setErrors((e) => (e[key] === message ? e : { ...e, [key]: message }));
  const failedParts = Object.entries(errors)
    .filter(([, msg]) => Boolean(msg))
    .map(([key]) => FAILED_LABEL[key] ?? key);

  // Periodic refresh so a dashboard left open doesn't drift — and so the
  // status banner's "last sync" can't claim freshness the rest of the page
  // doesn't have. Matches the API's 5-minute Cache-Control. A hidden tab
  // skips its ticks (nobody is reading, and phones throttle timers anyway)
  // and catches up the moment it is visible again.
  const [refreshKey, setRefreshKey] = useState(0);
  useEffect(() => {
    let lastRefresh = Date.now();
    const refresh = () => {
      lastRefresh = Date.now();
      setRefreshKey((k) => k + 1);
    };
    const id = setInterval(() => {
      if (document.visibilityState === "visible") refresh();
    }, REFRESH_MS);
    const onVisible = () => {
      if (document.visibilityState === "visible" && Date.now() - lastRefresh >= REFRESH_MS) {
        refresh();
      }
    };
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      clearInterval(id);
      document.removeEventListener("visibilitychange", onVisible);
    };
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
    const ac = new AbortController();
    const { signal } = ac;
    // Aborted requests are superseded, not failed: their rejections and any
    // late results are dropped so an older response never overwrites a
    // newer one and never paints an error the page has already moved past.
    const load = <T,>(key: string, req: Promise<T>, apply: (d: T) => void) =>
      req
        .then((d) => {
          if (signal.aborted) return;
          apply(d);
          noteError(key, "");
        })
        .catch((e) => {
          if (signal.aborted || isAbort(e)) return;
          noteError(key, String(e));
        });

    load("summary", fetchSummary(signal), setSummary);
    // severity 2+, matching the tile counts: the headlines shown must be the
    // events behind the numbers, and analysis pieces are not counted there.
    load(
      "board",
      fetchIncidents({ days: 7, tone: "negative", severity: 2, limit: 200 }, signal),
      setBoardIncidents,
    );
    load("sources", fetchSources(signal), setSources);
    load("layers", fetchLayers(signal), (d) => {
      setLayers(d);
      // Each failed endpoint is its own banner entry, so "the map signal
      // layers" never hides which layer is missing.
      for (const src of LAYER_SOURCES) noteError(`layer:${src}`, d.failed[src] ?? "");
    });
    // 90 days: enough prior weeks for a median, kept small.
    load("certpl", fetchCertPL(90, signal), setCertpl);
    return () => ac.abort();
  }, [refreshKey]);

  // The taxonomy and the posture rules are static per deploy; fetched once.
  useEffect(() => {
    const ac = new AbortController();
    fetchMeta(ac.signal)
      .then(setMeta)
      .catch(() => {});
    return () => ac.abort();
  }, []);

  // "View in the feed" from the posture banner. The feed may be collapsed
  // and its filters may exclude the event, so this reveals the section,
  // widens the filters to the defaults if the id is not in the current
  // list, and scrolls once the row exists. The pending id is settled against
  // whichever incident list is current: the one on screen at click time, or
  // the next one the feed fetch delivers.
  const pendingIncident = useRef<number | null>(null);
  const incidentsRef = useRef(incidents);
  useEffect(() => {
    incidentsRef.current = incidents;
  }, [incidents]);
  const settlePending = useCallback((list: Incident[]) => {
    const id = pendingIncident.current;
    if (id === null) return;
    if (list.some((i) => i.id === id)) {
      pendingIncident.current = null;
      requestAnimationFrame(() => {
        document.getElementById(`incident-${id}`)?.scrollIntoView({ block: "center" });
        history.replaceState(null, "", `#incident-${id}`);
      });
      return;
    }
    // Not in this view: widen once. If it is still absent under the default
    // filters there is nothing more to do (the event may have aged out).
    const f = filtersRef.current;
    const atDefaults = (Object.keys(DEFAULT_FILTERS) as (keyof FilterState)[]).every(
      (k) => f[k] === DEFAULT_FILTERS[k],
    );
    if (atDefaults) pendingIncident.current = null;
    else setFilters({ ...DEFAULT_FILTERS });
  }, []);
  const viewIncident = useCallback(
    (id: number) => {
      pendingIncident.current = id;
      revealSection("feed");
      if (incidentsRef.current) settlePending(incidentsRef.current);
    },
    [settlePending],
  );

  // Refetch-in-flight flags: stale rows dim rather than posing as current
  // while a filter change is loading (see .refetching). Only user-initiated
  // changes set them; the 5-minute background refresh is silent, since
  // dimming live content every few minutes reads as a fault.
  const [timelineBusy, setTimelineBusy] = useState(false);
  const [feedBusy, setFeedBusy] = useState(false);
  const timelineRefreshSeen = useRef(refreshKey);
  const feedRefreshSeen = useRef(refreshKey);

  useEffect(() => {
    const ac = new AbortController();
    const { signal } = ac;
    const background = timelineRefreshSeen.current !== refreshKey;
    timelineRefreshSeen.current = refreshKey;
    if (!background) setTimelineBusy(true);
    fetchTimeline(days, country || undefined, signal)
      .then((d) => {
        if (signal.aborted) return;
        setTimeline(d);
        noteError("timeline", "");
      })
      .catch((e) => {
        if (signal.aborted || isAbort(e)) return;
        noteError("timeline", String(e));
      })
      .finally(() => {
        // A superseded request leaves the flag to the request that replaced it.
        if (!signal.aborted) setTimelineBusy(false);
      });
    // Posture follows the country filter so it reads for whatever is on
    // screen. Its failure must never be silent: this is the one element the
    // visitor came for, and a swallowed error left "Reading regional
    // posture…" on screen forever.
    fetchPosture(country || undefined, signal)
      .then((d) => {
        if (signal.aborted) return;
        setPosture(d);
        noteError("posture", "");
      })
      .catch((e) => {
        if (signal.aborted || isAbort(e)) return;
        noteError("posture", String(e));
      });
    return () => ac.abort();
  }, [days, country, refreshKey]);

  useEffect(() => {
    const ac = new AbortController();
    const { signal } = ac;
    const background = feedRefreshSeen.current !== refreshKey;
    feedRefreshSeen.current = refreshKey;
    if (!background) setFeedBusy(true);
    fetchIncidents(
      {
        days,
        day: day || undefined,
        country: country || undefined,
        category: category || undefined,
        tone: tone || undefined,
        severity: sev || undefined,
      },
      signal,
    )
      .then((d) => {
        if (signal.aborted) return;
        setIncidents(d);
        noteError("incidents", "");
        settlePending(d);
      })
      .catch((e) => {
        if (signal.aborted || isAbort(e)) return;
        noteError("incidents", String(e));
      })
      .finally(() => {
        if (!signal.aborted) setFeedBusy(false);
      });
    return () => ac.abort();
  }, [days, country, category, tone, day, sev, refreshKey, settlePending]);


  const fresh = useMemo(() => freshness(sources), [sources]);

  // Stable callbacks so the memoised board and the map do not re-render on
  // every App state tick.
  const onBoardSelect = useCallback((cc: string, cat: string) => {
    // The tile counts adverse items over 7 days, so the feed must match or
    // the number the reader clicked would not be the number they get. A
    // stale single-day selection would likewise leave the feed showing fewer
    // items than the tile. sev: 2 mirrors the tile counts (analysis
    // excluded), so the number clicked equals the number of rows shown.
    setPreDrill(filtersRef.current);
    setFilters((f) => ({
      ...f,
      country: cc,
      category: cat,
      tone: "negative",
      days: 7,
      day: "",
      sev: 2,
    }));
    revealSection("feed");
  }, []);
  const onFocusHandled = useCallback(() => setFocusedSite(null), []);

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
      label: formatDay(day),
      clear: () => patch({ day: "" }),
    });
  // sev has no chip: unlike the other filters it now has a permanent,
  // visible control in the strip, so a chip would restate visible state.

  return (
    <div className="container">
      <a className="skip-link" href="#posture">
        Skip to the posture reading
      </a>
      <StatusBanner sources={sources} />

      <header className="site">
        <h1>Baltic OSINT Hub</h1>
        <p>
          Is this week unusual? Open-source tracking of hybrid-threat activity
          against Lithuania, Latvia, Estonia and Poland.
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

      {/* Main precedes the nav in DOM order so the posture reading is the
          first thing on a phone (and the first thing a screen reader meets);
          the desktop grid places the nav in the left column by CSS. */}
      <div className="layout">
        <main>
          <ErrorBoundary label="The posture reading">
            <PostureBanner
              posture={posture}
              scope={country ? countryName(country) : ""}
              meta={meta}
              onViewIncident={viewIncident}
            />
          </ErrorBoundary>

          <Section id="board" title="Last 7 days by country">
            <ThreatBoard
              cells={summary}
              incidents={boardIncidents}
              onSelect={onBoardSelect}
              fresh={fresh}
            />
            <CertPLPanel days={certpl} error={errors.certpl || undefined} />
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
            {/* Events-only is the resident default; analysis is opt-in. The
                severity filter was previously a state only drill-throughs
                could set — a chip you could remove but never add. */}
            <button
              aria-pressed={sev >= 2}
              onClick={() => patch({ sev: sev >= 2 ? 0 : 2 })}
              title="Events of severity 2 and above — severity-1 analysis and commentary hidden"
            >
              Events only
            </button>
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
                  fresh={fresh}
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

          <Section id="map" title="Situation map" defaultOpen={!NARROW}>
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
                onFocusHandled={onFocusHandled}
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
                  onClick={() => {
                    setFilters({ ...DEFAULT_FILTERS });
                    // The pre-drill snapshot is a way back from a drill, and
                    // "clear all" is a fresh start — offering "back to
                    // previous view" after it would restore a stale filter set.
                    setPreDrill(null);
                  }}
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
            {/* The methodology is part of the product: every reading above
                is auditable only if these are one click away. */}
            <p className="methodology-links">
              Read the methodology:{" "}
              {METHODOLOGY_DOCS.map((d, i) => (
                <span key={d.href}>
                  {i > 0 && " · "}
                  <a href={d.href} target="_blank" rel="noopener noreferrer">
                    {d.label}
                  </a>
                </span>
              ))}
              {" · "}
              <a
                href="https://github.com/mjudeikis/baltic-osint-hub"
                target="_blank"
                rel="noopener noreferrer"
              >
                source code
              </a>
            </p>
            <SourcesPanel sources={sources} />
          </Section>
        </main>

        <SideNav items={NAV_ITEMS} />
      </div>
    </div>
  );
}
