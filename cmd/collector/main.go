// Collector runs one fetch+classify cycle and exits. It is designed to run
// as a Kubernetes CronJob (hourly; see deploy/). Each source and layer keeps
// its own minimum interval, so the schedule can be tightened without
// multiplying upstream calls.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/mjudeikis/baltic-osint-hub/internal/cluster"
	"github.com/mjudeikis/baltic-osint-hub/internal/config"
	"github.com/mjudeikis/baltic-osint-hub/internal/enrich"
	"github.com/mjudeikis/baltic-osint-hub/internal/layers"
	"github.com/mjudeikis/baltic-osint-hub/internal/posture"
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
	// One run must be able to cover the whole watchlist. Each SAR site is a
	// separate Copernicus round-trip, so this budget scales with the site count.
	ctx, cancel := context.WithTimeout(context.Background(), cfg.RunTimeout)
	defer cancel()

	db, err := store.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("store", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	db.SetLogger(log)

	// State-controlled outlets are monitored but must never move the
	// posture reading; the store filters them out of every published count.
	store.SetStateControlled(sources.StateControlledList())
	log.Info("state-controlled sources excluded from counts", "sources", len(store.StateControlledSources))

	// Retention runs whether or not the rest of the cycle completes, so a
	// run that dies in classification still keeps the tables bounded.
	defer prune(log, db)

	fetchAll(ctx, log, db)
	runLayers(ctx, log, db, cfg)

	if cfg.OpenAIAPIKey == "" {
		log.Warn("OPENAI_API_KEY not set; skipping classification")
		return
	}
	classify(ctx, log, db,
		enrich.NewClassifier(cfg.OpenAIAPIKey, cfg.EnrichModel, cfg.OpenAIBaseURL).WithLogger(log),
		cfg.MaxEnrichPerRun)

	// Clustering runs after classification so items enriched in this same run
	// are grouped immediately rather than a cycle late.
	clusterIncidents(ctx, log, db, enrich.NewEmbedder(cfg.OpenAIAPIKey, cfg.OpenAIBaseURL),
		cfg.MaxClusterPerRun, cfg.ClusterThreshold)

	// Snapshot last, so the archive records the reading as it stands after
	// this run's classification and clustering rather than before them.
	snapshotPosture(ctx, log, db)
}

// prune applies the retention policy. It uses its own short context so it
// still runs when the run budget has been exhausted by the work before it.
func prune(log *slog.Logger, db *store.Store) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	res, err := db.Prune(ctx)
	if err != nil {
		log.Error("prune", "err", err)
		return
	}
	log.Info("pruned", "source_runs", res.SourceRuns, "raw_items", res.RawItems,
		"air", res.Air, "firms", res.FIRMS, "sea", res.Sea, "sar_images", res.SARImages)
}

// snapshotPosture archives what the dashboard is publishing right now, so the
// reading can be checked after the fact — including when it turns out to have
// been wrong. A monitor that asks to be trusted should keep a record of its own
// judgements rather than only ever showing the current one.
func snapshotPosture(ctx context.Context, log *slog.Logger, db *store.Store) {
	byTone, sev, corroborated, err := db.ToneCounts(ctx, 7, "")
	if err != nil {
		log.Error("snapshot: tone counts", "err", err)
		return
	}
	reading := posture.Evaluate(posture.Counts{
		Positive:               byTone[enrich.TonePositive],
		Neutral:                byTone[enrich.ToneNeutral],
		Negative:               byTone[enrich.ToneNegative],
		NegativeBySeverity:     sev,
		CorroboratedBySeverity: corroborated,
	})
	if history, err := db.WeeklyAdverseHistory(ctx, 12); err == nil {
		reading = reading.WithHistory(history)
	}
	// min severity 2, matching the dashboard's board: the archived snapshot
	// should record what the page showed, and severity-1 commentary is not
	// counted as an event there.
	summary, err := db.Summary(ctx, 2)
	if err != nil {
		log.Error("snapshot: summary", "err", err)
		summary = nil
	}
	if err := db.SavePostureSnapshot(ctx, time.Now(), int(reading.Level), reading.LevelName,
		reading.Trend, reading.Counts.Negative, reading.Counts.Positive, reading.Counts.Neutral,
		reading, summary); err != nil {
		log.Error("snapshot: save", "err", err)
		return
	}
	log.Info("posture snapshot", "level", reading.LevelName, "trend", reading.Trend,
		"adverse", reading.Counts.Negative, "favourable", reading.Counts.Positive)
}

// clusterIncidents groups incident reports of the same event. It is the reason
// the dashboard's counts mean anything: without it, one sabotage story carried
// by six outlets contributed six adverse items to the posture reading.
func clusterIncidents(ctx context.Context, log *slog.Logger, db *store.Store, emb *enrich.Embedder, max int, threshold float64) {
	started := time.Now()
	res, err := cluster.Run(ctx, db, emb, max, threshold, log)
	db.RecordSourceRun(ctx, "cluster", started, res.Considered, res.Merged, err)
	if err != nil {
		log.Error("clustering", "err", err)
		return
	}

	// Recompute every event in the published window, not just the ones this
	// run touched. cluster.Run only refreshes events it attached to, so a
	// change to the aggregation rules would otherwise apply to new events
	// alone and leave the dashboard showing a mix of old and new logic. This
	// makes such changes self-healing: deploy, wait one run, done. aggregate()
	// is a pure function of an event's members, so re-running it on an
	// unchanged event rewrites identical values.
	//
	// The window covers everything the dashboard publishes — the 7-day
	// posture, the 35-day board and the 12-week baseline — with room to spare.
	if n, err := db.RefreshEventsSince(ctx, time.Now().AddDate(0, 0, -120)); err != nil {
		log.Error("refresh events", "err", err)
	} else if n > 0 {
		log.Info("events refreshed", "count", n)
	}
	if res.Considered > 0 {
		log.Info("clustering done", "considered", res.Considered,
			"new_events", res.NewEvents, "merged", res.Merged)
	}
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

// runLayers ingests the machine-signal layers (thermal anomalies, GPS
// jamming, air activity). Errors are logged and recorded, never fatal.
func runLayers(ctx context.Context, log *slog.Logger, db *store.Store, cfg *config.Config) {
	client := &http.Client{Timeout: 2 * time.Minute}
	run := func(name string, fn func() error) {
		started := time.Now()
		err := fn()
		db.RecordSourceRun(ctx, name, started, 0, 0, err)
		if err != nil {
			log.Error("layer", "name", name, "err", err)
		}
	}

	// Each layer carries its own minimum interval so the cron schedule and the
	// upstream budgets stay independent: making the CronJob more frequent must
	// not multiply calls to a rate-limited service. The gate keys off the last
	// *successful* run, so a failing layer keeps retrying every invocation.
	gated := func(name string, minInterval time.Duration, fn func() error) {
		if recentlyRan(ctx, db, name, minInterval) {
			log.Info("layer skipped; ran recently", "name", name, "interval", minInterval.String())
			return
		}
		run(name, fn)
	}

	// gpsjam publishes a day behind and stores per day, so a handful of
	// attempts is enough to catch publication.
	gated("layer:gpsjam", 6*time.Hour, func() error {
		return (&layers.Gpsjam{Client: client}).Run(ctx, db, log)
	})
	// One snapshot per border box per run; aircraft transit a box in minutes,
	// so sampling more often than this buys little against the credit budget.
	gated("layer:opensky", 30*time.Minute, func() error {
		return (&layers.OpenSky{
			ClientID:     cfg.OpenSkyClientID,
			ClientSecret: cfg.OpenSkyClientSecret,
			Client:       client,
		}).Run(ctx, db, log)
	})
	// VIIRS has roughly two overpasses a day and NRT data lands within hours.
	if cfg.FIRMSMapKey != "" {
		gated("layer:firms", 2*time.Hour, func() error {
			return (&layers.FIRMS{MapKey: cfg.FIRMSMapKey, Client: client}).Run(ctx, db, log)
		})
	} else {
		log.Warn("FIRMS_MAP_KEY not set; skipping thermal layer")
	}

	// The sanctions watchlist is rebuilt daily upstream and is a 5MB download,
	// so once a day is both sufficient and polite. It needs no key.
	gated("layer:sanctions", 24*time.Hour, func() error {
		return (&layers.SanctionedVessels{Client: client}).Run(ctx, db, log)
	})

	// The whole warning list is a ~25MB download that restates its own history,
	// so daily is right: more often wastes bandwidth for identical data.
	gated("layer:certpl", 24*time.Hour, func() error {
		return (&layers.CertPL{Client: client}).Run(ctx, db, log)
	})

	// SAR is the expensive layer (one Statistical API call per AOI against a
	// finite processing-unit budget) and Sentinel-1's revisit is measured in
	// days, so run it at most daily.
	if cfg.CopernicusClientID == "" || cfg.CopernicusClientSecret == "" {
		log.Warn("COPERNICUS_CLIENT_ID/SECRET not set; skipping SAR change detection")
		return
	}
	gated("layer:sentinel", 20*time.Hour, func() error {
		return (&layers.Sentinel{
			ClientID:     cfg.CopernicusClientID,
			ClientSecret: cfg.CopernicusClientSecret,
			Client:       client,
		}).Run(ctx, db, log)
	})
}

func recentlyRan(ctx context.Context, db *store.Store, name string, within time.Duration) bool {
	last, err := db.LastSuccessfulRun(ctx, name)
	return err == nil && time.Since(last) < within
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
	//
	// State-controlled and independent items are batched separately. The
	// item text is untrusted input to the model, and a single crafted post
	// in a batch can steer the verdicts for the nineteen items around it. A
	// state outlet's post is the one most likely to try; keeping it in its
	// own batch bounds the damage to other state-media items, which the
	// posture calculation already discounts.
	var independent, stateRun []store.RawItem
	for _, it := range pending {
		if !enrich.PassesPrefilter(it.Source, it.Title, it.Body) {
			if err := db.SetItemStatus(ctx, it.ID, store.StatusIrrelevant); err != nil {
				log.Error("status", "id", it.ID, "err", err)
			}
			continue
		}
		if sources.IsStateControlled(it.Source) {
			stateRun = append(stateRun, it)
		} else {
			independent = append(independent, it)
		}
	}
	log.Info("classifying", "pending", len(pending),
		"after_prefilter", len(independent)+len(stateRun), "state_media", len(stateRun))

	var stats classifyStats
	for _, group := range [][]store.RawItem{independent, stateRun} {
		for start := 0; start < len(group); start += batchSize {
			classifyBatch(ctx, log, db, cls, group[start:min(start+batchSize, len(group))], &stats)
		}
	}
	log.Info("classification done", "classified", stats.classified, "incidents", stats.relevant,
		"retired", stats.retired)
}

type classifyStats struct{ classified, relevant, retired int }

// classifyBatch runs one batch, splitting it when the model's reply was cut
// off by the token cap. A truncated reply for twenty items is usually a
// reply that would fit for ten, so halving recovers most of the batch in
// the same run rather than leaving it all for the next one. Every attempt
// is counted first, so a batch that never succeeds is retired after
// MaxClassifyAttempts rather than retried forever.
func classifyBatch(ctx context.Context, log *slog.Logger, db *store.Store, cls *enrich.Classifier, batch []store.RawItem, stats *classifyStats) {
	ids := make([]int64, len(batch))
	for i, it := range batch {
		ids[i] = it.ID
	}
	retired, err := db.RecordClassifyAttempt(ctx, ids)
	if err != nil {
		log.Error("record attempt", "err", err)
		return
	}
	if len(retired) > 0 {
		log.Warn("items retired after repeated classification failures", "ids", retired)
		stats.retired += len(retired)
	}

	verdicts, err := cls.ClassifyBatch(ctx, batch)
	if errors.Is(err, enrich.ErrTruncated) && len(batch) > 1 {
		log.Warn("classify batch truncated; splitting", "size", len(batch))
		half := len(batch) / 2
		classifyBatch(ctx, log, db, cls, batch[:half], stats)
		classifyBatch(ctx, log, db, cls, batch[half:], stats)
		return
	}
	if err != nil {
		log.Error("classify batch", "err", err, "size", len(batch))
		return // items stay 'new' (or retired above) and are retried next run
	}
	for _, it := range batch {
		v, ok := verdicts[it.ID]
		if !ok {
			continue // model skipped it; retry next run
		}
		stats.classified++
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
			Tone:       v.Tone,
			Place:      v.Place,
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
		stats.relevant++
	}
}
