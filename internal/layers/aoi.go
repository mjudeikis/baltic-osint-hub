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
		Note: "Iskander-M missile brigade garrison; the closest strike capability to Lithuania.",
		Side: SideAdversary, DepthKm: 60,
		Box: Box{Name: "chernyakhovsk", LatMin: 54.58, LonMin: 21.75, LatMax: 54.68, LonMax: 21.90},
	},
	{
		Key: "gvardeysk", Label: "Gvardeysk", Country: "RU-KGD", Kind: "garrison",
		Note: "Motor-rifle garrison and depot serving the Kaliningrad grouping.",
		Side: SideAdversary, DepthKm: 80,
		Box: Box{Name: "gvardeysk", LatMin: 54.60, LonMin: 21.00, LatMax: 54.70, LonMax: 21.15},
	},
	{
		Key: "baltiysk", Label: "Baltiysk naval base", Country: "RU-KGD", Kind: "naval",
		Note: "Baltic Fleet main base — amphibious and surface units, plus the naval air arm.",
		Side: SideAdversary, DepthKm: 110,
		Box: Box{Name: "baltiysk", LatMin: 54.62, LonMin: 19.86, LatMax: 54.70, LonMax: 19.98},
	},
	{
		Key: "gusev", Label: "Gusev garrison", Country: "RU-KGD", Kind: "garrison",
		Note: "Motor-rifle garrison ~25 km from the Lithuanian border.",
		Side: SideAdversary, DepthKm: 25,
		Box: Box{Name: "gusev", LatMin: 54.54, LonMin: 22.13, LatMax: 54.64, LonMax: 22.28},
	},

	// ---- Russian mainland: the deep early-warning set ----
	{
		// 76th Guards Air Assault Division — the single most-watched
		// formation for the north-eastern approach.
		Key: "pskov", Label: "Pskov — 76th Air Assault Div.", Country: "RU", Kind: "garrison",
		Note: "76th Guards Air Assault Division. Airborne units move first and fastest; the most-watched formation on the north-eastern approach.",
		Side: SideAdversary, DepthKm: 55,
		Box: Box{Name: "pskov", LatMin: 57.77, LonMin: 28.25, LatMax: 57.88, LonMax: 28.44},
	},
	{
		Key: "ostrov", Label: "Ostrov air base", Country: "RU", Kind: "airbase",
		Note: "Army-aviation and transport base supporting the Pskov airborne grouping.",
		Side: SideAdversary, DepthKm: 75,
		Box: Box{Name: "ostrov", LatMin: 57.27, LonMin: 28.34, LatMax: 57.37, LonMax: 28.52},
	},
	{
		Key: "luga", Label: "Luga training ground", Country: "RU", Kind: "training",
		Note: "Large training ground for the 6th Combined Arms Army — normally sparse, so concentrations stand out.",
		Side: SideAdversary, DepthKm: 130,
		Box: Box{Name: "luga", LatMin: 58.68, LonMin: 29.72, LatMax: 58.80, LonMax: 29.98},
	},

	// ---- Belarus: garrisons, training grounds and rail heads ----
	{
		Key: "lida", Label: "Lida air base & garrison", Country: "BY", Kind: "airbase",
		Note: "Air base and garrison; the closest significant Belarusian installation to the Vilnius approach.",
		Side: SideAdversary, DepthKm: 40,
		Box: Box{Name: "lida", LatMin: 53.83, LonMin: 25.20, LatMax: 53.93, LonMax: 25.40},
	},
	{
		Key: "ashmyany", Label: "Ashmyany", Country: "BY", Kind: "garrison",
		Note: "Border-district garrison directly on the Vilnius axis.",
		Side: SideAdversary, DepthKm: 25,
		Box: Box{Name: "ashmyany", LatMin: 54.37, LonMin: 25.86, LatMax: 54.47, LonMax: 26.02},
	},
	{
		Key: "baranavichy", Label: "Baranavichy air base", Country: "BY", Kind: "airbase",
		Note: "Air base and air-defence node covering central Belarus.",
		Side: SideAdversary, DepthKm: 130,
		Box: Box{Name: "baranavichy", LatMin: 53.06, LonMin: 25.96, LatMax: 53.16, LonMax: 26.12},
	},
	{
		Key: "obuz-lesnovsky", Label: "Obuz-Lesnovsky training ground", Country: "BY", Kind: "training",
		Note: "Training ground used to stage Zapad exercises — empty in peacetime, which makes it a clean indicator.",
		Side: SideAdversary, DepthKm: 140,
		Box: Box{Name: "obuz-lesnovsky", LatMin: 52.54, LonMin: 25.48, LatMax: 52.68, LonMax: 25.72},
	},
	{
		Key: "slonim", Label: "Slonim — 11th Mech Bde", Country: "BY", Kind: "garrison",
		Note: "11th Guards Mechanised Brigade garrison.",
		Side: SideAdversary, DepthKm: 110,
		Box: Box{Name: "slonim", LatMin: 53.04, LonMin: 25.24, LatMax: 53.14, LonMax: 25.40},
	},
	{
		Key: "marina-gorka", Label: "Marina Gorka — 5th Spetsnaz Bde", Country: "BY", Kind: "garrison",
		Note: "5th Spetsnaz Brigade — special-operations units deploy ahead of conventional forces.",
		Side: SideAdversary, DepthKm: 220,
		Box: Box{Name: "marina-gorka", LatMin: 53.46, LonMin: 28.07, LatMax: 53.56, LonMax: 28.25},
	},
	{
		Key: "asipovichy", Label: "Asipovichy training ground & rail", Country: "BY", Kind: "training",
		Note: "Combined training area and rail loading point; a staging site during Zapad and the 2022 buildup.",
		Side: SideAdversary, DepthKm: 250,
		Box: Box{Name: "asipovichy", LatMin: 53.25, LonMin: 28.55, LatMax: 53.38, LonMax: 28.75},
	},
	{
		Key: "luninets", Label: "Luninets air base", Country: "BY", Kind: "airbase",
		Note: "Air base in southern Belarus, used as a forward operating site in 2022.",
		Side: SideAdversary, DepthKm: 190,
		Box: Box{Name: "luninets", LatMin: 52.23, LonMin: 26.75, LatMax: 52.33, LonMax: 26.90},
	},
	{
		// Rail is how Russian and Belarusian formations actually move; a
		// marshalling yard filling up is the classic movement indicator.
		Key: "orsha-rail", Label: "Orsha rail junction", Country: "BY", Kind: "rail",
		Note: "Major rail junction. Russian and Belarusian formations move by rail, so a filling marshalling yard is a movement indicator.",
		Side: SideAdversary, DepthKm: 300,
		Box: Box{Name: "orsha-rail", LatMin: 54.46, LonMin: 30.33, LatMax: 54.57, LonMax: 30.52},
	},
	{
		Key: "brest-rail", Label: "Brest rail yard", Country: "BY", Kind: "rail",
		Note: "Break-of-gauge rail yard on the Polish border — everything moving west by rail is transloaded here.",
		Side: SideAdversary, DepthKm: 5,
		Box: Box{Name: "brest-rail", LatMin: 52.05, LonMin: 23.62, LatMax: 52.15, LonMax: 23.78},
	},

	// ---- Crossing terminals, watched on the FAR side of the line ----
	// The adversary-side apron is where vehicles queue and stage; the NATO-side
	// terminal only shows what has already arrived.
	{
		Key: "kamenny-log", Label: "Kamenny Log terminal (BY side)", Country: "BY", Kind: "crossing",
		Note: "Belarusian-side terminal of the main Vilnius–Minsk crossing; queues and staging form on this apron.",
		Side: SideAdversary, DepthKm: 5,
		Box: Box{Name: "kamenny-log", LatMin: 54.47, LonMin: 25.60, LatMax: 54.57, LonMax: 25.78},
	},
	{
		Key: "benyakoni", Label: "Benyakoni terminal (BY side)", Country: "BY", Kind: "crossing",
		Note: "Belarusian-side terminal opposite Šalčininkai.",
		Side: SideAdversary, DepthKm: 5,
		Box: Box{Name: "benyakoni", LatMin: 54.24, LonMin: 25.45, LatMax: 54.34, LonMax: 25.62},
	},
	{
		Key: "sovetsk", Label: "Sovetsk (RU side of Neman bridge)", Country: "RU-KGD", Kind: "crossing",
		Note: "Kaliningrad-side bridgehead at the Neman crossing.",
		Side: SideAdversary, DepthKm: 3,
		Box: Box{Name: "sovetsk", LatMin: 55.05, LonMin: 21.86, LatMax: 55.15, LonMax: 22.02},
	},
	{
		Key: "bagrationovsk", Label: "Bagrationovsk terminal (RU side)", Country: "RU-KGD", Kind: "crossing",
		Note: "Kaliningrad-side terminal opposite Bezledy.",
		Side: SideAdversary, DepthKm: 3,
		Box: Box{Name: "bagrationovsk", LatMin: 54.38, LonMin: 20.58, LatMax: 54.48, LonMax: 20.74},
	},
	{
		Key: "grigorovshchina", Label: "Grigorovshchina terminal (BY side)", Country: "BY", Kind: "crossing",
		Note: "Belarusian-side terminal opposite Pāternieki.",
		Side: SideAdversary, DepthKm: 5,
		Box: Box{Name: "grigorovshchina", LatMin: 55.74, LonMin: 26.87, LatMax: 55.84, LonMax: 27.04},
	},
	{
		Key: "ivangorod", Label: "Ivangorod (RU side of Narva)", Country: "RU", Kind: "crossing",
		Note: "Russian-side town and terminal across the Narva river from Estonia.",
		Side: SideAdversary, DepthKm: 2,
		Box: Box{Name: "ivangorod", LatMin: 59.34, LonMin: 28.20, LatMax: 59.44, LonMax: 28.36},
	},
	{
		Key: "pechory", Label: "Pechory / Kunichina Gora (RU side)", Country: "RU", Kind: "crossing",
		Note: "Russian-side crossing and garrison town opposite Koidula.",
		Side: SideAdversary, DepthKm: 5,
		Box: Box{Name: "pechory", LatMin: 57.86, LonMin: 27.60, LatMax: 57.96, LonMax: 27.80},
	},
	{
		Key: "nesterov", Label: "Nesterov rail crossing (RU side)", Country: "RU-KGD", Kind: "rail",
		Note: "Kaliningrad rail crossing — the rail route into Lithuania.",
		Side: SideAdversary, DepthKm: 5,
		Box: Box{Name: "nesterov", LatMin: 54.60, LonMin: 22.53, LatMax: 54.70, LonMax: 22.70},
	},

	// ---- Training ranges ("polygons") ----
	// The highest-value category for change detection: a range sits empty most
	// of the year, so anything parked on it is a clean signal. A permanently
	// occupied barracks, by contrast, has almost no baseline delta.
	{
		Key: "dretun", Label: "Dretun range", Country: "BY", Kind: "training",
		Note: "Large Belarusian range roughly 60 km from the Latvian border; used for Russian–Belarusian joint training.",
		Side: SideAdversary, DepthKm: 60,
		Box: Box{Name: "dretun", LatMin: 55.64, LonMin: 28.62, LatMax: 55.80, LonMax: 28.88},
	},
	{
		Key: "gozha", Label: "Gozhsky range (Grodno)", Country: "BY", Kind: "training",
		Note: "Range beside the Polish and Lithuanian tri-border, on the northern approach to the Suwałki gap.",
		Side: SideAdversary, DepthKm: 20,
		Box: Box{Name: "gozha", LatMin: 53.78, LonMin: 23.72, LatMax: 53.92, LonMax: 23.96},
	},
	{
		Key: "losvida", Label: "Losvida range (Vitebsk)", Country: "BY", Kind: "training",
		Note: "Range north-east of Vitebsk, on the axis between Russia and the Latvian border.",
		Side: SideAdversary, DepthKm: 150,
		Box: Box{Name: "losvida", LatMin: 55.28, LonMin: 29.94, LatMax: 55.42, LonMax: 30.18},
	},
	{
		Key: "domanovo", Label: "Domanovo / Ruzhany", Country: "BY", Kind: "training",
		Note: "Former strategic-missile site reactivated as a deployment area; watched for missile and air-defence hardware.",
		Side: SideAdversary, DepthKm: 120,
		Box: Box{Name: "domanovo", LatMin: 52.78, LonMin: 24.76, LatMax: 52.92, LonMax: 24.98},
	},
	{
		Key: "strugi-krasnye", Label: "Strugi Krasnye range & airfield", Country: "RU", Kind: "training",
		Note: "Range and airfield between Pskov and Luga, on the approach to south-eastern Estonia.",
		Side: SideAdversary, DepthKm: 110,
		Box: Box{Name: "strugi-krasnye", LatMin: 58.20, LonMin: 29.02, LatMax: 58.34, LonMax: 29.26},
	},
	{
		Key: "pravdinsk-range", Label: "Pravdinsk range (Kaliningrad)", Country: "RU-KGD", Kind: "training",
		Note: "The main manoeuvre area inside Kaliningrad oblast, where the exclave's units exercise.",
		Side: SideAdversary, DepthKm: 45,
		Box: Box{Name: "pravdinsk-range", LatMin: 54.38, LonMin: 20.94, LatMax: 54.52, LonMax: 21.16},
	},
	{
		// The Yelnya vehicle park was the clearest early public indicator of
		// the 2021 build-up, months before the invasion.
		Key: "yelnya", Label: "Yelnya storage & range", Country: "RU", Kind: "training",
		Note: "Deep staging area for the 144th Motor Rifle Division. Its filling vehicle park was the earliest widely-reported open-source indicator ahead of 2022.",
		Side: SideAdversary, DepthKm: 450,
		Box: Box{Name: "yelnya", LatMin: 54.50, LonMin: 33.05, LatMax: 54.66, LonMax: 33.30},
	},

	// ---- Additional garrisons and air bases ----
	{
		Key: "vitsebsk", Label: "Vitsebsk — 103rd Airborne Bde", Country: "BY", Kind: "garrison",
		Note: "Belarus's airborne brigade. Like their Russian counterparts, airborne units are among the first to move.",
		Side: SideAdversary, DepthKm: 150,
		Box: Box{Name: "vitsebsk", LatMin: 55.13, LonMin: 30.10, LatMax: 55.25, LonMax: 30.32},
	},
	{
		Key: "barysaw", Label: "Barysaw — 120th Mech Bde", Country: "BY", Kind: "garrison",
		Note: "One of Belarus's two main mechanised brigades, sitting on the Minsk–Moscow corridor.",
		Side: SideAdversary, DepthKm: 240,
		Box: Box{Name: "barysaw", LatMin: 54.17, LonMin: 28.40, LatMax: 54.29, LonMax: 28.62},
	},
	{
		Key: "grodno", Label: "Grodno — 6th Mech Bde", Country: "BY", Kind: "garrison",
		Note: "Mechanised brigade nearest the Polish and Lithuanian borders, directly behind the Suwałki gap.",
		Side: SideAdversary, DepthKm: 25,
		Box: Box{Name: "grodno", LatMin: 53.62, LonMin: 23.74, LatMax: 53.74, LonMax: 23.94},
	},
	{
		Key: "polatsk", Label: "Polatsk / Navapolatsk", Country: "BY", Kind: "garrison",
		Note: "Northern Belarusian garrison and refinery hub roughly 90 km from Latvia.",
		Side: SideAdversary, DepthKm: 90,
		Box: Box{Name: "polatsk", LatMin: 55.44, LonMin: 28.66, LatMax: 55.56, LonMax: 28.90},
	},
	{
		Key: "smolensk", Label: "Smolensk — 144th Motor Rifle Div.", Country: "RU", Kind: "garrison",
		Note: "Division-level formation of the Western Military District; the parent of the Yelnya staging area.",
		Side: SideAdversary, DepthKm: 400,
		Box: Box{Name: "smolensk", LatMin: 54.72, LonMin: 31.94, LatMax: 54.84, LonMax: 32.18},
	},
	{
		Key: "machulishchy", Label: "Machulishchy air base", Country: "BY", Kind: "airbase",
		Note: "Air base south of Minsk used for Russian A-50 airborne-early-warning aircraft.",
		Side: SideAdversary, DepthKm: 190,
		Box: Box{Name: "machulishchy", LatMin: 53.71, LonMin: 27.44, LatMax: 53.83, LonMax: 27.66},
	},
	{
		Key: "chkalovsk", Label: "Chkalovsk naval air base", Country: "RU-KGD", Kind: "airbase",
		Note: "Baltic Fleet naval aviation base outside Kaliningrad city.",
		Side: SideAdversary, DepthKm: 95,
		Box: Box{Name: "chkalovsk", LatMin: 54.71, LonMin: 20.30, LatMax: 54.83, LonMax: 20.50},
	},

	// ---- The one friendly-side box worth keeping ----
	{
		// Not early warning: this is the corridor NATO would have to hold, so
		// its own state is worth a baseline for comparison.
		Key: "suwalki-corridor", Label: "Suwałki gap corridor", Country: "PL/LT", Kind: "corridor",
		Note: "The land link between Poland and Lithuania that NATO would have to hold. Watched for its own baseline rather than for warning.",
		Side: SideFriendly, DepthKm: 0,
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
