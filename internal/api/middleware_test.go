package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiterThrottlesPerClient(t *testing.T) {
	l := &RateLimiter{rps: 1, burst: 2, seen: map[string]*client{}}
	h := l.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) }))
	get := func(ip string) int {
		req := httptest.NewRequest("GET", "/api/incidents", nil)
		req.Header.Set("CF-Connecting-IP", ip)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code == http.StatusTooManyRequests && rec.Header().Get("Access-Control-Allow-Origin") != "*" {
			t.Error("429 must carry the CORS header")
		}
		return rec.Code
	}
	for i := 0; i < 2; i++ {
		if get("1.1.1.1") != 200 {
			t.Fatalf("request %d of a burst of 2 should be allowed", i+1)
		}
	}
	if get("1.1.1.1") != http.StatusTooManyRequests {
		t.Error("third request within the burst window should be throttled")
	}
	if get("2.2.2.2") != 200 {
		t.Error("another client must have its own bucket")
	}
}

func TestRateLimiterEvictsIdle(t *testing.T) {
	l := &RateLimiter{rps: 1, burst: 1, seen: map[string]*client{}}
	l.allow("a")
	l.seen["a"].last = time.Now().Add(-time.Hour)
	l.allow("b")
	l.evict(time.Now().Add(-time.Minute))
	if _, ok := l.seen["a"]; ok {
		t.Error("idle client should be evicted")
	}
	if _, ok := l.seen["b"]; !ok {
		t.Error("active client should be kept")
	}
}

func TestClientIPPrefersEdgeHeaders(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	if got := ClientIP(req); got != "10.0.0.1" {
		t.Errorf("no headers: %q", got)
	}
	req.Header.Set("X-Forwarded-For", " 203.0.113.9 , 10.0.0.1")
	if got := ClientIP(req); got != "203.0.113.9" {
		t.Errorf("xff: %q", got)
	}
	req.Header.Set("CF-Connecting-IP", "198.51.100.7")
	if got := ClientIP(req); got != "198.51.100.7" {
		t.Errorf("cf: %q", got)
	}
}

func TestWithTimeoutCancelsContext(t *testing.T) {
	done := make(chan bool, 1)
	h := WithTimeout(10*time.Millisecond, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		done <- true
	}))
	rec := httptest.NewRecorder()
	go h.ServeHTTP(rec, httptest.NewRequest("GET", "/api/x", nil))
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("request context was not cancelled by the timeout")
	}
}
