package layers

import "fmt"

// AOI is a monitored area for SAR change detection: publicly known military
// installations, training grounds, and rail/border chokepoints near the
// monitored countries. Boxes are deliberately coarse (roughly 5–12 km across)
// — the layer reports "activity at this site changed", not object-level
// detections.
type AOI struct {
	Key     string
	Label   string
	Country string // country the site sits in
	Kind    string // garrison | airbase | naval | training | rail | border
	Box     Box
}

// MonitoredAOIs — sites widely reported in open sources. Coordinates are
// approximate centres expanded into boxes.
var MonitoredAOIs = []AOI{
	{
		Key: "chernyakhovsk", Label: "Chernyakhovsk", Country: "RU-KGD", Kind: "garrison",
		Box: Box{Name: "chernyakhovsk", LatMin: 54.58, LonMin: 21.75, LatMax: 54.68, LonMax: 21.90},
	},
	{
		Key: "gvardeysk", Label: "Gvardeysk", Country: "RU-KGD", Kind: "garrison",
		Box: Box{Name: "gvardeysk", LatMin: 54.60, LonMin: 21.00, LatMax: 54.70, LonMax: 21.15},
	},
	{
		Key: "baltiysk", Label: "Baltiysk naval base", Country: "RU-KGD", Kind: "naval",
		Box: Box{Name: "baltiysk", LatMin: 54.62, LonMin: 19.86, LatMax: 54.70, LonMax: 19.98},
	},
	{
		Key: "kybartai-nesterov", Label: "Nesterov–Kybartai rail crossing", Country: "RU-KGD", Kind: "rail",
		Box: Box{Name: "kybartai-nesterov", LatMin: 54.60, LonMin: 22.53, LatMax: 54.68, LonMax: 22.68},
	},
	{
		Key: "asipovichy", Label: "Asipovichy training ground", Country: "BY", Kind: "training",
		Box: Box{Name: "asipovichy", LatMin: 53.25, LonMin: 28.55, LatMax: 53.38, LonMax: 28.75},
	},
	{
		Key: "baranavichy", Label: "Baranavichy air base", Country: "BY", Kind: "airbase",
		Box: Box{Name: "baranavichy", LatMin: 53.06, LonMin: 25.96, LatMax: 53.16, LonMax: 26.12},
	},
	{
		Key: "luninets", Label: "Luninets air base", Country: "BY", Kind: "airbase",
		Box: Box{Name: "luninets", LatMin: 52.23, LonMin: 26.75, LatMax: 52.33, LonMax: 26.90},
	},
	{
		Key: "brest-rail", Label: "Brest rail yard", Country: "BY", Kind: "rail",
		Box: Box{Name: "brest-rail", LatMin: 52.05, LonMin: 23.62, LatMax: 52.15, LonMax: 23.78},
	},
	{
		Key: "bruzgi-kuznica", Label: "Bruzgi–Kuźnica border crossing", Country: "BY", Kind: "border",
		Box: Box{Name: "bruzgi-kuznica", LatMin: 53.47, LonMin: 23.60, LatMax: 53.57, LonMax: 23.76},
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
