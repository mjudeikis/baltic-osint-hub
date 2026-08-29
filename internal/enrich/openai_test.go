package enrich

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

func TestClassifyBatchAgainstMock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("auth header = %q", got)
		}
		var req chatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatal(err)
		}
		if req.Model != "gpt-5-mini" || len(req.Messages) != 2 || req.Messages[0].Role != "system" {
			t.Errorf("bad request: %+v", req)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{
					"content": `[{"id": 7, "relevant": true, "category": "gps-jamming", "countries": ["LT"], "severity": 3, "summary": "Jamming near Vilnius."}]`,
				},
				"finish_reason": "stop",
			}},
		})
	}))
	defer srv.Close()

	cls := NewClassifier("test-key", "gpt-5-mini", srv.URL)
	verdicts, err := cls.ClassifyBatch(context.Background(), []store.RawItem{
		{ID: 7, Source: "lrt-en", Title: "GPS jamming over Vilnius"},
	})
	if err != nil {
		t.Fatal(err)
	}
	v, ok := verdicts[7]
	if !ok || !v.Relevant || v.Category != "gps-jamming" || v.Severity != 3 {
		t.Fatalf("verdict = %+v", v)
	}
}

func TestOpenAIErrorSurface(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "Incorrect API key", "type": "invalid_request_error"}}`))
	}))
	defer srv.Close()

	cls := NewClassifier("bad", "gpt-5-mini", srv.URL)
	_, err := cls.ClassifyBatch(context.Background(), []store.RawItem{{ID: 1, Title: "x"}})
	if err == nil {
		t.Fatal("expected error")
	}
}
