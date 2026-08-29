package layers

import (
	"testing"
	"time"
)

func TestParseCertPLDate(t *testing.T) {
	want := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	for _, in := range []string{
		"2026-03-14T09:30:00Z",
		"2026-03-14T09:30:00",
		"2026-03-14 09:30:00",
		"2026-03-14",
	} {
		got, ok := parseCertPLDate(in)
		if !ok || !got.Equal(want) {
			t.Errorf("parseCertPLDate(%q) = %v ok=%v, want %v", in, got, ok, want)
		}
	}
	// An unparseable or empty value must be skipped, never bucketed into a
	// default date — that would put a false spike on whatever day that is.
	for _, in := range []string{"", "not a date", "14/03/2026"} {
		if _, ok := parseCertPLDate(in); ok {
			t.Errorf("parseCertPLDate(%q) accepted a bad value", in)
		}
	}
}

func TestCountByDay(t *testing.T) {
	added, removed := countByDay([]certPLRecord{
		{InsertDate: "2026-03-14T09:00:00Z"},
		{InsertDate: "2026-03-14T18:00:00Z"}, // same day, different hour
		{InsertDate: "2026-03-15T01:00:00Z"},
		{InsertDate: "2026-03-15T02:00:00Z", DeleteDate: "2026-03-20T00:00:00Z"},
		{InsertDate: "", DeleteDate: ""}, // junk row
	})
	d14 := time.Date(2026, 3, 14, 0, 0, 0, 0, time.UTC)
	d15 := time.Date(2026, 3, 15, 0, 0, 0, 0, time.UTC)
	d20 := time.Date(2026, 3, 20, 0, 0, 0, 0, time.UTC)

	if added[d14] != 2 {
		t.Errorf("added on 14th = %d, want 2 — same-day records must bucket together", added[d14])
	}
	if added[d15] != 2 {
		t.Errorf("added on 15th = %d, want 2", added[d15])
	}
	if removed[d20] != 1 {
		t.Errorf("removed on 20th = %d, want 1", removed[d20])
	}
	if len(added) != 2 {
		t.Errorf("added has %d days, want 2 — the junk row must not create one", len(added))
	}
}
