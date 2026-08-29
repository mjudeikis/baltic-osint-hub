import { Posture } from "../api";

// Official civil-preparedness guidance for each monitored country. The point
// of the dashboard is to inform so people can prepare; without this it reports
// a threat and leaves the reader with nowhere to go.
const GUIDES = [
  {
    country: "Lithuania",
    name: "LT72 — 72-hour preparedness guide",
    url: "https://lt72.lt",
    body: "What to keep at home, how to act in an emergency, and where to shelter.",
  },
  {
    country: "Latvia",
    name: "72 stundas",
    url: "https://www.sargs.lv/lv/72-stundas",
    body: "State preparedness guidance for surviving the first 72 hours unaided.",
  },
  {
    country: "Estonia",
    name: "kriis.ee — Be prepared",
    url: "https://www.kriis.ee/en",
    body: "Official crisis guidance, alerts and the 'Ole valmis!' preparedness app.",
  },
  {
    country: "Poland",
    name: "RCB — Government Centre for Security",
    url: "https://www.gov.pl/web/rcb",
    body: "National alerts and civil-protection guidance.",
  },
];

export default function Preparedness({ posture }: { posture: Posture | null }) {
  // Above the ordinary background, lead with the practical step rather than
  // burying it under the reading.
  const elevated = (posture?.level ?? 0) >= 3;

  return (
    <>
      <p className="prep-intro">
        {elevated
          ? "The regional reading is above its ordinary background. Being prepared is routine civil-protection advice in this region, not a reaction to any single event:"
          : "Preparedness in the Baltic region is ordinary civic advice, independent of any current reading. Each country publishes official guidance:"}
      </p>

      <div className="board">
        {GUIDES.map((g) => (
          <div className="tile" key={g.country}>
            <div className="country">
              <span>{g.country}</span>
            </div>
            <p className="site-note" style={{ marginTop: 6 }}>
              {g.body}
            </p>
            <div className="site-actions">
              <a href={g.url} target="_blank" rel="noopener noreferrer">
                {g.name} ↗
              </a>
            </div>
          </div>
        ))}
      </div>

      <p className="legend" style={{ color: "var(--text-muted)", display: "block" }}>
        This dashboard is an open-source monitor, not an alerting system and not an
        official assessment. For warnings that require action, follow your national
        authority and its emergency-alert channel.
      </p>
    </>
  );
}
