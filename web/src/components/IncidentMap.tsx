import { useEffect, useRef } from "react";
import maplibregl from "maplibre-gl";
import "maplibre-gl/dist/maplibre-gl.css";
import { Incident } from "../api";
import { categoryLabel, severityColor, SEVERITY_LABELS } from "../taxonomy";

// Severity is the color channel (sequential blue ramp) — category identity
// lives in the tooltip, since 10 categorical hues can't validate all-pairs.
export default function IncidentMap({ incidents }: { incidents: Incident[] }) {
  const el = useRef<HTMLDivElement>(null);
  const map = useRef<maplibregl.Map | null>(null);
  const markers = useRef<maplibregl.Marker[]>([]);

  useEffect(() => {
    if (!el.current || map.current) return;
    map.current = new maplibregl.Map({
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
      center: [23.5, 56.5], // Baltic region
      zoom: 4.4,
      attributionControl: { compact: true },
    });
    map.current.addControl(new maplibregl.NavigationControl({ showCompass: false }));
    return () => {
      map.current?.remove();
      map.current = null;
    };
  }, []);

  useEffect(() => {
    if (!map.current) return;
    markers.current.forEach((m) => m.remove());
    markers.current = [];
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
        `<strong>${escapeHTML(inc.title)}</strong><br/>` +
          `${categoryLabel(inc.category)} · severity ${inc.severity} (${SEVERITY_LABELS[inc.severity]})<br/>` +
          `<a href="${escapeHTML(inc.url)}" target="_blank" rel="noopener noreferrer">source ↗</a>`,
      );
      markers.current.push(
        new maplibregl.Marker({ element: dot })
          .setLngLat([inc.lon, inc.lat])
          .setPopup(popup)
          .addTo(map.current),
      );
    }
  }, [incidents]);

  return (
    <>
      <div ref={el} className="map-wrap" role="region" aria-label="Incident map" />
      <div className="legend">
        {[1, 2, 3, 4, 5].map((s) => (
          <span className="key" key={s}>
            <span
              className="swatch"
              style={{ background: severityColor(s), borderRadius: "50%" }}
            />
            {s} · {SEVERITY_LABELS[s]}
          </span>
        ))}
        <span style={{ color: "var(--text-muted)" }}>
          only incidents with a known location are shown
        </span>
      </div>
    </>
  );
}

function escapeHTML(s: string): string {
  return s.replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]!,
  );
}
