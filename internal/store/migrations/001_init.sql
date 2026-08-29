CREATE TABLE IF NOT EXISTS raw_items (
    id            BIGSERIAL PRIMARY KEY,
    source        TEXT        NOT NULL,
    url           TEXT        NOT NULL UNIQUE,
    title         TEXT        NOT NULL,
    body          TEXT        NOT NULL DEFAULT '',
    lang          TEXT        NOT NULL DEFAULT '',
    published_at  TIMESTAMPTZ,
    fetched_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    content_hash  TEXT        NOT NULL,
    -- new: awaiting classification; classified: incident row exists;
    -- irrelevant: LLM or pre-filter rejected it; error: classification failed
    status        TEXT        NOT NULL DEFAULT 'new'
);

CREATE INDEX IF NOT EXISTS raw_items_status_idx ON raw_items (status);
CREATE INDEX IF NOT EXISTS raw_items_hash_idx ON raw_items (content_hash);
CREATE INDEX IF NOT EXISTS raw_items_published_idx ON raw_items (published_at DESC);

CREATE TABLE IF NOT EXISTS incidents (
    id            BIGSERIAL PRIMARY KEY,
    raw_item_id   BIGINT      NOT NULL UNIQUE REFERENCES raw_items(id) ON DELETE CASCADE,
    category      TEXT        NOT NULL,
    countries     TEXT[]      NOT NULL DEFAULT '{}',
    severity      SMALLINT    NOT NULL CHECK (severity BETWEEN 1 AND 5),
    summary_en    TEXT        NOT NULL,
    lat           DOUBLE PRECISION,
    lon           DOUBLE PRECISION,
    confidence    REAL        NOT NULL DEFAULT 0,
    occurred_at   TIMESTAMPTZ NOT NULL,
    classified_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS incidents_occurred_idx ON incidents (occurred_at DESC);
CREATE INDEX IF NOT EXISTS incidents_category_idx ON incidents (category);
CREATE INDEX IF NOT EXISTS incidents_countries_idx ON incidents USING GIN (countries);

CREATE TABLE IF NOT EXISTS source_runs (
    id          BIGSERIAL PRIMARY KEY,
    source      TEXT        NOT NULL,
    started_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    finished_at TIMESTAMPTZ,
    items_found INT         NOT NULL DEFAULT 0,
    items_new   INT         NOT NULL DEFAULT 0,
    error       TEXT        NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS source_runs_source_idx ON source_runs (source, started_at DESC);
