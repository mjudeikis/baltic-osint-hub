package layers

import (
	"strings"
	"testing"
)

func TestParseGpsjam(t *testing.T) {
	csv := "hex,count_good_aircraft,count_bad_aircraft\n" +
		"84004bdffffffff,1,0\n" +
		"8401227ffffffff,4,2\n" +
		"8401233ffffffff,0,5\n"
	cells, err := parseGpsjam(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(cells) != 2 { // all-good cell dropped
		t.Fatalf("got %d cells", len(cells))
	}
	if cells[0].Hex != "8401227ffffffff" || cells[0].Bad != 2 || cells[0].Good != 4 {
		t.Errorf("cell 0: %+v", cells[0])
	}
}

func TestParseFIRMS(t *testing.T) {
	csv := "latitude,longitude,bright_ti4,scan,track,acq_date,acq_time,satellite,instrument,confidence,version,bright_ti5,frp,daynight\n" +
		"54.85,22.90,340.5,0.5,0.5,2026-08-28,0112,N,VIIRS,n,2.0NRT,290.1,5.2,N\n" +
		"bad,row\n"
	dets, err := parseFIRMS(strings.NewReader(csv))
	if err != nil {
		t.Fatal(err)
	}
	if len(dets) != 1 {
		t.Fatalf("got %d detections", len(dets))
	}
	d := dets[0]
	if d.Lat != 54.85 || d.FRP != 5.2 || d.DetectedAt.Hour() != 1 || d.DetectedAt.Minute() != 12 {
		t.Errorf("detection: %+v", d)
	}
	// 54.85,22.90 is inside the kaliningrad sector.
	if got := Sector(BorderSectors, d.Lat, d.Lon); got != "kaliningrad" {
		t.Errorf("sector = %q", got)
	}
}

func TestParseStatesAndNotable(t *testing.T) {
	data := []byte(`{"time": 1, "states": [
		["4b1234","RFF1234 ","Russia",1,1,21.5,54.8,10000.5,false,220.0,90.0,0.0,null,10100.0,"1200",false,0],
		["502d24","BTI611  ","Latvia",1,1,23.2,55.4,10972.8,false,203.0,187.8,5.8,null,10005.0,"7700",false,0],
		["aabbcc","LOT45   ","Poland",1,1,22.0,54.5,9000.0,false,200.0,90.0,0.0,null,9100.0,"1200",false,0]
	]}`)
	states, err := parseStates(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(states) != 3 {
		t.Fatalf("got %d states", len(states))
	}
	if r := notable(states[0]); r != "watchlist-callsign" {
		t.Errorf("state 0 reason = %q", r)
	}
	if r := notable(states[1]); r != "emergency-squawk" {
		t.Errorf("state 1 reason = %q", r)
	}
	if r := notable(states[2]); r != "" {
		t.Errorf("state 2 reason = %q", r)
	}
}

func TestSectorMiss(t *testing.T) {
	if got := Sector(BorderSectors, 52.2, 21.0); got != "" { // Warsaw, not a border strip
		t.Errorf("sector = %q", got)
	}
}
