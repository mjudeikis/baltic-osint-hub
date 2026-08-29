// Collector runs one fetch+classify cycle and exits. It is designed to run
// as a Kubernetes CronJob (every 30 minutes).
package main

import (
	"context"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/config"
	"github.com/mjudeikis/baltic-osint-hub/internal/enrich"
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
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("store", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	fetchAll(ctx, log, db)

	if cfg.AnthropicAPIKey == "" {
		log.Warn("ANTHROPIC_API_KEY not set; skipping classification")
		return
	}
	classify(ctx, log, db, enrich.NewClassifier(cfg.AnthropicAPIKey, cfg.EnrichModel), cfg.MaxEnrichPerRun)
}

func fetchAll(ctx context.Context, log *slog.Logger, db *store.Store) {
	var wg sync.WaitGroup
	for _, f := range sources.All() {
		wg.Add(1)
		go func(f sources.Fetcher) {
			defer wg.Done()
			last, err := db.LastSuccessfulRun(ctx, f.Name())
			if err == nil && time.Since(last) < f.Interval() {
				return
			}
			started := time.Now()
			items, err := f.Fetch(ctx)
			added := 0
			for i := range items {
				ok, insErr := db.InsertRawItem(ctx, &items[i])
				if insErr != nil {
					log.Error("insert", "source", f.Name(), "err", insErr)
					continue
				}
				if ok {
					added++
				}
			}
			db.RecordSourceRun(ctx, f.Name(), started, len(items), added, err)
			if err != nil {
				log.Error("fetch", "source", f.Name(), "err", err)
			} else {
				log.Info("fetched", "source", f.Name(), "found", len(items), "new", added)
			}
		}(f)
	}
	wg.Wait()
}

const batchSize = 20

func classify(ctx context.Context, log *slog.Logger, db *store.Store, cls *enrich.Classifier, max int) {
	pending, err := db.PendingItems(ctx, max)
	if err != nil {
		log.Error("pending", "err", err)
		return
	}

	// Cheap keyword pre-filter first: off-region/off-topic items never
	// reach the LLM.
	var toClassify []store.RawItem
	for _, it := range pending {
		if enrich.PassesPrefilter(it.Source, it.Title, it.Body) {
			toClassify = append(toClassify, it)
			continue
		}
		if err := db.SetItemStatus(ctx, it.ID, store.StatusIrrelevant); err != nil {
			log.Error("status", "id", it.ID, "err", err)
		}
	}
	log.Info("classifying", "pending", len(pending), "after_prefilter", len(toClassify))

	classified, relevant := 0, 0
	for start := 0; start < len(toClassify); start += batchSize {
		batch := toClassify[start:min(start+batchSize, len(toClassify))]
		verdicts, err := cls.ClassifyBatch(ctx, batch)
		if err != nil {
			log.Error("classify batch", "err", err)
			continue // items stay 'new' and are retried next run
		}
		for _, it := range batch {
			v, ok := verdicts[it.ID]
			if !ok {
				continue // model skipped it; retry next run
			}
			classified++
			if !v.Relevant {
				_ = db.SetItemStatus(ctx, it.ID, store.StatusIrrelevant)
				continue
			}
			occurred := time.Now()
			if it.PublishedAt != nil {
				occurred = *it.PublishedAt
			}
			inc := &store.Incident{
				RawItemID:  it.ID,
				Category:   v.Category,
				Countries:  v.Countries,
				Severity:   v.Severity,
				SummaryEN:  v.Summary,
				Lat:        v.Lat,
				Lon:        v.Lon,
				OccurredAt: occurred,
			}
			if err := db.InsertIncident(ctx, inc); err != nil {
				log.Error("insert incident", "id", it.ID, "err", err)
				_ = db.SetItemStatus(ctx, it.ID, store.StatusError)
				continue
			}
			_ = db.SetItemStatus(ctx, it.ID, store.StatusClassified)
			relevant++
		}
	}
	log.Info("classification done", "classified", classified, "incidents", relevant)
}
