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
	// ACLED authenticates with myACLED account credentials (OAuth token
	// exchange); the fetcher is skipped when either is empty.
	ACLEDEmail    string
	ACLEDPassword string
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
		ACLEDEmail:             os.Getenv("ACLED_EMAIL"),
		ACLEDPassword:          os.Getenv("ACLED_PASSWORD"),
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
