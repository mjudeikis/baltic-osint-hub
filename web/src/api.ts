export interface Incident {
  id: number;
  category: string;
  countries: string[];
  severity: number;
  summary: string;
  lat?: number;
  lon?: number;
  occurred_at: string;
  source: string;
  url: string;
  title: string;
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
  baseline: number;
  max_severity: number;
}

export interface SourceStatus {
  source: string;
  last_run: string;
  items_found: number;
  items_new: number;
  error: string;
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(path);
  if (!res.ok) throw new Error(`${path}: ${res.status}`);
  return res.json();
}

export interface IncidentQuery {
  category?: string;
  country?: string;
  severity?: number;
  days?: number;
  limit?: number;
}

export function fetchIncidents(q: IncidentQuery): Promise<Incident[]> {
  const params = new URLSearchParams();
  if (q.category) params.set("category", q.category);
  if (q.country) params.set("country", q.country);
  if (q.severity) params.set("severity", String(q.severity));
  if (q.days) params.set("days", String(q.days));
  params.set("limit", String(q.limit ?? 200));
  return get(`/api/incidents?${params}`);
}

export const fetchTimeline = (days: number, country?: string) =>
  get<TimelineBucket[]>(
    `/api/stats/timeline?days=${days}${country ? `&country=${country}` : ""}`,
  );

export const fetchSummary = () => get<SummaryCell[]>("/api/stats/summary");
export const fetchSources = () => get<SourceStatus[]>("/api/sources");

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
}

export interface SarObservation {
  start: string;
  end: string;
  bright_fraction: number;
  mean_db: number;
  sample_count: number;
}

export interface SarAOI {
  key: string;
  label: string;
  country: string;
  kind: string;
  bbox: [number, number, number, number]; // lonMin, latMin, lonMax, latMax
  browser_url: string;
  series: SarObservation[];
  anomaly: boolean;
  zscore: number;
  latest: number;
  median: number;
  baseline: number;
}

export interface Layers {
  firms: FIRMSDetection[];
  gpsjam: GpsjamCell[];
  air: AirSighting[];
  sea: SeaEvent[];
  sar: SarAOI[];
}

export async function fetchLayers(): Promise<Layers> {
  const [firms, gpsjam, air, sea, sar] = await Promise.all([
    get<FIRMSDetection[]>("/api/layers/firms?days=7"),
    get<GpsjamCell[]>("/api/layers/gpsjam"),
    get<AirSighting[]>("/api/layers/air?days=2"),
    get<SeaEvent[]>("/api/layers/sea?days=7"),
    get<SarAOI[]>("/api/layers/sar"),
  ]);
  return { firms, gpsjam, air, sea, sar };
}
