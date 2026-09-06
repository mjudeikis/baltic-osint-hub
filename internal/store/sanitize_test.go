package store

import (
	"strings"
	"testing"
)

// Error text on source_runs is public via /api/sources, so anything
// credential-shaped in it must be gone before it is stored.
func TestSanitizeErrorRedactsSecrets(t *testing.T) {
	cases := []struct {
		in       string
		mustLack string
		mustHave string
	}{
		{`Get "https://x.example/api?map_key=SECRET123&area=1": dial tcp: timeout`, "SECRET123", "map_key=<redacted>&area=1"},
		{`status 401 for https://x.example/v1?token=abc.def-ghi`, "abc.def-ghi", "token=<redacted>"},
		{`https://x.example/?client_secret=sekrit&api_key=k2`, "sekrit", "api_key=<redacted>"},
		{`Authorization: Bearer eyJhbGciOi.abc-DEF_123 rejected`, "eyJhbGciOi", "Bearer <redacted> rejected"},
	}
	for _, c := range cases {
		got := SanitizeError(c.in)
		if strings.Contains(got, c.mustLack) {
			t.Errorf("SanitizeError(%q) = %q still contains %q", c.in, got, c.mustLack)
		}
		if !strings.Contains(got, c.mustHave) {
			t.Errorf("SanitizeError(%q) = %q, want it to contain %q", c.in, got, c.mustHave)
		}
	}
}

func TestSanitizeErrorTruncates(t *testing.T) {
	long := strings.Repeat("ą", 1000) // multi-byte, so a byte cut could split a rune
	got := SanitizeError(long)
	if len(got) > maxStoredError+len("…") {
		t.Errorf("len = %d, want <= %d", len(got), maxStoredError+len("…"))
	}
	if !strings.HasSuffix(got, "…") {
		t.Error("truncated text should end with an ellipsis")
	}
	if got != strings.TrimSuffix(got, "…")+"…" || !strings.HasPrefix(long, strings.TrimSuffix(got, "…")) {
		t.Error("truncation split a rune")
	}
}
