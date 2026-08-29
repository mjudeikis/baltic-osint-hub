// Package layers ingests machine-measured signal layers (satellite thermal
// anomalies, GPS jamming, air and sea activity) shown on the dashboard map,
// separate from the LLM-classified news incidents.
package layers

// Box is a named geographic bounding box.
type Box struct {
	Name                   string
	LatMin, LonMin, LatMax, LonMax float64
}

func (b Box) Contains(lat, lon float64) bool {
	return lat >= b.LatMin && lat <= b.LatMax && lon >= b.LonMin && lon <= b.LonMax
}

// BorderSectors approximate the strips along the RU/BY land borders with
// LT/LV/EE/PL (plus Kaliningrad). Used to filter FIRMS thermal detections
// and to watch air activity.
var BorderSectors = []Box{
	{Name: "kaliningrad", LatMin: 54.0, LonMin: 19.0, LatMax: 55.5, LonMax: 23.6},
	{Name: "lt-by", LatMin: 53.8, LonMin: 23.4, LatMax: 56.4, LonMax: 27.0},
	{Name: "lv-by-ru", LatMin: 55.6, LonMin: 26.5, LatMax: 58.2, LonMax: 28.5},
	{Name: "ee-ru", LatMin: 57.5, LonMin: 26.8, LatMax: 59.9, LonMax: 28.6},
	{Name: "pl-by", LatMin: 50.5, LonMin: 22.8, LatMax: 54.0, LonMax: 24.2},
}

// CableCorridors approximate the Baltic undersea cable/pipeline zones for the
// AIS loitering watch.
var CableCorridors = []Box{
	{Name: "gulf-of-finland", LatMin: 59.2, LonMin: 23.5, LatMax: 60.3, LonMax: 27.8},
	{Name: "central-baltic", LatMin: 56.5, LonMin: 17.5, LatMax: 58.8, LonMax: 21.5},
	{Name: "nordbalt", LatMin: 55.2, LonMin: 16.5, LatMax: 56.6, LonMax: 21.0},
}

// Sector returns the name of the first box containing the point, or "".
func Sector(boxes []Box, lat, lon float64) string {
	for _, b := range boxes {
		if b.Contains(lat, lon) {
			return b.Name
		}
	}
	return ""
}
