package sources

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

// Fetcher retrieves recent items from one source.
type Fetcher interface {
	Name() string
	// Interval is the minimum time between fetches; the collector skips the
	// source when it ran successfully more recently than this.
	Interval() time.Duration
	Fetch(ctx context.Context) ([]store.RawItem, error)
}

// HTTPClient is shared by all fetchers: modest timeout, identifying UA.
var HTTPClient = &http.Client{
	Timeout:   30 * time.Second,
	Transport: userAgentTransport{base: http.DefaultTransport},
}

type userAgentTransport struct{ base http.RoundTripper }

func (t userAgentTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", "baltic-osint-hub/1.0 (+https://github.com/mjudeikis/baltic-osint-hub)")
	return t.base.RoundTrip(req)
}

// ContentHash produces a stable hash of the normalized title used for
// cross-source deduplication (agencies syndicate the same headline).
func ContentHash(title string) string {
	norm := strings.ToLower(strings.Join(strings.Fields(title), " "))
	sum := sha256.Sum256([]byte(norm))
	return hex.EncodeToString(sum[:])
}

// Truncate limits text to max bytes without splitting a UTF-8 rune;
// classification needs the lede, not the article.
func Truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	cut := s[:max]
	for len(cut) > 0 && !utf8.ValidString(cut) {
		cut = cut[:len(cut)-1]
	}
	return cut
}

// MaxFeedBytes caps how much of an upstream response is read. A feed is a
// few hundred kilobytes; a response that keeps going is either broken or
// hostile, and either way it must not be buffered into memory whole.
const MaxFeedBytes = 8 << 20

// LimitBody bounds an upstream body at MaxFeedBytes.
func LimitBody(r io.Reader) io.Reader { return io.LimitReader(r, MaxFeedBytes) }

// ValidLink reports whether a feed item's link is an absolute http(s) URL.
// Links are stored and later rendered as anchors on a public page, so a
// javascript: or data: link from a feed would become a clickable payload;
// anything else is dropped at ingest rather than filtered at render.
func ValidLink(link string) bool {
	u, err := url.Parse(link)
	if err != nil || u.Host == "" {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return true
	}
	return false
}
