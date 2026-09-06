package enrich

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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

// A verdict for an id that was never sent is untrusted output: item text can
// try to steer the model toward other items, and the only ids it may speak
// to are the ones in the batch.
func TestClassifyBatchIgnoresUnknownIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message": map[string]any{"content": `[
				  {"id": 7, "relevant": true, "category": "cyber", "countries": ["EE"], "severity": 2, "summary": "a"},
				  {"id": 4123, "relevant": false}]`},
				"finish_reason": "stop",
			}},
		})
	}))
	defer srv.Close()
	cls := NewClassifier("k", "gpt-5-mini", srv.URL)
	verdicts, err := cls.ClassifyBatch(context.Background(), []store.RawItem{{ID: 7, Title: "x"}})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := verdicts[4123]; ok {
		t.Error("verdict for an id outside the batch must be dropped")
	}
	if _, ok := verdicts[7]; !ok {
		t.Error("verdict for the batch's own id must be kept")
	}
}

// Rate limits and server errors are retried with backoff; a hard 4xx is not.
func TestOpenAIRetriesOn429ThenSucceeds(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		if n < 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error": {"message": "slow down", "type": "rate_limit"}}`))
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message":       map[string]any{"content": `[{"id": 1, "relevant": false}]`},
				"finish_reason": "stop",
			}},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 5},
		})
	}))
	defer srv.Close()
	cls := NewClassifier("k", "gpt-5-mini", srv.URL)
	cls.client.retryBase = time.Millisecond
	if _, err := cls.ClassifyBatch(context.Background(), []store.RawItem{{ID: 1, Title: "x"}}); err != nil {
		t.Fatalf("expected success after retries, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("calls = %d, want 3 (two 429s then success)", got)
	}
}

func TestOpenAIGivesUpAfterMaxTries(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	cls := NewClassifier("k", "gpt-5-mini", srv.URL)
	cls.client.retryBase = time.Millisecond
	if _, err := cls.ClassifyBatch(context.Background(), []store.RawItem{{ID: 1, Title: "x"}}); err == nil {
		t.Fatal("expected error after exhausting retries")
	}
	if got := atomic.LoadInt32(&calls); got != maxTries {
		t.Errorf("calls = %d, want %d", got, maxTries)
	}
}

func TestOpenAIDoesNotRetryHard4xx(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": {"message": "bad key", "type": "invalid_request_error"}}`))
	}))
	defer srv.Close()
	cls := NewClassifier("k", "gpt-5-mini", srv.URL)
	cls.client.retryBase = time.Millisecond
	if _, err := cls.ClassifyBatch(context.Background(), []store.RawItem{{ID: 1, Title: "x"}}); err == nil {
		t.Fatal("expected error")
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("calls = %d, want 1 — a 401 must not be retried", got)
	}
}

// A reply cut off by the token cap is a distinct failure: the caller must
// split the batch, not resend it.
func TestOpenAITruncatedReply(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{{
				"message":       map[string]any{"content": `[{"id": 1, "relevant": true, "cat`},
				"finish_reason": "length",
			}},
		})
	}))
	defer srv.Close()
	cls := NewClassifier("k", "gpt-5-mini", srv.URL)
	_, err := cls.ClassifyBatch(context.Background(), []store.RawItem{{ID: 1, Title: "x"}})
	if !errors.Is(err, ErrTruncated) {
		t.Fatalf("err = %v, want ErrTruncated", err)
	}
}
