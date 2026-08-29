-- Event clustering.
--
-- Until now one article produced one `incidents` row, so a single railway
-- sabotage reported by LRT, Delfi, ERR, LSM, Reuters and TASS counted six
-- times. Every count on the dashboard — the posture reading above all — was
-- therefore measuring media volume rather than events.
--
-- An `events` row is the deduplicated real-world occurrence; `incidents` rows
-- stay exactly as they are and gain a pointer to it. Nothing is dropped or
-- rewritten: an incident with a NULL event_id simply has not been clustered
-- yet, which is the correct state for the whole existing table until the
-- backfill catches up.

CREATE TABLE IF NOT EXISTS events (
    id          BIGSERIAL PRIMARY KEY,
    category    TEXT        NOT NULL,
    countries   TEXT[]      NOT NULL DEFAULT '{}',
    severity    SMALLINT    NOT NULL DEFAULT 1,
    tone        TEXT        NOT NULL DEFAULT 'neutral',
    place       TEXT        NOT NULL DEFAULT '',
    summary_en  TEXT        NOT NULL DEFAULT '',
    lat         DOUBLE PRECISION,
    lon         DOUBLE PRECISION,
    -- Earliest report of the event: who got there first, not who we saw first.
    occurred_at TIMESTAMPTZ NOT NULL,

    -- All derived from member incidents and recomputed whenever one is
    -- attached, so they are never allowed to drift from the members.
    -- source_count counts DISTINCT sources excluding state-controlled outlets,
    -- because corroboration by four Kremlin wires is not corroboration.
    source_count  INT     NOT NULL DEFAULT 0,
    total_reports INT     NOT NULL DEFAULT 0,
    confidence    REAL    NOT NULL DEFAULT 0,

    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS events_occurred_idx ON events (occurred_at DESC);
CREATE INDEX IF NOT EXISTS events_category_idx ON events (category);
CREATE INDEX IF NOT EXISTS events_countries_idx ON events USING GIN (countries);

-- ON DELETE SET NULL, not CASCADE: losing an event must never silently delete
-- the classified incidents that evidence it.
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS event_id BIGINT
    REFERENCES events(id) ON DELETE SET NULL;

-- 512-dimension embedding of summary_en. The classifier already writes every
-- summary in English regardless of the source language, which is what makes
-- cross-language clustering possible at all — an Estonian and a Russian report
-- of one event arrive here as two English sentences.
--
-- Stored as REAL[] rather than a pgvector column so the stock postgres:17-alpine
-- image needs no extension. Similarity is computed in Go over the small
-- candidate window (one category, ±72h), never as a table scan.
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS embedding REAL[];

CREATE INDEX IF NOT EXISTS incidents_event_idx ON incidents (event_id);
-- Drives the backfill scan: classified rows still awaiting an embedding.
CREATE INDEX IF NOT EXISTS incidents_unembedded_idx ON incidents (id)
    WHERE embedding IS NULL;
