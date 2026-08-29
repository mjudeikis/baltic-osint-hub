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
