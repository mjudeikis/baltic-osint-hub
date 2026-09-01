import { useEffect, useRef, useState } from "react";
import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { cellToBoundary, cellToLatLng } from "h3-js";
import MapLegend from "./MapLegend";
import { makeIcon, Swatch } from "../shapes";
import { Incident, Layers } from "../api";
import { INCIDENTS_DEF, MAP_LAYERS, OverlayKey } from "../layers";
import { categoryLabel, cssColor, severityColor, SEVERITY_LABELS } from "../taxonomy";

// Encoding: incident markers (the human-reported layer) use the sequential
// blue ramp for severity; machine-measured overlays use the muted --layer-*
// instrument family (opacity = intensity where applicable), so a detection
// never wears a category's colour. Shape is the layer's identity and is
// fixed; colour reinforces it but never carries it alone — see ../shapes.
// Layer identity (key, label, colour, shape) lives in ../layers, shared with
// the legend.

// The overlays whose map icon is a shape bitmap, registered on load.
const ICON_LAYERS = ["thermal", "air", "sea", "searoutine"] as const;

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
  const sitePopup = useRef<maplibregl.Popup | null>(null);
  const [loaded, setLoaded] = useState(false);
  // WebGL can be unavailable (disabled by policy, old GPUs, some privacy
  // browsers) and the MapLibre constructor throws — which, uncaught, unmounts
  // the entire app. The map is one section; its failure must cost the map.
  const [mapFailed, setMapFailed] = useState(false);
  const [visible, setVisible] = useState<Record<OverlayKey, boolean>>(
    () =>
      Object.fromEntries(
        MAP_LAYERS.map((l) => [l.key, l.defaultVisible]),
      ) as Record<OverlayKey, boolean>,
  );
  const [showIncidents, setShowIncidents] = useState(true);

  useEffect(() => {
    if (!el.current || map.current) return;
    let m: maplibregl.Map;
    try {
      m = new maplibregl.Map({
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
        // A fixed-height canvas mid-page otherwise swallows the scroll
        // gesture: a phone thumb-drag pans the map instead of the page.
        // Cooperative gestures require ctrl+scroll / two fingers, like every
        // embedded map.
        cooperativeGestures: true,
      });
    } catch {
      setMapFailed(true);
      return;
    }
    m.addControl(new maplibregl.NavigationControl({ showCompass: false }));
    m.on("load", () => {
      // Register the shape bitmaps before any symbol layer references them.
      // A symbol layer whose icon-image is missing renders nothing at all and
      // logs only a warning, so this must happen on load, not lazily. Shape
      // and colour come from the shared layer defs, so the icon on the map is
      // the icon in the toggles and the legend by construction.
      const surface = cssColor("--surface-1");
      for (const key of ICON_LAYERS) {
        const def = MAP_LAYERS.find((l) => l.key === key)!;
        const id = `sh-${key}`;
        if (m.hasImage(id)) continue;
        const img = makeIcon(def.shape, cssColor(def.cssVar), !def.hollow, surface);
        if (img) m.addImage(id, img, { pixelRatio: 4 });
      }
      setLoaded(true);
    });
    map.current = m;
    if ((import.meta as { env?: Record<string, unknown> }).env?.DEV) {
      (window as unknown as Record<string, unknown>).__osintMap = m;
    }
    return () => {
      sitePopup.current?.remove();
      sitePopup.current = null;
      m.remove();
      map.current = null;
      setLoaded(false);
    };
  }, []);

  // Incidents as a circle layer, like every other data layer: no per-incident
  // DOM nodes, consistent popup behaviour, and MapLibre handles z-order.
  useEffect(() => {
    const m = map.current;
    if (!m || !loaded) return;
    setGeoJSON(m, "incidents", pointGeoJSON(incidents, (inc) => ({
      lon: inc.lon, lat: inc.lat,
      title: inc.title, place: inc.place ?? "",
      category: inc.category, severity: inc.severity, url: inc.url,
    })), () => {
      m.addLayer({
        id: "incidents",
        type: "circle",
        source: "incidents",
        paint: {
          // Same sizing as the legend swatches: radius grows with severity.
          "circle-radius": ["+", 4, ["get", "severity"]],
          "circle-color": [
            "match", ["get", "severity"],
            1, cssColor("--seq-1"),
            2, cssColor("--seq-2"),
            3, cssColor("--seq-3"),
            4, cssColor("--seq-4"),
            5, cssColor("--seq-5"),
            cssColor("--seq-3"),
          ],
          "circle-stroke-width": 2,
          "circle-stroke-color": cssColor("--surface-1"),
        },
      });
      bindPopup(m, "incidents", (p) =>
        `<strong>${esc(p.title)}</strong><br/>` +
          (p.place ? `📍 ${esc(p.place)}<br/>` : "") +
          `${categoryLabel(p.category)} · severity ${p.severity} (${SEVERITY_LABELS[p.severity]})<br/>` +
          `<a href="${esc(p.url)}" target="_blank" rel="noopener noreferrer">source ↗</a>`,
      );
    });
  }, [incidents, loaded]);

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
                "fill-color": cssColor("--layer-territory"),
                "fill-opacity": 0.1,
                "fill-outline-color": cssColor("--layer-territory"),
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
              "line-color": cssColor("--layer-cable"),
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
          "fill-color": cssColor("--layer-jamming"),
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
        type: "symbol",
        source: "thermal",
        layout: {
          "icon-image": "sh-thermal",
          // Fire radiative power drives size: a bigger fire is a bigger mark.
          "icon-size": ["interpolate", ["linear"], ["get", "frp"], 0, 0.35, 50, 0.8],
          "icon-allow-overlap": true,
          "icon-ignore-placement": true,
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
        type: "symbol",
        source: "air",
        layout: {
          "icon-image": "sh-air",
          "icon-size": 0.6,
          "icon-allow-overlap": true,
          "icon-ignore-placement": true,
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
      started: s.started_at ?? "",
      notable: s.notable ? 1 : 0,
      sanctioned: s.sanctioned?.name ?? "",
      risk: s.sanctioned?.risk ?? "",
      flag: s.sanctioned?.flag ?? "",
      url: s.sanctioned?.url ?? "",
    })), () => {
      // Two layers over one source so each gets its own toggle. Same shape —
      // it is the same kind of thing — solid for a listed vessel or a
      // transponder going dark, hollow for a routine stop.
      m.addLayer({
        id: "sea",
        type: "symbol",
        source: "sea",
        filter: ["==", ["get", "notable"], 1],
        layout: {
          "icon-image": "sh-sea",
          "icon-size": 0.7,
          "icon-allow-overlap": true,
          "icon-ignore-placement": true,
        },
      });
      m.addLayer({
        id: "searoutine",
        type: "symbol",
        source: "sea",
        filter: ["==", ["get", "notable"], 0],
        layout: {
          "icon-image": "sh-searoutine",
          "icon-size": 0.42,
          "icon-allow-overlap": true,
          "icon-ignore-placement": true,
          "visibility": "none",
        },
      });
      bindPopup(m, "searoutine", seaPopup);
      bindPopup(m, "sea", seaPopup);
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
            cssColor("--layer-sites"),
          ],
          "line-width": 2,
        },
      });
      bindPopup(m, "sites", (p) =>
        `<strong>${esc(p.label)}</strong><br/>${esc(p.country)} · ${esc(p.kind)}<br/>` +
          (p.measured
            ? `bright pixels ${(p.latest * 100).toFixed(1)}% (baseline ${(p.median * 100).toFixed(1)}%)<br/>` +
              `${p.anomaly ? "◆ change detected" : "○ nominal"}<br/>`
            : "no radar measurements yet<br/>") +
          `<a href="${esc(p.browser)}" target="_blank" rel="noopener noreferrer">inspect imagery ↗</a>`,
      );
    });

    // Incidents are the human-reported layer; they stay above every overlay
    // regardless of which effect happened to create its layers first.
    if (m.getLayer("incidents")) m.moveLayer("incidents");
  }, [layers, loaded]);

  // Visibility toggles.
  useEffect(() => {
    const m = map.current;
    if (!m || !loaded) return;
    for (const o of MAP_LAYERS) {
      // "sites" is drawn as an outline plus a fill; both follow one toggle.
      for (const id of o.key === "sites" ? ["sites", "sites-fill"] : [o.key]) {
        if (m.getLayer(id)) {
          m.setLayoutProperty(id, "visibility", visible[o.key] ? "visible" : "none");
        }
      }
    }
    if (m.getLayer("incidents")) {
      m.setLayoutProperty("incidents", "visibility", showIncidents ? "visible" : "none");
    }
  }, [visible, showIncidents, loaded, layers, incidents]);

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
                    ? "◆ change detected"
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
    // traffic kept as baseline; putting 180 in the toggle implied 180 things
    // worth looking at when seven were.
    sea: layers?.sea.filter((s) => s.notable).length ?? 0,
    searoutine: layers?.sea.filter((s) => !s.notable).length ?? 0,
    sites: layers?.sar.length ?? 0,
    cables: 0,
    territory: 0,
  };
  // Located only: an incident without a named place gets no pin, so the count
  // on the toggle is the count on the map, not the count in the feed.
  const locatedIncidents = incidents.filter(
    (i) => i.lat != null && i.lon != null,
  ).length;

  if (mapFailed) {
    return (
      <p style={{ color: "var(--text-muted)" }}>
        The interactive map could not be loaded on this device — it needs
        WebGL, which this browser has disabled or does not support. Nothing is
        lost: every located incident also appears in the incident feed below,
        and the satellite table carries the monitored sites.
      </p>
    );
  }

  return (
    <>
      <div className="filters" role="group" aria-label="Map layers" style={{ marginTop: 0, marginBottom: 10 }}>
        <button aria-pressed={showIncidents} onClick={() => setShowIncidents((v) => !v)}>
          <Swatch shape={INCIDENTS_DEF.shape} color={cssColor(INCIDENTS_DEF.cssVar)} />
          <span style={{ marginLeft: 5 }}>{INCIDENTS_DEF.label}</span>
          {` (${locatedIncidents})`}
        </button>
        {MAP_LAYERS.map((o) => (
          <button
            key={o.key}
            aria-pressed={visible[o.key]}
            onClick={() => setVisible((v) => ({ ...v, [o.key]: !v[o.key] }))}
          >
            <Swatch shape={o.shape} color={cssColor(o.cssVar)} filled={!o.hollow} />
            <span style={{ marginLeft: 5 }}>{o.label}</span>
            {o.key !== "cables" && o.key !== "territory" && ` (${counts[o.key]})`}
          </button>
        ))}
      </div>
      <div ref={el} className="map-wrap" role="region" aria-label="Incident map" />
      <div className="legend">
        {[1, 2, 3, 4, 5].map((s) => (
          <span className="key" key={s}>
            {/* Circles, matching the incident markers, and sized by severity
                the same way the map sizes them. */}
            <Swatch shape="circle" color={severityColor(s)} size={7 + s} />
            {s} · {SEVERITY_LABELS[s]}
          </span>
        ))}
        <span style={{ color: "var(--text-muted)" }}>
          incident severity — shapes, shading and each layer's limits are
          explained under “What these layers mean”
        </span>
      </div>
      <MapLegend />
    </>
  );
}

// Shared by the notable and baseline sea layers — the same kind of thing gets
// the same popup. A gap's length is the difference between a receiver dropout
// and dark activity, so for gaps the popup states it outright.
function seaPopup(p: Record<string, any>): string {
  const dark =
    p.event === "ais-gap" && p.started
      ? ` · ${((new Date(p.when).getTime() - new Date(p.started).getTime()) / 3.6e6).toFixed(1)} h dark`
      : "";
  const listed = p.sanctioned
    ? `<br/><strong>⚠ listed:</strong> ${esc(p.sanctioned)}${p.risk ? ` (${esc(p.risk)})` : ""}${p.flag ? ` · flag ${esc(p.flag)}` : ""}` +
      (p.url ? `<br/><a href="${esc(p.url)}" target="_blank" rel="noopener noreferrer">OpenSanctions entry</a>` : "")
    : "";
  return `<strong>⚓ ${esc(p.name)}</strong><br/>${esc(p.event)}${dark} · ${esc(p.corridor)} · ${p.sog ?? "?"} kn<br/>${new Date(p.when).toLocaleString("en-GB")}${listed}`;
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

// Layers with a popup bound, so overlapping layers can defer to the topmost
// one — without this, one click over stacked layers opened several popups.
const popupLayers = new Set<string>();

function bindPopup(
  m: maplibregl.Map,
  layerId: string,
  html: (props: Record<string, any>) => string,
) {
  popupLayers.add(layerId);
  m.on("click", layerId, (e) => {
    // Only the topmost popup-bearing layer under the cursor responds.
    const top = m
      .queryRenderedFeatures(e.point)
      .find((f) => popupLayers.has(f.layer.id));
    if (!top || top.layer.id !== layerId) return;
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
