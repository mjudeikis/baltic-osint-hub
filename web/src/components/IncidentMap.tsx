import { useEffect, useRef, useState } from "react";
import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { cellToBoundary, cellToLatLng } from "h3-js";
import { Incident, Layers } from "../api";
import { categoryLabel, cssColor, severityColor, SEVERITY_LABELS } from "../taxonomy";

// Encoding: incident markers use the sequential blue ramp for severity;
// jamming cells are a one-hue orange ramp (opacity = share of affected
// aircraft); thermal/air/sea overlays carry identity via categorical slots
// (red/violet/green) — never rank-assigned.
const OVERLAYS = [
  { key: "jamming", label: "GPS jamming", cssVar: "--series-2" },
  { key: "thermal", label: "Thermal (FIRMS)", cssVar: "--series-8" },
  { key: "air", label: "Air activity", cssVar: "--series-7" },
  { key: "sea", label: "Sea activity", cssVar: "--series-6" },
  { key: "sites", label: "Radar sites", cssVar: "--series-4" },
  { key: "cables", label: "Cables & pipelines", cssVar: "--series-3" },
  { key: "territory", label: "RU / BY territory", cssVar: "--series-8" },
] as const;
type OverlayKey = (typeof OVERLAYS)[number]["key"];

const REGION = { latMin: 47, latMax: 63, lonMin: 8, lonMax: 34 };

export default function IncidentMap({
  incidents,
  layers,
  focusedSite,
  onFocusHandled,
}: {
  incidents: Incident[];
  layers: Layers | null;
  focusedSite?: string | null;
  onFocusHandled?: () => void;
}) {
  const el = useRef<HTMLDivElement>(null);
  const map = useRef<maplibregl.Map | null>(null);
  const markers = useRef<maplibregl.Marker[]>([]);
  const sitePopup = useRef<maplibregl.Popup | null>(null);
  const [loaded, setLoaded] = useState(false);
  const [visible, setVisible] = useState<Record<OverlayKey, boolean>>({
    jamming: true,
    thermal: true,
    air: true,
    sea: true,
    sites: true,
    cables: true,
    territory: true,
  });
  const [showIncidents, setShowIncidents] = useState(true);

  useEffect(() => {
    if (!el.current || map.current) return;
    const m = new maplibregl.Map({
      container: el.current,
      style: {
        version: 8,
        sources: {
          osm: {
            type: "raster",
            tiles: ["https://tile.openstreetmap.org/{z}/{x}/{y}.png"],
            tileSize: 256,
            attribution: "© OpenStreetMap contributors",
          },
        },
        layers: [{ id: "osm", type: "raster", source: "osm" }],
      },
      center: [23.5, 56.5],
      zoom: 4.4,
      attributionControl: { compact: true },
    });
    m.addControl(new maplibregl.NavigationControl({ showCompass: false }));
    m.on("load", () => setLoaded(true));
    map.current = m;
    (window as any).__osintMap = m;
    return () => {
      sitePopup.current?.remove();
      sitePopup.current = null;
      m.remove();
      map.current = null;
      setLoaded(false);
    };
  }, []);

  // Incident markers (DOM markers stay above canvas layers).
  useEffect(() => {
    if (!map.current) return;
    markers.current.forEach((mk) => mk.remove());
    markers.current = [];
    if (!showIncidents) return;
    for (const inc of incidents) {
      if (inc.lat == null || inc.lon == null) continue;
      const dot = document.createElement("div");
      const size = 8 + inc.severity * 2;
      Object.assign(dot.style, {
        width: `${size}px`,
        height: `${size}px`,
        borderRadius: "50%",
        background: severityColor(inc.severity),
        border: "2px solid var(--surface-1)",
        cursor: "pointer",
      });
      const popup = new maplibregl.Popup({ offset: 10, maxWidth: "280px" }).setHTML(
        `<strong>${esc(inc.title)}</strong><br/>` +
          (inc.place ? `📍 ${esc(inc.place)}<br/>` : "") +
          `${categoryLabel(inc.category)} · severity ${inc.severity} (${SEVERITY_LABELS[inc.severity]})<br/>` +
          `<a href="${esc(inc.url)}" target="_blank" rel="noopener noreferrer">source ↗</a>`,
      );
      markers.current.push(
        new maplibregl.Marker({ element: dot })
          .setLngLat([inc.lon, inc.lat])
          .setPopup(popup)
          .addTo(map.current),
      );
    }
  }, [incidents, showIncidents]);

  // Static geographic context, loaded once and drawn beneath the data: which
  // side of the line a thing is on, and what infrastructure runs under the
  // sea. Without these an incident in open water has nothing to mean.
  useEffect(() => {
    const m = map.current;
    if (!m || !loaded) return;
    let cancelled = false;

    (async () => {
      try {
        const [land, cables] = await Promise.all([
          fetch("/adversary-landmass.json").then((r) => r.json()),
          fetch("/baltic-cables.json").then((r) => r.json()),
        ]);
        if (cancelled || !map.current) return;

        if (!m.getSource("territory")) {
          m.addSource("territory", { type: "geojson", data: land });
          // Beneath every data layer: context, not content.
          const first = m.getLayersOrder()[1];
          m.addLayer(
            {
              id: "territory",
              type: "fill",
              source: "territory",
              paint: {
                "fill-color": cssColor("--series-8"),
                "fill-opacity": 0.1,
                "fill-outline-color": cssColor("--series-8"),
              },
            },
            first,
          );
        }
        if (!m.getSource("cables")) {
          m.addSource("cables", { type: "geojson", data: cables });
          m.addLayer({
            id: "cables",
            type: "line",
            source: "cables",
            paint: {
              "line-color": cssColor("--series-3"),
              "line-width": 1.5,
              "line-opacity": 0.75,
              "line-dasharray": [3, 2],
            },
          });
          bindPopup(m, "cables", (p) =>
            `<strong>${esc(p.name)}</strong><br/>submarine cable or interconnector`,
          );
        }
      } catch {
        /* context layers are optional; the map is useful without them */
      }
    })();

    return () => {
      cancelled = true;
    };
  }, [loaded]);

  // Canvas overlays.
  useEffect(() => {
    const m = map.current;
    if (!m || !loaded || !layers) return;

    setGeoJSON(m, "jamming", jammingGeoJSON(layers), () => {
      m.addLayer({
        id: "jamming",
        type: "fill",
        source: "jamming",
        paint: {
          "fill-color": cssColor("--series-2"),
          "fill-opacity": [
            "interpolate",
            ["linear"],
            ["get", "ratio"],
            0, 0.12,
            1, 0.55,
          ],
        },
      });
      bindPopup(m, "jamming", (p) =>
        `<strong>GPS interference</strong><br/>${p.bad} of ${p.total} aircraft affected (${Math.round(p.ratio * 100)}%)<br/>day: ${p.day}`,
      );
    });

    setGeoJSON(m, "thermal", pointGeoJSON(layers.firms, (d) => ({
      lon: d.lon, lat: d.lat,
      frp: d.frp, sector: d.sector, when: d.detected_at,
    })), () => {
      m.addLayer({
        id: "thermal",
        type: "circle",
        source: "thermal",
        paint: {
          "circle-color": cssColor("--series-8"),
          "circle-radius": ["interpolate", ["linear"], ["get", "frp"], 0, 3, 50, 9],
          "circle-opacity": 0.75,
          "circle-stroke-color": cssColor("--surface-1"),
          "circle-stroke-width": 1,
        },
      });
      bindPopup(m, "thermal", (p) =>
        `<strong>Thermal anomaly</strong><br/>sector: ${esc(p.sector)} · FRP ${p.frp} MW<br/>${new Date(p.when).toLocaleString("en-GB")}`,
      );
    });

    setGeoJSON(m, "air", pointGeoJSON(layers.air, (a) => ({
      lon: a.lon, lat: a.lat,
      callsign: a.callsign || a.icao24, country: a.country,
      reason: a.reason, box: a.box, when: a.seen_at,
    })), () => {
      m.addLayer({
        id: "air",
        type: "circle",
        source: "air",
        paint: {
          "circle-color": cssColor("--series-7"),
          "circle-radius": 7,
          "circle-stroke-color": cssColor("--surface-1"),
          "circle-stroke-width": 2,
        },
      });
      bindPopup(m, "air", (p) =>
        `<strong>✈ ${esc(p.callsign)}</strong> (${esc(p.country)})<br/>${esc(p.reason)} · ${esc(p.box)}<br/>${new Date(p.when).toLocaleString("en-GB")}`,
      );
    });

    // Notable events (a listed vessel, or a transponder going dark) are drawn
    // larger and in the status colour; ordinary stops stay small and muted so
    // they read as background traffic rather than as findings.
    setGeoJSON(m, "sea", pointGeoJSON(layers.sea, (s) => ({
      lon: s.lon, lat: s.lat,
      name: s.ship_name || String(s.mmsi), event: s.event,
      corridor: s.corridor, sog: s.sog, when: s.detected_at,
      notable: s.notable ? 1 : 0,
      sanctioned: s.sanctioned?.name ?? "",
      risk: s.sanctioned?.risk ?? "",
      flag: s.sanctioned?.flag ?? "",
      url: s.sanctioned?.url ?? "",
    })), () => {
      m.addLayer({
        id: "sea",
        type: "circle",
        source: "sea",
        paint: {
          "circle-color": [
            "case", ["==", ["get", "notable"], 1],
            cssColor("--status-warning"), cssColor("--series-6"),
          ],
          "circle-radius": ["case", ["==", ["get", "notable"], 1], 8, 4],
          "circle-opacity": ["case", ["==", ["get", "notable"], 1], 1, 0.45],
          "circle-stroke-color": cssColor("--surface-1"),
          "circle-stroke-width": 2,
        },
      });
      bindPopup(m, "sea", (p) => {
        const listed = p.sanctioned
          ? `<br/><strong>⚠ listed:</strong> ${esc(p.sanctioned)}${p.risk ? ` (${esc(p.risk)})` : ""}${p.flag ? ` · flag ${esc(p.flag)}` : ""}` +
            (p.url ? `<br/><a href="${esc(p.url)}" target="_blank" rel="noopener noreferrer">OpenSanctions entry</a>` : "")
          : "";
        return `<strong>⚓ ${esc(p.name)}</strong><br/>${esc(p.event)} · ${esc(p.corridor)} · ${p.sog ?? "?"} kn<br/>${new Date(p.when).toLocaleString("en-GB")}${listed}`;
      });
    });

    // SAR monitored sites: outlined always, filled when the latest pass
    // deviates from the site's own baseline.
    setGeoJSON(m, "sites", sitesGeoJSON(layers), () => {
      m.addLayer({
        id: "sites-fill",
        type: "fill",
        source: "sites",
        paint: {
          "fill-color": cssColor("--status-serious"),
          "fill-opacity": ["case", ["get", "anomaly"], 0.35, 0],
        },
      });
      m.addLayer({
        id: "sites",
        type: "line",
        source: "sites",
        paint: {
          "line-color": [
            "case",
            ["get", "anomaly"],
            cssColor("--status-serious"),
            cssColor("--series-4"),
          ],
          "line-width": 2,
        },
      });
      bindPopup(m, "sites", (p) =>
        `<strong>${esc(p.label)}</strong><br/>${esc(p.country)} · ${esc(p.kind)}<br/>` +
          (p.measured
            ? `bright pixels ${(p.latest * 100).toFixed(1)}% (baseline ${(p.median * 100).toFixed(1)}%)<br/>` +
              `${p.anomaly ? "▲ change detected" : "○ nominal"}<br/>`
            : "no radar measurements yet<br/>") +
          `<a href="${esc(p.browser)}" target="_blank" rel="noopener noreferrer">inspect imagery ↗</a>`,
      );
    });
  }, [layers, loaded]);

  // Visibility toggles.
  useEffect(() => {
    const m = map.current;
    if (!m || !loaded) return;
    for (const o of OVERLAYS) {
      // "sites" is drawn as an outline plus a fill; both follow one toggle.
      for (const id of o.key === "sites" ? ["sites", "sites-fill"] : [o.key]) {
        if (m.getLayer(id)) {
          m.setLayoutProperty(id, "visibility", visible[o.key] ? "visible" : "none");
        }
      }
    }
  }, [visible, loaded, layers]);

  // Selecting a site in the satellite panel flies the map to it and opens its
  // detail popup, so the panel and the map stay one view rather than two.
  useEffect(() => {
    const m = map.current;
    if (!m || !loaded || !focusedSite || !layers) return;
    const aoi = layers.sar.find((a) => a.key === focusedSite);
    if (!aoi) return;

    const [lonMin, latMin, lonMax, latMax] = aoi.bbox;
    m.fitBounds(
      [
        [lonMin, latMin],
        [lonMax, latMax],
      ],
      { padding: 140, maxZoom: 12, duration: document.hidden ? 0 : 900 },
    );
    setVisible((v) => ({ ...v, sites: true }));

    // Held in a ref rather than torn down by the effect cleanup: clearing the
    // focus re-runs this effect, and a cleanup-owned popup would remove itself
    // the instant it appeared.
    sitePopup.current?.remove();
    const popup = new maplibregl.Popup({ maxWidth: "320px" })
      .setLngLat([(lonMin + lonMax) / 2, (latMin + latMax) / 2])
      .setHTML(
        `<strong>${esc(aoi.label)}</strong><br/>${esc(aoi.country)} · ${esc(aoi.kind)}` +
          (aoi.side === "adversary" && aoi.depth_km
            ? ` · ~${aoi.depth_km} km behind the border`
            : "") +
          (aoi.note ? `<br/><span style="opacity:.85">${esc(aoi.note)}</span>` : "") +
          (aoi.series.length
            ? `<br/><br/>bright pixels <strong>${(aoi.latest * 100).toFixed(1)}%</strong>` +
              ` (baseline ${(aoi.median * 100).toFixed(1)}%)<br/>` +
              (aoi.baseline < 8
                ? "○ baselining"
                : aoi.scene_shifted
                  ? "◌ conditions changed — comparison suppressed"
                  : aoi.anomaly
                    ? "▲ change detected"
                    : "○ nominal")
            : "<br/><br/>no radar measurements yet") +
          `<br/><a href="${esc(aoi.browser_url)}" target="_blank" rel="noopener noreferrer">inspect imagery ↗</a>`,
      )
      .addTo(m);

    sitePopup.current = popup;
    onFocusHandled?.();
  }, [focusedSite, loaded, layers, onFocusHandled]);

  const counts: Record<OverlayKey, number> = {
    jamming: layers?.gpsjam.length ?? 0,
    thermal: layers?.firms.length ?? 0,
    air: layers?.air.length ?? 0,
    // Sea counts only the notable events. The rest is ordinary harbour
    // traffic kept as baseline; putting 120 in the toggle implied 120 things
    // worth looking at when four were.
    sea: layers?.sea.filter((s) => s.notable).length ?? 0,
    sites: layers?.sar.length ?? 0,
    cables: 0,
    territory: 0,
  };

  return (
    <>
      <div className="filters" role="group" aria-label="Map layers" style={{ marginTop: 0, marginBottom: 10 }}>
        <button aria-pressed={showIncidents} onClick={() => setShowIncidents((v) => !v)}>
          <span className="swatch" style={{ background: cssColor("--seq-3"), borderRadius: "50%", display: "inline-block", width: 10, height: 10, marginRight: 5 }} />
          Incidents
        </button>
        {OVERLAYS.map((o) => (
          <button
            key={o.key}
            aria-pressed={visible[o.key]}
            onClick={() => setVisible((v) => ({ ...v, [o.key]: !v[o.key] }))}
          >
            <span className="swatch" style={{ background: cssColor(o.cssVar), borderRadius: o.key === "jamming" ? 3 : "50%", display: "inline-block", width: 10, height: 10, marginRight: 5 }} />
            {o.label}
            {o.key !== "cables" && o.key !== "territory" && ` (${counts[o.key]})`}
          </button>
        ))}
      </div>
      <div ref={el} className="map-wrap" role="region" aria-label="Incident map" />
      <div className="legend">
        {[1, 2, 3, 4, 5].map((s) => (
          <span className="key" key={s}>
            <span className="swatch" style={{ background: severityColor(s), borderRadius: "50%" }} />
            {s} · {SEVERITY_LABELS[s]}
          </span>
        ))}
        <span style={{ color: "var(--text-muted)" }}>
          incident severity · jamming shading = share of affected aircraft (previous day) ·
          thermal/air/sea are machine detections, not verified incidents · sea shows
          listed vessels and AIS gaps; routine stops are dimmed
        </span>
      </div>
    </>
  );
}

// setGeoJSON updates a source in place, creating source+layer on first use.
function setGeoJSON(
  m: maplibregl.Map,
  id: string,
  data: GeoJSON.FeatureCollection,
  create: () => void,
) {
  const src = m.getSource(id) as maplibregl.GeoJSONSource | undefined;
  if (src) {
    src.setData(data);
    return;
  }
  m.addSource(id, { type: "geojson", data });
  create();
}

function bindPopup(
  m: maplibregl.Map,
  layerId: string,
  html: (props: Record<string, any>) => string,
) {
  m.on("click", layerId, (e) => {
    // A marker sits above the canvas; without this the same click would also
    // open the underlying cell's popup.
    const target = e.originalEvent?.target as Element | null;
    if (target?.closest?.(".maplibregl-marker")) return;
    const f = e.features?.[0];
    if (!f) return;
    new maplibregl.Popup({ maxWidth: "280px" })
      .setLngLat(e.lngLat)
      .setHTML(html(f.properties as Record<string, any>))
      .addTo(m);
  });
  m.on("mouseenter", layerId, () => (m.getCanvas().style.cursor = "pointer"));
  m.on("mouseleave", layerId, () => (m.getCanvas().style.cursor = ""));
}

function jammingGeoJSON(layers: Layers): GeoJSON.FeatureCollection {
  const features: GeoJSON.Feature[] = [];
  for (const c of layers.gpsjam) {
    const [lat, lon] = cellToLatLng(c.hex);
    if (lat < REGION.latMin || lat > REGION.latMax || lon < REGION.lonMin || lon > REGION.lonMax) {
      continue;
    }
    const ring = cellToBoundary(c.hex, true); // [lng, lat], closed
    const total = c.good + c.bad;
    features.push({
      type: "Feature",
      geometry: { type: "Polygon", coordinates: [ring] },
      properties: {
        ratio: total > 0 ? c.bad / total : 0,
        bad: c.bad,
        total,
        day: c.day.slice(0, 10),
      },
    });
  }
  return { type: "FeatureCollection", features };
}

function sitesGeoJSON(layers: Layers): GeoJSON.FeatureCollection {
  return {
    type: "FeatureCollection",
    features: layers.sar.map((a) => {
      const [lonMin, latMin, lonMax, latMax] = a.bbox;
      return {
        type: "Feature",
        geometry: {
          type: "Polygon",
          coordinates: [
            [
              [lonMin, latMin],
              [lonMax, latMin],
              [lonMax, latMax],
              [lonMin, latMax],
              [lonMin, latMin],
            ],
          ],
        },
        properties: {
          label: a.label,
          country: a.country,
          kind: a.kind,
          anomaly: a.anomaly && a.baseline >= 8,
          measured: a.series.length > 0,
          latest: a.latest,
          median: a.median,
          browser: a.browser_url,
        },
      };
    }),
  };
}

function pointGeoJSON<T>(
  items: T[],
  toProps: (item: T) => Record<string, unknown> & { lon?: number; lat?: number },
): GeoJSON.FeatureCollection {
  const features: GeoJSON.Feature[] = [];
  for (const item of items) {
    const { lon, lat, ...props } = toProps(item);
    if (lon == null || lat == null) continue;
    features.push({
      type: "Feature",
      geometry: { type: "Point", coordinates: [lon, lat] },
      properties: props,
    });
  }
  return { type: "FeatureCollection", features };
}

function esc(s: string): string {
  return String(s).replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}
