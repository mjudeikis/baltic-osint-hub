package layers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The MAP_KEY travels in the URL path, and a transport error's message
// embeds the whole URL. That message is stored on source_runs and served by
// /api/sources, so the key must never survive into any error this layer
// returns.
func TestFIRMSErrorsNeverCarryTheKey(t *testing.T) {
	const key = "0123456789abcdefSECRET"
	f := &FIRMS{MapKey: key, Client: &http.Client{}}

	// Transport failure: point at a server that is already closed so Do
	// returns a *url.Error naming the full request URL.
	srv := httptest.NewServer(http.NotFoundHandler())
	srv.Close()
	old := firmsBase
	firmsBase = srv.URL
	defer func() { firmsBase = old }()

	_, err := f.fetch(context.Background())
	if err == nil {
		t.Fatal("expected a transport error")
	}
	if strings.Contains(err.Error(), key) {
		t.Errorf("error leaks the map key: %v", err)
	}
	if !strings.Contains(err.Error(), "<redacted>") {
		t.Errorf("error should show where the key was removed: %v", err)
	}

	// A non-200 must not echo the URL either.
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv2.Close()
	firmsBase = srv2.URL
	if _, err := f.fetch(context.Background()); err == nil || strings.Contains(err.Error(), key) {
		t.Errorf("status error leaks the map key or is nil: %v", err)
	}
}
