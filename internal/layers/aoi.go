package layers

import "fmt"

// Side records whose territory an AOI sits on. Early warning comes from
// watching the adversary's own ground: by the time equipment is visible at a
// NATO-side crossing the warning value is spent. Friendly-side boxes are kept
// only where the crossing terminal itself is the subject.
const (
	SideAdversary = "adversary" // RU / BY territory
	SideBorder    = "border"    // spans the line itself
	SideFriendly  = "friendly"  // NATO territory
)

// Baseline class governs how much a change at a site is worth. Derived from
// the 2021-22 Russian build-up, where object detection alone (tanks, tents,
// field hospitals) fired identically in April 2021 and February 2022 and so
// cried wolf. What discriminated was change against a site's normal state.
const (
	// ClassEmpty: ranges and rail ramps that sit unused most of the year.
	// Anything parked there is a clean signal — the highest value per image.
	ClassEmpty = "empty"
	// ClassOccupied: permanent garrisons. Presence means nothing; only a
	// change in density does, which is a far weaker delta.
	ClassOccupied = "occupied"
	// ClassHollow: garrisons that look normal but whose units are in Ukraine.
	// Refilling one produces almost no signature until the vehicle parks fill,
	// so these are the analytical trap — watched, but never trusted to be
	// quiet just because they look unchanged.
	ClassHollow = "hollow"
)

// AOI is a monitored area for SAR change detection: publicly known military
// installations, training grounds, and rail/border chokepoints. Boxes are
// deliberately coarse (roughly 5–15 km across) — the layer reports "activity
// at this site changed", not object-level detections.
type AOI struct {
	Key     string
	Label   string
	Country string
	Kind    string // garrison | airbase | naval | training | rail | crossing
	Side    string
	// Note explains what the site is and why it is watched.
	Note string
	// Class is the site's normal state, which sets how much a change is worth.
	Class string
	// Depth is the rough distance in km from the nearest NATO border, and so
	// how much warning the site buys. Deeper sites move first.
	DepthKm int
	Box     Box
}

// MonitoredAOIs — sites widely reported in open sources. Ordered roughly by
// how far back from the border they sit: the deepest give the most warning.
var MonitoredAOIs = []AOI{
	// ---- Kaliningrad garrisons (permanently forward-deployed) ----
	{
		Key: "chernyakhovsk", Label: "Chernyakhovsk", Country: "RU-KGD", Kind: "garrison",
		Note:  "Iskander-M missile brigade garrison; the closest strike capability to Lithuania.",
		Class: ClassHollow, Side: SideAdversary, DepthKm: 29,
		Box: Box{Name: "chernyakhovsk", LatMin: 54.58, LonMin: 21.75, LatMax: 54.68, LonMax: 21.90},
	},
	{
		Key: "gvardeysk", Label: "Gvardeysk", Country: "RU-KGD", Kind: "garrison",
		Note:  "Home of the 183rd Anti-Aircraft Missile Regiment. (The 7th Motor Rifle Regiment is at Kaliningrad city, not here.)",
		Class: ClassHollow, Side: SideAdversary, DepthKm: 28,
		Box: Box{Name: "gvardeysk", LatMin: 54.60, LonMin: 21.00, LatMax: 54.70, LonMax: 21.15},
	},
	{
		Key: "baltiysk", Label: "Baltiysk naval base", Country: "RU-KGD", Kind: "naval",
		Note:  "Baltic Fleet main base — amphibious and surface units, plus the naval air arm.",
		Class: ClassHollow, Side: SideAdversary, DepthKm: 23,
		Box: Box{Name: "baltiysk", LatMin: 54.62, LonMin: 19.86, LatMax: 54.70, LonMax: 19.98},
	},
	{
		Key: "gusev", Label: "Gusev garrison", Country: "RU-KGD", Kind: "garrison",
		Note:  "Motor-rifle garrison ~25 km from the Lithuanian border.",
		Class: ClassHollow, Side: SideAdversary, DepthKm: 25,
		Box: Box{Name: "gusev", LatMin: 54.54, LonMin: 22.13, LatMax: 54.64, LonMax: 22.28},
	},

	// ---- Russian mainland: the deep early-warning set ----
	{
		// 76th Guards Air Assault Division — the single most-watched
		// formation for the north-eastern approach.
		Key: "pskov", Label: "Pskov — 76th Air Assault Div.", Country: "RU", Kind: "garrison",
		Note:  "76th Guards Air Assault Division. Airborne units move first and fastest; the most-watched formation on the north-eastern approach.",
		Class: ClassHollow, Side: SideAdversary, DepthKm: 30,
		Box: Box{Name: "pskov", LatMin: 57.77, LonMin: 28.25, LatMax: 57.88, LonMax: 28.44},
	},
	{
		Key: "ostrov", Label: "Ostrov air base", Country: "RU", Kind: "airbase",
		Note:  "Army-aviation and transport base supporting the Pskov airborne grouping.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 28,
		Box: Box{Name: "ostrov", LatMin: 57.27, LonMin: 28.34, LatMax: 57.37, LonMax: 28.52},
	},
	{
		Key: "luga", Label: "Luga training ground", Country: "RU", Kind: "training",
		Note:  "Luzhsky range — the largest manoeuvre area in the north-west, with its own military rail spur. Normally empty, so a division arriving by rail is unmistakable.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 104,
		Box: Box{Name: "luga", LatMin: 58.68, LonMin: 29.72, LatMax: 58.80, LonMax: 29.98},
	},

	// ---- Belarus: garrisons, training grounds and rail heads ----
	{
		Key: "lida", Label: "Lida air base & garrison", Country: "BY", Kind: "airbase",
		Note:  "Air base and garrison; the closest significant Belarusian installation to the Vilnius approach.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 40,
		Box: Box{Name: "lida", LatMin: 53.83, LonMin: 25.20, LatMax: 53.93, LonMax: 25.40},
	},
	{
		Key: "ashmyany", Label: "Ashmyany", Country: "BY", Kind: "garrison",
		Note:  "Border-district garrison directly on the Vilnius axis.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 25,
		Box: Box{Name: "ashmyany", LatMin: 54.37, LonMin: 25.86, LatMax: 54.47, LonMax: 26.02},
	},
	{
		Key: "baranavichy", Label: "Baranavichy air base", Country: "BY", Kind: "airbase",
		Note:  "Air base and air-defence node covering central Belarus.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 130,
		Box: Box{Name: "baranavichy", LatMin: 53.06, LonMin: 25.96, LatMax: 53.16, LonMax: 26.12},
	},
	{
		Key: "obuz-lesnovsky", Label: "Obuz-Lesnovsky training ground", Country: "BY", Kind: "training",
		Note:  "Training ground used to stage Zapad exercises — empty in peacetime, which makes it a clean indicator.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 140,
		Box: Box{Name: "obuz-lesnovsky", LatMin: 52.54, LonMin: 25.48, LatMax: 52.68, LonMax: 25.72},
	},
	{
		Key: "slonim", Label: "Slonim — 11th Mech Bde", Country: "BY", Kind: "garrison",
		Note:  "11th Guards Mechanised Brigade garrison.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 110,
		Box: Box{Name: "slonim", LatMin: 53.04, LonMin: 25.24, LatMax: 53.14, LonMax: 25.40},
	},
	{
		Key: "marina-gorka", Label: "Marina Gorka — 5th Spetsnaz Bde", Country: "BY", Kind: "garrison",
		Note:  "5th Spetsnaz Brigade — special-operations units deploy ahead of conventional forces.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 176,
		Box: Box{Name: "marina-gorka", LatMin: 53.46, LonMin: 28.07, LatMax: 53.56, LonMax: 28.25},
	},
	{
		Key: "asipovichy", Label: "Asipovichy training ground & rail", Country: "BY", Kind: "training",
		Note:  "Combined training area and rail loading point; a staging site during Zapad and the 2022 buildup.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 214,
		Box: Box{Name: "asipovichy", LatMin: 53.25, LonMin: 28.55, LatMax: 53.38, LonMax: 28.75},
	},
	{
		Key: "luninets", Label: "Luninets air base", Country: "BY", Kind: "airbase",
		Note:  "Air base in southern Belarus, used as a forward operating site in 2022.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 190,
		Box: Box{Name: "luninets", LatMin: 52.23, LonMin: 26.75, LatMax: 52.33, LonMax: 26.90},
	},
	{
		// Rail is how Russian and Belarusian formations actually move; a
		// marshalling yard filling up is the classic movement indicator.
		Key: "orsha-rail", Label: "Orsha rail junction", Country: "BY", Kind: "rail",
		Note:  "Major rail junction. Russian and Belarusian formations move by rail, so a filling marshalling yard is a movement indicator.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 232,
		Box: Box{Name: "orsha-rail", LatMin: 54.46, LonMin: 30.33, LatMax: 54.57, LonMax: 30.52},
	},
	{
		Key: "brest-rail", Label: "Brest rail yard", Country: "BY", Kind: "rail",
		Note:  "Break-of-gauge rail yard on the Polish border — everything moving west by rail is transloaded here.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 5,
		Box: Box{Name: "brest-rail", LatMin: 52.05, LonMin: 23.62, LatMax: 52.15, LonMax: 23.78},
	},

	// ---- Crossing terminals, watched on the FAR side of the line ----
	// The adversary-side apron is where vehicles queue and stage; the NATO-side
	// terminal only shows what has already arrived.
	{
		Key: "kamenny-log", Label: "Kamenny Log terminal (BY side)", Country: "BY", Kind: "crossing",
		Note:  "Belarusian-side terminal of the main Vilnius–Minsk crossing; queues and staging form on this apron.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 5,
		Box: Box{Name: "kamenny-log", LatMin: 54.47, LonMin: 25.60, LatMax: 54.57, LonMax: 25.78},
	},
	{
		Key: "benyakoni", Label: "Benyakoni terminal (BY side)", Country: "BY", Kind: "crossing",
		Note:  "Belarusian-side terminal opposite Šalčininkai.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 5,
		Box: Box{Name: "benyakoni", LatMin: 54.24, LonMin: 25.45, LatMax: 54.34, LonMax: 25.62},
	},
	{
		Key: "sovetsk", Label: "Sovetsk (RU side of Neman bridge)", Country: "RU-KGD", Kind: "crossing",
		Note:  "Kaliningrad-side bridgehead at the Neman crossing.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 3,
		Box: Box{Name: "sovetsk", LatMin: 55.05, LonMin: 21.86, LatMax: 55.15, LonMax: 22.02},
	},
	{
		Key: "bagrationovsk", Label: "Bagrationovsk terminal (RU side)", Country: "RU-KGD", Kind: "crossing",
		Note:  "Kaliningrad-side terminal opposite Bezledy.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 3,
		Box: Box{Name: "bagrationovsk", LatMin: 54.38, LonMin: 20.58, LatMax: 54.48, LonMax: 20.74},
	},
	{
		Key: "grigorovshchina", Label: "Grigorovshchina terminal (BY side)", Country: "BY", Kind: "crossing",
		Note:  "Belarusian-side terminal opposite Pāternieki.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 5,
		Box: Box{Name: "grigorovshchina", LatMin: 55.74, LonMin: 26.87, LatMax: 55.84, LonMax: 27.04},
	},
	{
		Key: "ivangorod", Label: "Ivangorod (RU side of Narva)", Country: "RU", Kind: "crossing",
		Note:  "Russian-side town and terminal across the Narva river from Estonia.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 2,
		Box: Box{Name: "ivangorod", LatMin: 59.34, LonMin: 28.20, LatMax: 59.44, LonMax: 28.36},
	},
	{
		Key: "pechory", Label: "Pechory / Kunichina Gora (RU side)", Country: "RU", Kind: "crossing",
		Note:  "Russian-side crossing and garrison town opposite Koidula.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 5,
		Box: Box{Name: "pechory", LatMin: 57.86, LonMin: 27.60, LatMax: 57.96, LonMax: 27.80},
	},
	{
		Key: "nesterov", Label: "Nesterov rail crossing (RU side)", Country: "RU-KGD", Kind: "rail",
		Note:  "Kaliningrad rail crossing — the rail route into Lithuania.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 5,
		Box: Box{Name: "nesterov", LatMin: 54.60, LonMin: 22.53, LatMax: 54.70, LonMax: 22.70},
	},

	// ---- Training ranges ("polygons") ----
	// The highest-value category for change detection: a range sits empty most
	// of the year, so anything parked on it is a clean signal. A permanently
	// occupied barracks, by contrast, has almost no baseline delta.
	{
		Key: "dretun", Label: "Dretun range", Country: "BY", Kind: "training",
		Note:  "Dormant — converted to a state forestry enterprise, so its baseline reflects civilian use. Retained for reference rather than warning.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 60,
		Box: Box{Name: "dretun", LatMin: 55.64, LonMin: 28.62, LatMax: 55.80, LonMax: 28.88},
	},
	{
		Key: "gozha", Label: "Gozhsky range (Grodno)", Country: "BY", Kind: "training",
		Note:  "41 km² of normally-empty range 2 km from Lithuania and 33 km from the Suwałki gap. Used for Union Resolve 2022 but absent from the advertised Zapad lists — a site used for the real thing rather than the exercise.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 2,
		Box: Box{Name: "gozha", LatMin: 53.78, LonMin: 23.72, LatMax: 53.92, LonMax: 23.96},
	},
	{
		Key: "losvida", Label: "Losvida rail ramp (Vitebsk)", Country: "BY", Kind: "rail",
		Note:  "Rail station and loading ramp serving the 103rd Airborne Brigade's training field — a loading point, not a range.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 152,
		Box: Box{Name: "losvida", LatMin: 55.28, LonMin: 29.94, LatMax: 55.42, LonMax: 30.18},
	},
	{
		Key: "domanovo", Label: "Domanovo (174th range)", Country: "BY", Kind: "training",
		Note:  "Former strategic-missile site reactivated as a deployment area; watched for missile and air-defence hardware.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 120,
		Box: Box{Name: "domanovo", LatMin: 52.70, LonMin: 25.58, LatMax: 52.83, LonMax: 25.76},
	},
	{
		Key: "strugi-krasnye", Label: "Strugi Krasnye range & airfield", Country: "RU", Kind: "training",
		Note:  "Range and airfield between Pskov and Luga, on the approach to south-eastern Estonia.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 110,
		Box: Box{Name: "strugi-krasnye", LatMin: 58.20, LonMin: 29.02, LatMax: 58.34, LonMax: 29.26},
	},
	{
		Key: "pravdinsk-range", Label: "Pravdinsk range (Kaliningrad)", Country: "RU-KGD", Kind: "training",
		Note:  "The main manoeuvre area inside Kaliningrad oblast, where the exclave's units exercise.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 45,
		Box: Box{Name: "pravdinsk-range", LatMin: 54.38, LonMin: 20.94, LatMax: 54.52, LonMax: 21.16},
	},
	{
		// The Yelnya vehicle park was the clearest early public indicator of
		// the 2021 build-up, months before the invasion.
		Key: "yelnya", Label: "Yelnya storage & range", Country: "RU", Kind: "training",
		Note:  "Deep staging area for the 144th Motor Rifle Division. Its filling vehicle park was the earliest widely-reported open-source indicator ahead of 2022.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 450,
		Box: Box{Name: "yelnya", LatMin: 54.50, LonMin: 33.05, LatMax: 54.66, LonMax: 33.30},
	},

	// ---- Additional garrisons and air bases ----
	{
		Key: "vitsebsk", Label: "Vitsebsk — 103rd Airborne Bde", Country: "BY", Kind: "garrison",
		Note:  "Belarus's airborne brigade. Like their Russian counterparts, airborne units are among the first to move.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 168,
		Box: Box{Name: "vitsebsk", LatMin: 55.13, LonMin: 30.10, LatMax: 55.25, LonMax: 30.32},
	},
	{
		Key: "barysaw", Label: "Barysaw — 120th Mech Bde", Country: "BY", Kind: "garrison",
		Note:  "One of Belarus's two main mechanised brigades, sitting on the Minsk–Moscow corridor.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 240,
		Box: Box{Name: "barysaw", LatMin: 54.17, LonMin: 28.40, LatMax: 54.29, LonMax: 28.62},
	},
	{
		Key: "grodno", Label: "Grodno — 6th Mech Bde", Country: "BY", Kind: "garrison",
		Note:  "Mechanised brigade nearest the Polish and Lithuanian borders, directly behind the Suwałki gap.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 25,
		Box: Box{Name: "grodno", LatMin: 53.62, LonMin: 23.74, LatMax: 53.74, LonMax: 23.94},
	},
	{
		Key: "polatsk", Label: "Polatsk / Navapolatsk", Country: "BY", Kind: "garrison",
		Note:  "Northern Belarusian garrison and refinery hub roughly 90 km from Latvia.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 90,
		Box: Box{Name: "polatsk", LatMin: 55.44, LonMin: 28.66, LatMax: 55.56, LonMax: 28.90},
	},
	{
		Key: "machulishchy", Label: "Machulishchy air base", Country: "BY", Kind: "airbase",
		Note:  "Air base south of Minsk used for Russian A-50 airborne-early-warning aircraft.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 190,
		Box: Box{Name: "machulishchy", LatMin: 53.71, LonMin: 27.44, LatMax: 53.83, LonMax: 27.66},
	},
	{
		Key: "chkalovsk", Label: "Chkalovsk naval air base", Country: "RU-KGD", Kind: "airbase",
		Note:  "Baltic Fleet naval aviation base outside Kaliningrad city.",
		Class: ClassOccupied, Side: SideAdversary, DepthKm: 95,
		Box: Box{Name: "chkalovsk", LatMin: 54.71, LonMin: 20.30, LatMax: 54.83, LonMax: 20.50},
	},

	// ---- Added from the 2021-22 precedent research (docs/ukraine-2021-22.md).
	// Empty-class ranges and rail loading ramps: the classes that actually
	// discriminated, as opposed to permanently occupied garrisons.
	{
		Key: "brest-range", Label: "Brest range", Country: "BY", Kind: "training",
		Note:  "60 km² of normally-empty range 3 km from Poland. On both the Zapad-2021 and Union Resolve 2022 lists.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 3,
		Box: Box{Name: "brest-range", LatMin: 51.92, LonMin: 23.68, LatMax: 52.05, LonMax: 23.86},
	},
	{
		Key: "borisovsky", Label: "Borisovsky (227th) range", Country: "BY", Kind: "training",
		Note:  "The Zapad-2025 main event site and the largest range in Belarus at 108 km². Deliberately deep — the deepest Belarusian staging option toward Lithuania.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 146,
		Box: Box{Name: "borisovsky", LatMin: 54.14, LonMin: 28.22, LatMax: 54.28, LonMax: 28.44},
	},
	{
		Key: "vladimirsky-lager", Label: "Vladimirsky Lager — 68th Gds MRD", Country: "RU", Kind: "training",
		Note:  "The best composite indicator in the set: an empty range beside a hollow barracks with its own railway station. A division arriving by rail into an empty range next to an empty garrison is unmistakable.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 84,
		Box: Box{Name: "vladimirsky-lager", LatMin: 58.15, LonMin: 28.96, LatMax: 58.28, LonMax: 29.18},
	},
	{
		Key: "tugany", Label: "Tugany aviation range", Country: "RU", Kind: "training",
		Note:  "The largest empty military area on the Estonian approach (~160 km²) and almost never discussed in open reporting.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 32,
		Box: Box{Name: "tugany", LatMin: 59.15, LonMin: 28.56, LatMax: 59.28, LonMax: 28.80},
	},
	{
		Key: "khmelevka", Label: "Khmelevka amphibious range", Country: "RU-KGD", Kind: "training",
		Note:  "Coastal dune range used in Zapad 2017 and 2021. Amphibious rehearsal here is one of the cleanest single indicators available.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 36,
		Box: Box{Name: "khmelevka", LatMin: 54.70, LonMin: 19.90, LatMax: 54.81, LonMax: 20.06},
	},
	{
		Key: "ruzhany", Label: "Ruzhany (210th range)", Country: "BY", Kind: "training",
		Note:  "Range appearing on all four recent Zapad and Union Resolve exercise lists, 62 km from Poland.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 62,
		Box: Box{Name: "ruzhany", LatMin: 52.68, LonMin: 24.76, LatMax: 52.80, LonMax: 24.94},
	},
	{
		Key: "palonka-rail", Label: "Palonka rail ramp", Country: "BY", Kind: "rail",
		Note:  "Documented Russian unloading point serving the 230th Obuz-Lesnovsky range 7 km south. A ramp is empty by default, so a loaded one is unambiguous.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 112,
		Box: Box{Name: "palonka-rail", LatMin: 53.07, LonMin: 25.66, LatMax: 53.17, LonMax: 25.80},
	},
	{
		Key: "zaslonava-rail", Label: "Zaslonava rail ramp", Country: "BY", Kind: "rail",
		Note:  "Loading point for the 19th Mechanised Brigade and the Lepelsky range, on the axis toward Latvia.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 133,
		Box: Box{Name: "zaslonava-rail", LatMin: 54.82, LonMin: 28.85, LatMax: 54.93, LonMax: 29.01},
	},
	{
		Key: "smarhon-rail", Label: "Smarhon rail ramp", Country: "BY", Kind: "rail",
		Note:  "Rail loading point 42 km from Lithuania, on the direct approach to Vilnius.",
		Class: ClassEmpty, Side: SideAdversary, DepthKm: 42,
		Box: Box{Name: "smarhon-rail", LatMin: 54.42, LonMin: 26.29, LatMax: 54.53, LonMax: 26.45},
	},

	// ---- The one friendly-side box worth keeping ----
	{
		// Not early warning: this is the corridor NATO would have to hold, so
		// its own state is worth a baseline for comparison.
		Key: "suwalki-corridor", Label: "Suwałki gap corridor", Country: "PL/LT", Kind: "corridor",
		Note:  "The land link between Poland and Lithuania that NATO would have to hold. Watched for its own baseline rather than for warning.",
		Class: ClassOccupied, Side: SideFriendly, DepthKm: 0,
		Box: Box{Name: "suwalki-corridor", LatMin: 54.05, LonMin: 22.95, LatMax: 54.20, LonMax: 23.15},
	},
}

// Centre returns the AOI midpoint.
func (a AOI) Centre() (lat, lon float64) {
	return (a.Box.LatMin + a.Box.LatMax) / 2, (a.Box.LonMin + a.Box.LonMax) / 2
}

// BrowserURL deep-links the Copernicus Browser to this AOI so a human can
// inspect the actual imagery — an anomaly nobody can verify is useless.
func (a AOI) BrowserURL() string {
	lat, lon := a.Centre()
	return fmt.Sprintf(
		"https://browser.dataspace.copernicus.eu/?zoom=13&lat=%.4f&lng=%.4f&datasetId=S1_CDAS_IW_VVVH",
		lat, lon)
}
