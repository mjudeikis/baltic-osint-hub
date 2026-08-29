// Server exposes the dashboard API and serves the built frontend.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/api"
	"github.com/mjudeikis/baltic-osint-hub/internal/config"
	"github.com/mjudeikis/baltic-osint-hub/internal/layers"
	"github.com/mjudeikis/baltic-osint-hub/internal/sources"
	"github.com/mjudeikis/baltic-osint-hub/internal/store"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("store", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// State-controlled outlets are monitored but must never move the
	// posture reading; the store filters them out of tone counts.
	store.StateControlledSources = sources.StateControlledList()

	// AIS watch is a persistent stream, so it lives in the server process
	// rather than the collector cron.
	if cfg.AISStreamAPIKey != "" {
		go (&layers.AISWatch{APIKey: cfg.AISStreamAPIKey, DB: db, Log: log}).Run(ctx)
	} else {
		log.Warn("AISSTREAM_API_KEY not set; sea-activity watch disabled")
	}

	// The AIS archive lives here too, not in the collector: the collector runs
	// hourly, and hourly fixes are too coarse to reconstruct a track — a vessel
	// at 12 knots moves 12 nautical miles between them. It needs no key.
	go runAISArchive(ctx, db, log, cfg.AISArchiveInterval)

	mux := http.NewServeMux()
	api.New(db, log).Register(mux)
	if cfg.StaticDir != "" {
		mux.Handle("/", spaHandler(cfg.StaticDir))
	}

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, c := context.WithTimeout(context.Background(), 10*time.Second)
		defer c()
		_ = srv.Shutdown(shutdownCtx)
	}()
	log.Info("listening", "addr", cfg.ListenAddr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

// runAISArchive polls Finnish Digitraffic on a ticker, archiving vessel
// positions inside the cable corridors. Failures are logged and retried on the
// next tick — a public open-data endpoint being briefly unavailable is not a
// reason to take the dashboard down.
func runAISArchive(ctx context.Context, db *store.Store, log *slog.Logger, interval time.Duration) {
	archive := &layers.AISArchive{
		Client:    &http.Client{Timeout: time.Minute},
		Retention: layers.DefaultAISRetention,
	}
	run := func() {
		started := time.Now()
		err := archive.Run(ctx, db, log)
		db.RecordSourceRun(ctx, "layer:ais-archive", started, 0, 0, err)
		if err != nil && ctx.Err() == nil {
			log.Error("ais archive", "err", err)
		}
	}
	run() // once at startup, so a restart does not leave a gap the length of the interval
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

// spaHandler serves static files, falling back to index.html for client-side
// routes (paths without a file extension).
func spaHandler(dir string) http.Handler {
	fs := http.FileServer(http.Dir(dir))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := filepath.Join(dir, filepath.Clean(r.URL.Path))
		if _, err := os.Stat(path); os.IsNotExist(err) && !strings.Contains(r.URL.Path, ".") {
			http.ServeFile(w, r, filepath.Join(dir, "index.html"))
			return
		}
		fs.ServeHTTP(w, r)
	})
}
