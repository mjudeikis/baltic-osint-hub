export interface Posture {
  level: number;
  level_name: string;
  headline: string;
  explanation: string;
  balance: number;
  context: string;
  typical_week: number;
  // "escalating" | "steady" | "de-escalating"; empty when history is too short.
  trend: string;
  counts: {
    positive: number;
    neutral: number;
    negative: number;
    negative_by_severity: number[];
  };
  // The severity-4/5 event behind an Elevated-or-above reading, so the
  // headline never says "a serious event" while withholding which one.
  trigger_event?: {
    id: number;
    summary: string;
    severity: number;
    corroborated: boolean;
  };
}

export const fetchPosture = (country?: string, signal?: AbortSignal) =>
  get<Posture>(`/api/stats/posture${country ? `?country=${country}` : ""}`, signal);

export interface Incident {
  id: number;
  category: string;
  countries: string[];
  severity: number;
  tone: string;
  place: string;
  credibility: string; // institutional | independent | state-controlled
  summary: string;
  lat?: number;
  lon?: number;
  occurred_at: string;
  source: string;
  url: string;
  title: string;
  // Event clustering. reports counts the articles behind this event and
  // sources names them. confidence_label is absent while an incident has not
  // been clustered yet — which means "not assessed", not "uncorroborated".
  event_id?: number;
  reports: number;
  sources?: string[];
  independent_sources?: number;
  confidence: number;
  confidence_label?: string;
}

export interface TimelineBucket {
  day: string;
  category: string;
  count: number;
}

export interface SummaryCell {
  country: string;
  category: string;
  recent: number;
  recent_adverse: number;
  recent_favourable: number;
  baseline: number;
  baseline_samples: number;
  max_severity: number;
  // Restricted to corroborated events, so the board's severity label agrees
  // with the posture banner rather than contradicting it.
  max_severity_corroborated: number;
}

export interface SourceStatus {
  source: string;
  last_run: string;
  items_found: number;
  items_new: number;
  error: string;
}

// Responses are cached for 5 minutes at the edge. Without a per-build cache
// key a freshly deployed bundle can be served a response from the previous
// schema, which renders as missing fields until the cache expires.
const BUILD = (import.meta as { env?: Record<string, string> }).env?.VITE_BUILD_ID ?? "dev";

// Every fetch takes an optional AbortSignal so a caller can cancel a request
// its own state has moved past (a filter changed twice in quick succession),
// combined with the timeout below. Older browsers lack AbortSignal.any; there
// the caller's signal wins, since a superseded response is the worse bug.
function withTimeout(signal?: AbortSignal): AbortSignal {
  const timeout = AbortSignal.timeout(15_000);
  if (!signal) return timeout;
  if (typeof AbortSignal.any === "function") return AbortSignal.any([signal, timeout]);
  return signal;
}

async function get<T>(path: string, signal?: AbortSignal): Promise<T> {
  const url = path + (path.includes("?") ? "&" : "?") + "v=" + BUILD;
  // Without a timeout a hung request never resolves into an error state and
  // the page shows "loading" forever; 15 s is far beyond any healthy response.
  const res = await fetch(url, { signal: withTimeout(signal) });
  if (!res.ok) throw new Error(`${path}: ${res.status}`);
  return res.json();
}

// True for the rejection a caller's own abort produces, which is not an
// error to report — the request was cancelled on purpose.
export const isAbort = (e: unknown): boolean =>
  e instanceof DOMException && e.name === "AbortError";

export interface IncidentQuery {
  category?: string;
  country?: string;
  tone?: string;
  severity?: number;
  days?: number;
  day?: string;
  limit?: number;
}

export function fetchIncidents(q: IncidentQuery, signal?: AbortSignal): Promise<Incident[]> {
  const params = new URLSearchParams();
  if (q.category) params.set("category", q.category);
  if (q.tone) params.set("tone", q.tone);
  if (q.country) params.set("country", q.country);
  if (q.severity) params.set("severity", String(q.severity));
  if (q.days) params.set("days", String(q.days));
  if (q.day) params.set("day", q.day);
  params.set("limit", String(q.limit ?? 200));
  return get(`/api/incidents?${params}`, signal);
}

// Both headline aggregates request min_severity=2: severity-1 analysis and
// commentary stay in the feed but are not counted as incidents. The API's own
// default stays 1 so external consumers see unchanged behaviour.
export const fetchTimeline = (days: number, country?: string, signal?: AbortSignal) =>
  get<TimelineBucket[]>(
    `/api/stats/timeline?days=${days}&min_severity=2${country ? `&country=${country}` : ""}`,
    signal,
  );

export const fetchSummary = (signal?: AbortSignal) =>
  get<SummaryCell[]>("/api/stats/summary?min_severity=2", signal);
export const fetchSources = (signal?: AbortSignal) =>
  get<SourceStatus[]>("/api/sources", signal);

export interface PostureRule {
  level: number;
  level_name: string;
  condition: string;
}

export interface Meta {
  categories: string[];
  countries: string[];
  tones: string[];
  posture_rules: PostureRule[];
  posture_adjustments: string[];
}

export const fetchMeta = (signal?: AbortSignal) => get<Meta>("/api/meta", signal);

// --- signal layers ---

export interface FIRMSDetection {
  lat: number;
  lon: number;
  brightness: number;
  frp: number;
  confidence: string;
  sector: string;
  detected_at: string;
}

export interface GpsjamCell {
  day: string;
  hex: string;
  good: number;
  bad: number;
}

export interface AirSighting {
  seen_at: string;
  icao24: string;
  callsign: string;
  country: string;
  box: string;
  lat?: number;
  lon?: number;
  altitude?: number;
  velocity?: number;
  reason: string;
}

export interface SanctionedVessel {
  mmsi: number;
  imo?: string;
  name: string;
  risk?: string;
  flag?: string;
  countries?: string;
  url?: string;
}

export interface SeaEvent {
  detected_at: string;
  mmsi: number;
  ship_name: string;
  corridor: string;
  lat?: number;
  lon?: number;
  sog?: number;
  event: string;
  started_at?: string;
  // Present when the vessel is on the OpenSanctions maritime watchlist.
  sanctioned?: SanctionedVessel;
  // AIS ship-and-cargo type where known (70s cargo, 80s tanker).
  ship_type?: number;
  // A listed vessel, or an extended AIS gap (4 h+) by a vessel able to damage
  // a cable. Plain loitering and short gaps are ordinary — mostly receiver
  // coverage — and are kept as baseline rather than shown as findings.
  notable: boolean;
}

export interface SarObservation {
  start: string;
  end: string;
  bright_fraction: number;
  mean_db: number;
  sample_count: number;
}

// One stored Sentinel-1 rendering of a flagged site; kind is "before"
// (representative baseline pass) or "after" (the flagged pass).
export interface SarImageMeta {
  kind: string;
  captured_start: string;
  captured_end: string;
  url: string;
}

export interface SarAOI {
  key: string;
  label: string;
  country: string;
  kind: string;
  side: string; // adversary | border | friendly
  class: string; // empty | occupied | hollow
  note: string;
  depth_km: number;
  bbox: [number, number, number, number]; // lonMin, latMin, lonMax, latMax
  browser_url: string;
  series: SarObservation[];
  anomaly: boolean;
  zscore: number;
  latest: number;
  median: number;
  baseline: number;
  scene_shifted: boolean;
  images?: SarImageMeta[];
}

// The five endpoints behind the map overlays. Keys are the API's names; the
// map's overlay keys map onto them in ../layers.
export const LAYER_SOURCES = ["firms", "gpsjam", "air", "sea", "sar"] as const;
export type LayerSource = (typeof LAYER_SOURCES)[number];

export interface Layers {
  firms: FIRMSDetection[];
  gpsjam: GpsjamCell[];
  air: AirSighting[];
  sea: SeaEvent[];
  sar: SarAOI[];
  // Endpoints that did not load this round, with the error. A failed layer
  // is "unavailable", never "0 detections": an empty array here would render
  // a FIRMS outage as a calm border, which is exactly the lie this product
  // exists not to tell.
  failed: Partial<Record<LayerSource, string>>;
}

// Settled per layer: one failing endpoint costs that layer, not the whole map.
export async function fetchLayers(signal?: AbortSignal): Promise<Layers> {
  const [firms, gpsjam, air, sea, sar] = await Promise.allSettled([
    get<FIRMSDetection[]>("/api/layers/firms?days=7", signal),
    get<GpsjamCell[]>("/api/layers/gpsjam", signal),
    get<AirSighting[]>("/api/layers/air?days=2", signal),
    get<SeaEvent[]>("/api/layers/sea?days=7", signal),
    get<SarAOI[]>("/api/layers/sar", signal),
  ]);
  const failed: Layers["failed"] = {};
  const settled = <T>(key: LayerSource, r: PromiseSettledResult<T[]>): T[] => {
    if (r.status === "fulfilled") return r.value;
    failed[key] = String(r.reason);
    return [];
  };
  const out: Layers = {
    firms: settled("firms", firms),
    gpsjam: settled("gpsjam", gpsjam),
    air: settled("air", air),
    sea: settled("sea", sea),
    sar: settled("sar", sar),
    failed,
  };
  // A caller's own abort is not five layer failures; surface it as the
  // rejection the caller expects so it can be ignored like any other.
  if (signal?.aborted) throw new DOMException("aborted", "AbortError");
  return out;
}

// One day of CERT.PL warning-list churn: domains added to and removed from
// Poland's national malicious-domain list. Counts only — the list itself is
// never surfaced here.
export interface CertPLDay {
  day: string;
  added: number;
  removed: number;
}

export const fetchCertPL = (days: number, signal?: AbortSignal) =>
  get<CertPLDay[]>(`/api/layers/certpl?days=${days}`, signal);
