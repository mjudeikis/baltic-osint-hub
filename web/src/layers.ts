import { Shape } from "./shapes";

// Single source of truth for map layer identity: key, label, colour slot,
// shape, and whether the mark is drawn hollow (context) or filled (finding).
// The layer toggles, the on-map icons and the "what these layers mean" legend
// all read from here, so they cannot drift apart — previously the toggles and
// the legend kept parallel lists, and the sea layer's colours had already
// diverged between them.
export interface MapLayerDef {
  key: string;
  label: string;
  cssVar: string;
  shape: Shape;
  // Hollow marks the layers drawn as outlines on the map rather than as
  // filled marks, so every key and swatch matches what is actually rendered.
  hollow: boolean;
  defaultVisible: boolean;
}

// Machine-measured layers use the muted --layer-* instrument family, never
// the categorical slots: those hues carry fixed category meanings (orange =
// sabotage, red = undersea infrastructure) in every chart and chip on the
// page, and a jamming cell wearing sabotage's orange lied to anyone who had
// learned the vocabulary. Desaturation also says "detection, not event" at a
// glance — the epistemic split the legend used to carry in prose alone.
export const MAP_LAYERS = [
  { key: "jamming", label: "GPS jamming", cssVar: "--layer-jamming", shape: "hex", hollow: false, defaultVisible: true },
  { key: "thermal", label: "Thermal (FIRMS)", cssVar: "--layer-thermal", shape: "square", hollow: false, defaultVisible: true },
  { key: "air", label: "Air activity", cssVar: "--layer-air", shape: "triangle", hollow: false, defaultVisible: true },
  // Notable sea events are deliberately drawn in the warning colour — a listed
  // vessel or an extended AIS gap is a status, not just an identity — so the
  // toggle and legend show the warning colour too.
  { key: "sea", label: "Sea activity", cssVar: "--status-warning", shape: "diamond", hollow: false, defaultVisible: true },
  // Baseline traffic is off by default: routine stops and short AIS gaps.
  // Ships stop constantly and legitimately, and short gaps are receiver
  // coverage far more often than dark activity — drawing them all buried the
  // handful of marks that mean something. Kept as an opt-in layer rather than
  // deleted, because the baseline is what makes an anomaly legible.
  { key: "searoutine", label: "Sea: baseline", cssVar: "--layer-searoutine", shape: "diamond", hollow: true, defaultVisible: false },
  { key: "sites", label: "Radar sites", cssVar: "--layer-sites", shape: "square", hollow: true, defaultVisible: true },
  { key: "cables", label: "Cables & pipelines", cssVar: "--layer-cable", shape: "line", hollow: false, defaultVisible: true },
  { key: "territory", label: "RU / BY territory", cssVar: "--layer-territory", shape: "area", hollow: false, defaultVisible: true },
] as const satisfies readonly MapLayerDef[];

export type OverlayKey = (typeof MAP_LAYERS)[number]["key"];

// The incident layer is not a toggleable overlay like the rest (it follows the
// page filters), but the legend still needs its identity from the same place.
export const INCIDENTS_DEF = {
  key: "incidents",
  label: "Incidents",
  cssVar: "--seq-3",
  shape: "circle" as Shape,
  hollow: false,
};

export const layerDef = (key: string): Pick<MapLayerDef, "label" | "cssVar" | "shape" | "hollow"> =>
  key === INCIDENTS_DEF.key
    ? INCIDENTS_DEF
    : MAP_LAYERS.find((l) => l.key === key) ?? INCIDENTS_DEF;
