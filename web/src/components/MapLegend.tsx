import { cssColor } from "../taxonomy";
import { Swatch } from "../shapes";
import { layerDef } from "../layers";

// What each map layer is, why it is worth watching, and — the part that
// matters most on a threat dashboard — what it does NOT mean.
//
// Every layer here is a machine measurement. Left unexplained, a scatter of
// dots invites the reader to supply their own story, and the story people
// supply for an unexplained dot on a threat map is worse than the truth. Each
// entry therefore carries an explicit limit alongside its rationale.
//
// Identity (label, colour, shape) comes from ../layers, shared with the map
// itself; only the prose lives here.
const LAYERS: {
  key: string;
  what: string;
  why: string;
  limit: string;
}[] = [
  {
    key: "incidents",
    what: "Reported events, classified from news and institutional sources. Colour is severity 1–5; the pin sits where the report named a place.",
    why: "This is the human-reported layer — arrests, sabotage, airspace violations, cyberattacks. Everything else on this map is a machine measurement that has not been confirmed by anyone.",
    limit: "Classification is automated and will contain errors. An event with no named location gets no pin rather than a guess, so the map under-represents what the feed contains.",
  },
  {
    key: "jamming",
    what: "Daily cells from gpsjam.org, shaded by the share of aircraft in that cell reporting degraded navigation accuracy. Derived from ADS-B, published a day or two behind.",
    why: "GNSS interference is the most persistent and most measurable hybrid activity in the region — it runs most days rather than in episodes, because it is cheap and deniable. It degrades civil aviation approaches and ship navigation for everyone in range, not only the intended target.",
    limit: "This shows where aircraft experienced interference, not where the transmitter is. Cells are roughly 22 km across, too coarse to attribute inside a country, and coverage follows air traffic — quiet skies produce quiet cells regardless of what is transmitting.",
  },
  {
    key: "thermal",
    what: "NASA VIIRS satellite heat detections, kept only where they fall inside the monitored border sectors.",
    why: "Sustained burning shows up at training grounds and staging areas, and appears in the imagery record before anything is reported.",
    limit: "This is the noisiest layer by far. The overwhelming majority of detections are agricultural burning and forest fires. Treat it as context for the other layers, never as an indicator on its own.",
  },
  {
    key: "air",
    what: "Snapshots of eight border sectors from OpenSky. A flight is kept only if it meets one of three tests: a watchlist callsign (RFF Russian Air Force, RSD Rossiya special flight detachment, NATO mission callsigns), an emergency squawk (7500 hijack, 7600 radio failure, 7700 general emergency), or a Russian- or Belarusian-registered aircraft airborne inside a sector.",
    why: "EU airspace is closed to Russian and Belarusian aircraft, so one inside these sectors is anomalous by default. RSD flights track senior government movement, NATO callsigns show the allied response, and emergency squawks mark aircraft in genuine trouble. Together these are the leading edge of an airspace incident.",
    limit: "Military aircraft routinely fly with transponders off — precisely when they matter most — and ADS-B ground coverage is uneven. An empty sector means nothing was broadcasting, not that nothing was flying. Absence here is never evidence.",
  },
  {
    key: "sea",
    what: "Live AIS inside the Baltic cable corridors. A solid diamond is a notable event — a vessel on the sanctions watchlist, or a cargo vessel, tanker or untyped vessel dark for 4+ hours. A hollow diamond is baseline traffic: routine stops and short AIS gaps.",
    why: "The Baltic's cables and pipelines are the region's most exposed infrastructure, and the 2023–25 incidents followed one pattern: a vessel stopping or slowing over a cable and dragging an anchor. A listed shadow-fleet tanker doing that is the specific thing worth seeing.",
    limit: "Ships stop constantly and legitimately, and most short AIS gaps are receiver coverage rather than a transponder switched off — the median gap here is under two hours. Both stay hollow baseline rather than findings. Vessels at anchor and service craft (pilots, tugs, SAR) are excluded entirely. AIS can also be spoofed, so a clean corridor is not a safe one.",
  },
  {
    key: "sites",
    what: "Monitored locations on the adversary side — Kaliningrad garrisons, Belarusian air bases and training grounds, rail and border chokepoints. Outlined always; filled when the newest Sentinel-1 radar pass departs from that site's own baseline.",
    why: "Radar sees through cloud and darkness, so a site can be checked on a fixed schedule regardless of weather. It measures how much metal is sitting there — vehicles, aircraft, rolling stock — against what is normal for that site.",
    limit: "This detects that something changed, not what changed. Weather, farm machinery and construction move the same number. Every site links to the underlying imagery so a human can look; the dashboard suppresses the comparison entirely when conditions differ too much from the reference period.",
  },
  {
    key: "cables",
    what: "The routes of Baltic undersea cables and pipelines.",
    why: "Context, not a detection. Without it a vessel stopped in open water means nothing — the whole question is whether it stopped over something.",
    limit: "Approximate published routes. Exact positions are not public, and cables are not always where charts suggest.",
  },
  {
    key: "territory",
    what: "Shading for Russian and Belarusian territory, including Kaliningrad.",
    why: "Context. Monitoring deliberately looks outward, at the adversary side of the border, so it matters at a glance which side of a line a detection sits on.",
    limit: "A border, not a judgement about anything inside it.",
  },
];

export default function MapLegend() {
  return (
    <details className="map-legend">
      <summary>What these layers mean</summary>
      <p className="map-legend-lead">
        <strong>Shape shows which layer a mark belongs to</strong>, so layers
        stay apart without relying on colour: ● incident, ▲ air, ◆ sea,
        ■ thermal, ⬢ jamming cell. A <strong>solid</strong> mark is worth attention; a{" "}
        <strong>hollow</strong> one is context.
      </p>
      <p className="map-legend-lead">
        Everything except <strong>Incidents</strong> is a machine measurement —
        a detection, not a verified event. Machine layers share a deliberately
        muted palette; the saturated colours belong to reported incidents and
        the charts. Each entry says what it is worth, and what it cannot tell
        you.
      </p>
      <dl>
        {LAYERS.map((l) => {
          const def = layerDef(l.key);
          return (
          <div className="map-legend-row" key={l.key}>
            <dt>
              <Swatch shape={def.shape} color={cssColor(def.cssVar)} filled={!def.hollow} />
              {def.label}
            </dt>
            <dd>
              <p>{l.what}</p>
              <p>
                <strong>Why it matters.</strong> {l.why}
              </p>
              <p className="map-legend-limit">
                <strong>What it does not mean.</strong> {l.limit}
              </p>
            </dd>
          </div>
          );
        })}
      </dl>
    </details>
  );
}
