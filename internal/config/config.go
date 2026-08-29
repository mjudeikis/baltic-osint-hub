package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/cluster"
)

// Config holds all runtime configuration, loaded from environment variables.
type Config struct {
	DatabaseURL   string
	OpenAIAPIKey  string
	OpenAIBaseURL string // optional; for OpenAI-compatible gateways
	// NOTE: ACLED credentials were removed deliberately, not lost. Their EULA
	// (read 2026-08-29) states that external use must be "transformative" and
	// that it is "not sufficient for Licensed Content to simply be
	// supplemented, appended, excerpted, reorganized, or made available
	// through Licensee's own dashboard" — which is exactly what this project
	// would do. Their content terms separately bar creating "a functional
	// substitute for" ACLED's platforms. So the phase-2 fetcher is cancelled;
	// link to ACLED, never ingest it. See docs/competitive-landscape.md.
	// Signal-layer credentials; each layer is skipped when its key is empty.
	FIRMSMapKey         string // NASA FIRMS (free registration)
	OpenSkyClientID     string // OpenSky OAuth2; anonymous fallback when empty
	OpenSkyClientSecret string
	AISStreamAPIKey     string // aisstream.io (free registration)
	// Copernicus Data Space OAuth client for Sentinel-1 SAR change detection.
	CopernicusClientID     string
	CopernicusClientSecret string
	ListenAddr             string
	StaticDir              string // directory with built frontend; empty disables static serving
	// EnrichModel is the OpenAI model used for classification.
	EnrichModel string
	// MaxEnrichPerRun caps LLM classification per collector run as a cost guard.
	MaxEnrichPerRun int
	// MaxClusterPerRun caps how many incidents are embedded and clustered per
	// run. Embeddings are far cheaper than classification, so this is set high
	// enough to clear the historical backlog over a few runs.
	MaxClusterPerRun int
	// ClusterThreshold is the cosine similarity at which two reports are
	// treated as the same event. Exposed so it can be retuned against live
	// data without a rebuild; see internal/cluster for why it is set high.
	ClusterThreshold float64
	// AISArchiveInterval is how often the server polls Digitraffic for vessel
	// positions to archive. 15 minutes gives roughly 3 nautical miles between
	// fixes at cruising speed, which is enough to reconstruct a track near a
	// cable; the endpoint is free, keyless and unmetered.
	AISArchiveInterval time.Duration
	// RunTimeout caps one collector run. It must exceed the time needed to
	// walk the whole SAR watchlist, which is one API round-trip per site.
	RunTimeout time.Duration
}

func Load() (*Config, error) {
	loadDotEnv(".env")
	c := &Config{
		DatabaseURL:            os.Getenv("DATABASE_URL"),
		OpenAIAPIKey:           os.Getenv("OPENAI_API_KEY"),
		OpenAIBaseURL:          os.Getenv("OPENAI_BASE_URL"),
		FIRMSMapKey:            os.Getenv("FIRMS_MAP_KEY"),
		OpenSkyClientID:        os.Getenv("OPENSKY_CLIENT_ID"),
		OpenSkyClientSecret:    os.Getenv("OPENSKY_CLIENT_SECRET"),
		AISStreamAPIKey:        os.Getenv("AISSTREAM_API_KEY"),
		CopernicusClientID:     os.Getenv("COPERNICUS_CLIENT_ID"),
		CopernicusClientSecret: os.Getenv("COPERNICUS_CLIENT_SECRET"),
		ListenAddr:             getenv("LISTEN_ADDR", ":8080"),
		StaticDir:              os.Getenv("STATIC_DIR"),
		EnrichModel:            getenv("ENRICH_MODEL", "gpt-5-mini"),
		MaxEnrichPerRun:        getenvInt("MAX_ENRICH_PER_RUN", 300),
		MaxClusterPerRun:       getenvInt("MAX_CLUSTER_PER_RUN", 1000),
		AISArchiveInterval:     time.Duration(getenvInt("AIS_ARCHIVE_MINUTES", 15)) * time.Minute,
		ClusterThreshold:       getenvFloat("CLUSTER_THRESHOLD", cluster.DefaultThreshold),
		RunTimeout:             time.Duration(getenvInt("RUN_TIMEOUT_MINUTES", 50)) * time.Minute,
	}
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
}

// loadDotEnv reads KEY=VALUE lines from path into the environment for local
// development. Real environment variables win; missing file is fine (in
// containers everything comes from the environment).
func loadDotEnv(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || value == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, value)
		}
	}
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvFloat(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
