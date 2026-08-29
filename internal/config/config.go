package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all runtime configuration, loaded from environment variables.
type Config struct {
	DatabaseURL     string
	AnthropicAPIKey string
	ACLEDAPIKey     string // optional; ACLED fetcher is skipped when empty
	ListenAddr      string
	StaticDir       string // directory with built frontend; empty disables static serving
	// EnrichModel is the Claude model used for classification.
	EnrichModel string
	// MaxEnrichPerRun caps LLM classification per collector run as a cost guard.
	MaxEnrichPerRun int
}

func Load() (*Config, error) {
	c := &Config{
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		AnthropicAPIKey: os.Getenv("ANTHROPIC_API_KEY"),
		ACLEDAPIKey:     os.Getenv("ACLED_API_KEY"),
		ListenAddr:      getenv("LISTEN_ADDR", ":8080"),
		StaticDir:       os.Getenv("STATIC_DIR"),
		EnrichModel:     getenv("ENRICH_MODEL", "claude-haiku-4-5"),
		MaxEnrichPerRun: getenvInt("MAX_ENRICH_PER_RUN", 300),
	}
	if c.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	return c, nil
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
