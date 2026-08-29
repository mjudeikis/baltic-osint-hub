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
