package layers

import (
	"strings"
	"testing"
)

const maritimeHeader = "type,caption,imo,risk,countries,flag,mmsi,id,url,datasets,aliases\n"

func TestParseMaritimeCSV(t *testing.T) {
	csv := maritimeHeader +
		// Ordinary sanctioned vessel.
		"VESSEL,TIGER,IMO9307841,sanction,lr,lr,636092140,ent-1,https://os/1,ext_a,\n" +
		// Shadow-fleet tagged — the designation the sea layer most cares about.
		"VESSEL,DARK STAR,IMO1234567,mare.shadow;poi,ru,pa,273380040,ent-2,https://os/2,ext_b,\n" +
		// Multi-valued MMSI: reflagging is the shadow-fleet pattern, so every
		// identifier must match.
		"VESSEL,GUNMETAL JAG,IMO7654321,sanction,ru,ru,311001724;636024321,ent-3,https://os/3,ext_c,\n" +
		// IMO-only: real listing, but AIS broadcasts MMSI so it cannot match.
		"VESSEL,GHOST,IMO1111111,sanction,ru,ru,,ent-4,https://os/4,ext_d,\n" +
		// Not a vessel.
		"ORGANIZATION,SOME SHIPPING LLC,,sanction,ru,,,ent-5,https://os/5,ext_e,\n" +
		// Junk MMSI must be dropped, not stored as 0.
		"VESSEL,BROKEN,IMO2222222,sanction,ru,ru,not-a-number,ent-6,https://os/6,ext_f,\n"

	vessels, rows, skipped, err := parseMaritimeCSV(strings.NewReader(csv))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if rows != 6 {
		t.Errorf("rows = %d, want 6", rows)
	}
	if skipped != 1 {
		t.Errorf("skipped (IMO-only) = %d, want 1", skipped)
	}
	// 1 + 1 + 2 (multi-MMSI) = 4; the org, the IMO-only and the junk are out.
	if len(vessels) != 4 {
		t.Fatalf("vessels = %d, want 4: %+v", len(vessels), vessels)
	}

	byMMSI := map[int64]string{}
	for _, v := range vessels {
		byMMSI[v.MMSI] = v.Name
	}
	for _, want := range []int64{636092140, 273380040, 311001724, 636024321} {
		if _, ok := byMMSI[want]; !ok {
			t.Errorf("MMSI %d missing from parse", want)
		}
	}
	if byMMSI[311001724] != "GUNMETAL JAG" || byMMSI[636024321] != "GUNMETAL JAG" {
		t.Error("both MMSIs of a multi-identifier vessel must carry its name")
	}

	var shadow bool
	for _, v := range vessels {
		if v.MMSI == 273380040 {
			shadow = strings.Contains(v.Risk, "mare.shadow")
			if v.URL != "https://os/2" {
				t.Errorf("URL = %q, want the entity page so a claim links to its evidence", v.URL)
			}
		}
	}
	if !shadow {
		t.Error("shadow-fleet risk tag was not preserved")
	}
}

// A renamed upstream column must fail loudly. Silently importing nothing would
// empty the watchlist, and an empty watchlist reads as "no vessel is
// sanctioned" — the most dangerous way for this to break.
func TestParseMaritimeCSVRejectsRenamedColumn(t *testing.T) {
	csv := "type,caption,imo,risk,countries,flag,mmsi_number,id,url,datasets,aliases\n" +
		"VESSEL,TIGER,IMO9307841,sanction,lr,lr,636092140,ent-1,https://os/1,ext_a,\n"
	if _, _, _, err := parseMaritimeCSV(strings.NewReader(csv)); err == nil {
		t.Fatal("expected an error when the mmsi column is renamed")
	}
}

func TestParseMaritimeCSVEmptyIsNotAnError(t *testing.T) {
	// No vessels is a valid parse; Run decides that it is unacceptable.
	vessels, rows, _, err := parseMaritimeCSV(strings.NewReader(maritimeHeader))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(vessels) != 0 || rows != 0 {
		t.Errorf("vessels=%d rows=%d, want 0/0", len(vessels), rows)
	}
}
