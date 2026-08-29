package cluster

import (
	"context"
	"log/slog"
	"time"
)

// Store is the slice of the database this package needs. Declared here rather
// than imported so the clustering loop can be tested against the real store
// and against a fake, and so cmd/collector stays thin.
type Store interface {
	IncidentsNeedingEmbedding(ctx context.Context, limit int) ([]Pending, error)
	SetIncidentEmbedding(ctx context.Context, id int64, vec []float32) error
	Candidates(ctx context.Context, category string, at time.Time, window time.Duration) ([]Candidate, error)
	CreateEventFor(ctx context.Context, incidentID int64) (int64, error)
	AttachIncident(ctx context.Context, incidentID, eventID int64) error
	RefreshEvent(ctx context.Context, eventID int64) error
}

// Embedder turns summaries into vectors.
type Embedder interface {
	Embed(ctx context.Context, inputs []string) ([][]float32, error)
}

// Pending is a classified incident awaiting an embedding.
type Pending struct {
	ID         int64
	Category   string
	Countries  []string
	SummaryEN  string
	OccurredAt time.Time
}

// Batch is the number of summaries per embeddings request. The endpoint
// accepts far more, but modest requests bound the damage when one fails and
// has to be retried on the next run.
const Batch = 64

// Result reports what one clustering pass did.
type Result struct {
	Considered int
	NewEvents  int
	Merged     int
}

// Run embeds and clusters incidents that do not yet belong to an event.
//
// The work is idempotent and resumable: an incident is picked up whenever its
// embedding is missing, so a failed or truncated run continues next time
// rather than leaving the table half-processed.
func Run(ctx context.Context, db Store, emb Embedder, max int, threshold float64, log *slog.Logger) (Result, error) {
	var res Result
	pending, err := db.IncidentsNeedingEmbedding(ctx, max)
	if err != nil {
		return res, err
	}
	res.Considered = len(pending)
	if len(pending) == 0 {
		return res, nil
	}

	for start := 0; start < len(pending); start += Batch {
		batch := pending[start:min(start+Batch, len(pending))]
		texts := make([]string, len(batch))
		for i, p := range batch {
			texts[i] = p.SummaryEN
		}
		vecs, err := emb.Embed(ctx, texts)
		if err != nil {
			// Leave the batch unembedded; it is retried on the next run.
			log.Error("cluster: embed", "err", err, "batch", len(batch))
			return res, err
		}

		// Assignment is sequential on purpose: an incident must be able to
		// join an event that an earlier incident in this same batch created,
		// which a concurrent pass would miss.
		for i, p := range batch {
			if err := db.SetIncidentEmbedding(ctx, p.ID, vecs[i]); err != nil {
				log.Error("cluster: store embedding", "id", p.ID, "err", err)
				continue
			}
			candidates, err := db.Candidates(ctx, p.Category, p.OccurredAt, Window)
			if err != nil {
				log.Error("cluster: candidates", "id", p.ID, "err", err)
				continue
			}

			eventID, score, ok := Best(vecs[i], p.Countries, candidates, threshold)
			if ok {
				if err := db.AttachIncident(ctx, p.ID, eventID); err != nil {
					log.Error("cluster: attach", "id", p.ID, "err", err)
					continue
				}
				res.Merged++
				log.Info("cluster: merged", "incident", p.ID, "event", eventID, "score", score)
			} else {
				eventID, err = db.CreateEventFor(ctx, p.ID)
				if err != nil {
					log.Error("cluster: create event", "id", p.ID, "err", err)
					continue
				}
				res.NewEvents++
			}
			// Derived fields must never lag their members, so refresh on every
			// attach — including the first, so a new event gets its
			// corroboration score rather than the default zero.
			if err := db.RefreshEvent(ctx, eventID); err != nil {
				log.Error("cluster: refresh", "event", eventID, "err", err)
			}
		}
	}
	return res, nil
}
