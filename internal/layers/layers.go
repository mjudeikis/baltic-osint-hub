// Package layers ingests machine-measured signal layers (satellite thermal
// anomalies, GPS jamming, air and sea activity) shown on the dashboard map,
// separate from the LLM-classified news incidents.
package layers

// Box is a named geographic bounding box.
type Box struct {
	Name                           string
	LatMin, LonMin, LatMax, LonMax float64
}

func (b Box) Contains(lat, lon float64) bool {
	return lat >= b.LatMin && lat <= b.LatMax && lon >= b.LonMin && lon <= b.LonMax
}

// BorderSectors are the approach sectors watched for thermal anomalies and
// air activity. They sit predominantly on RU/BY territory: activity detected
// on the NATO side of the line has already arrived, and carries no warning.
// The two NATO-side sectors are kept for a different purpose — airspace
// violations and incidents happen over friendly ground by definition.
var BorderSectors = []Box{
	// --- Adversary side: approach and staging ---
	{Name: "kaliningrad", LatMin: 54.3, LonMin: 19.6, LatMax: 55.3, LonMax: 22.9},
	{Name: "by-grodno-lida", LatMin: 53.5, LonMin: 23.8, LatMax: 54.6, LonMax: 26.3},
	{Name: "by-brest", LatMin: 51.8, LonMin: 23.6, LatMax: 52.9, LonMax: 25.9},
	{Name: "by-vitebsk", LatMin: 54.6, LonMin: 26.5, LatMax: 55.8, LonMax: 30.6},
	{Name: "ru-pskov", LatMin: 56.8, LonMin: 27.6, LatMax: 58.3, LonMax: 30.2},
	{Name: "ru-leningrad", LatMin: 58.3, LonMin: 28.0, LatMax: 59.9, LonMax: 30.6},

	// --- Friendly side: violation and incident detection, not warning ---
	{Name: "nato-border-north", LatMin: 57.4, LonMin: 26.0, LatMax: 59.7, LonMax: 28.2},
	{Name: "nato-border-south", LatMin: 53.6, LonMin: 22.6, LatMax: 56.3, LonMax: 26.9},
}

// AdversarySectors is the subset that carries early-warning value.
func AdversarySectors() []Box {
	out := make([]Box, 0, len(BorderSectors))
	for _, b := range BorderSectors {
		if b.Name != "nato-border-north" && b.Name != "nato-border-south" {
			out = append(out, b)
		}
	}
	return out
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
