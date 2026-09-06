-- Classification attempts and query indexes.
--
-- attempts counts how many times the classifier has been asked about a raw
-- item. Without it a batch the model can never answer (a poisoned post, an
-- over-long reply, a persistent parse failure) was retried on every run
-- forever, spending budget and starving newer items behind it. The collector
-- retires an item after three attempts.
ALTER TABLE raw_items ADD COLUMN IF NOT EXISTS attempts INT NOT NULL DEFAULT 0;

-- PendingItems orders by (status, attempts, id) so retries queue behind
-- fresh items rather than ahead of them.
CREATE INDEX IF NOT EXISTS raw_items_pending_idx ON raw_items (status, attempts, id);

-- Every counting query groups incidents by event and bounds them by time.
CREATE INDEX IF NOT EXISTS incidents_event_occurred_idx ON incidents (event_id, occurred_at DESC);

-- Retention: pruning old runs and layer rows walks these by time.
CREATE INDEX IF NOT EXISTS source_runs_started_idx ON source_runs (started_at);
CREATE INDEX IF NOT EXISTS raw_items_status_fetched_idx ON raw_items (status, fetched_at);
