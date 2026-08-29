package layers

import (
	"math"
	"strings"
	"testing"
)

func TestParseStatistics(t *testing.T) {
	// Shape of a Sentinel Hub Statistical API response: one good interval,
	// one the service could not compute, one where the pass barely clipped
	// the AOI (mostly no-data).
	body := []byte(`{
	  "data": [
	    {
	      "interval": {"from": "2026-08-01T00:00:00Z", "to": "2026-08-07T00:00:00Z"},
	      "outputs": {
	        "bright": {"bands": {"B0": {"stats": {"mean": 0.0431, "sampleCount": 250000, "noDataCount": 0}}}},
	        "vvdb":   {"bands": {"B0": {"stats": {"mean": -12.5, "sampleCount": 250000, "noDataCount": 0}}}}
	      }
	    },
	    {
	      "interval": {"from": "2026-08-07T00:00:00Z", "to": "2026-08-13T00:00:00Z"},
	      "error": {"type": "BAD_REQUEST"}
	    },
	    {
	      "interval": {"from": "2026-08-13T00:00:00Z", "to": "2026-08-19T00:00:00Z"},
	      "outputs": {
	        "bright": {"bands": {"B0": {"stats": {"mean": 0.02, "sampleCount": 250000, "noDataCount": 240000}}}}
	      }
	    }
	  ],
	  "status": "OK"
	}`)
	obs, err := parseStatistics(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 {
		t.Fatalf("got %d observations, want 1 (error and mostly-nodata intervals dropped)", len(obs))
	}
	o := obs[0]
	if o.BrightFraction != 0.0431 {
		t.Errorf("bright fraction = %v", o.BrightFraction)
	}
	if o.MeanDB != -12.5 {
		t.Errorf("mean dB = %v", o.MeanDB)
	}
	if o.SampleCount != 250000 {
		t.Errorf("sample count = %d", o.SampleCount)
	}
	if o.Start.Day() != 1 || o.Start.Month() != 8 {
		t.Errorf("interval start = %v", o.Start)
	}
}

func TestParseStatisticsEmpty(t *testing.T) {
	obs, err := parseStatistics([]byte(`{"data": [], "status": "OK"}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 0 {
		t.Errorf("got %d observations", len(obs))
	}
}

func TestEvalscriptShape(t *testing.T) {
	// The Statistical API rejects evalscripts without a dataMask output, and
	// the parser keys on these output ids.
	for _, want := range []string{"dataMask", `id: "bright"`, `id: "vvdb"`, "-5.0"} {
		if !strings.Contains(evalscript, want) {
			t.Errorf("evalscript missing %q", want)
		}
	}
}

func TestResolutionDegrees(t *testing.T) {
	// Bounds are sent as EPSG:4326, so resx/resy must be degrees. Sending
	// metres made the service read "20" as 20 degrees and collapse each AOI
	// to a single pixel (the 400 "11122 m/px exceeds 1500 m/px" error).
	resx, resy := resolutionDegrees(20, 54.63)
	if resy < 0.00017 || resy > 0.00019 {
		t.Errorf("resy = %v, want ~0.00018 deg (20 m of latitude)", resy)
	}
	// Longitude degrees are shorter at latitude, so the x step must be larger.
	if resx <= resy {
		t.Errorf("resx (%v) must exceed resy (%v) away from the equator", resx, resy)
	}
	// Sanity: the whole AOI must span many pixels, not one.
	if px := 0.10 / resy; px < 400 {
		t.Errorf("a 0.10 deg tall AOI spans only %.0f pixels", px)
	}
}

func TestResolutionDegreesGroundSquare(t *testing.T) {
	// Both axes should describe the same ground distance.
	const lat = 54.63
	resx, resy := resolutionDegrees(20, lat)
	groundX := resx * metresPerDegreeLat * math.Cos(lat*math.Pi/180)
	groundY := resy * metresPerDegreeLat
	if math.Abs(groundX-groundY) > 0.01 {
		t.Errorf("pixels not square on the ground: %.3f m x %.3f m", groundX, groundY)
	}
	if math.Abs(groundY-20) > 0.01 {
		t.Errorf("ground resolution = %.3f m, want 20", groundY)
	}
}

func TestAOIsWellFormed(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range MonitoredAOIs {
		if seen[a.Key] {
			t.Errorf("duplicate AOI key %q", a.Key)
		}
		seen[a.Key] = true
		if a.Box.LatMin >= a.Box.LatMax || a.Box.LonMin >= a.Box.LonMax {
			t.Errorf("%s: degenerate box %+v", a.Key, a.Box)
		}
		lat, lon := a.Centre()
		if !a.Box.Contains(lat, lon) {
			t.Errorf("%s: centre outside its own box", a.Key)
		}
		if !strings.Contains(a.BrowserURL(), "browser.dataspace.copernicus.eu") {
			t.Errorf("%s: bad browser URL %q", a.Key, a.BrowserURL())
		}
	}
}
