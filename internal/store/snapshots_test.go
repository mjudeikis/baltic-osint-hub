package store

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPostureSnapshotRoundTrip(t *testing.T) {
	s, ctx := testStore(t)
	day := time.Date(2026, 1, 12, 0, 0, 0, 0, time.UTC)

	reading := map[string]any{"level": 3, "level_name": "Elevated", "headline": "Repeated notable adverse activity"}
	summary := []map[string]any{{"country": "LT", "category": "sabotage", "recent_adverse": 4}}
	if err := s.SavePostureSnapshot(ctx, day, 3, "Elevated", "escalating", 4, 9, 12, reading, summary); err != nil {
		t.Fatalf("save: %v", err)
	}

	got, err := s.PostureOn(ctx, day)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got == nil {
		t.Fatal("snapshot not found")
	}
	if got.LevelName != "Elevated" || got.Level != 3 || got.Trend != "escalating" {
		t.Errorf("got %+v", got)
	}
	if got.Adverse != 4 || got.Favourable != 9 || got.Neutral != 12 {
		t.Errorf("counts = %d/%d/%d, want 4/9/12", got.Adverse, got.Favourable, got.Neutral)
	}
	var back map[string]any
	if err := json.Unmarshal(got.Reading, &back); err != nil {
		t.Fatalf("reading json: %v", err)
	}
	if back["headline"] != "Repeated notable adverse activity" {
		t.Errorf("reading payload lost: %v", back)
	}

	// Re-running the collector the same day must update, not accumulate: the
	// archive holds one answer per date.
	if err := s.SavePostureSnapshot(ctx, day, 2, "Watchful", "de-escalating", 1, 9, 12, reading, summary); err != nil {
		t.Fatalf("resave: %v", err)
	}
	got, err = s.PostureOn(ctx, day)
	if err != nil || got == nil {
		t.Fatalf("reread: %v", err)
	}
	if got.LevelName != "Watchful" || got.Level != 2 {
		t.Errorf("second save did not replace: %+v", got)
	}
	hist, err := s.PostureHistory(ctx, 3650)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(hist) != 1 {
		t.Errorf("history has %d rows, want 1 per day", len(hist))
	}
}

// "No record of that day" must be distinguishable from "that day was calm".
func TestPostureOnMissingDayIsNil(t *testing.T) {
	s, ctx := testStore(t)
	got, err := s.PostureOn(ctx, time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for an unrecorded day, got %+v", got)
	}
}
