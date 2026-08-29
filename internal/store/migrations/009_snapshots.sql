-- Daily snapshots of what the dashboard said.
--
-- Until now the dashboard had no memory of its own judgements: it showed the
-- current reading and nothing else, so "what did we say on 12 January, and did
-- we retract it?" was unanswerable. For a tool that asks to be trusted about a
-- politically charged subject, that is the wrong way round — a published
-- assessment should be checkable after the fact, including when it was wrong.
--
-- One row per day. The payloads are stored as JSON rather than exploded into
-- columns on purpose: this is an archive of what was published, and it must
-- keep meaning the same thing even after the posture rules change. Normalising
-- it would silently rewrite history the next time the schema moved.

CREATE TABLE IF NOT EXISTS posture_snapshots (
    day        DATE PRIMARY KEY,
    -- Denormalised for cheap listing and charting without parsing the JSON.
    level      SMALLINT NOT NULL,
    level_name TEXT     NOT NULL,
    trend      TEXT     NOT NULL DEFAULT '',
    adverse    INT      NOT NULL DEFAULT 0,
    favourable INT      NOT NULL DEFAULT 0,
    neutral    INT      NOT NULL DEFAULT 0,
    -- The full published reading and the country/category board, exactly as
    -- served that day.
    reading    JSONB    NOT NULL,
    summary    JSONB    NOT NULL DEFAULT '[]'::jsonb,
    taken_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS posture_snapshots_day_idx ON posture_snapshots (day DESC);
