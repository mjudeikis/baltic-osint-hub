-- Signal layers: machine measurements shown on the map, separate from the
-- LLM-classified incident feed.

CREATE TABLE IF NOT EXISTS layer_firms (
    id          BIGSERIAL PRIMARY KEY,
    lat         DOUBLE PRECISION NOT NULL,
    lon         DOUBLE PRECISION NOT NULL,
    brightness  REAL NOT NULL DEFAULT 0,
    frp         REAL NOT NULL DEFAULT 0,   -- fire radiative power, MW
    confidence  TEXT NOT NULL DEFAULT '',
    sector      TEXT NOT NULL DEFAULT '',  -- which border sector box matched
    detected_at TIMESTAMPTZ NOT NULL,
    UNIQUE (lat, lon, detected_at)
);
CREATE INDEX IF NOT EXISTS layer_firms_time_idx ON layer_firms (detected_at DESC);

CREATE TABLE IF NOT EXISTS layer_gpsjam (
    day  DATE NOT NULL,
    hex  TEXT NOT NULL,       -- H3 resolution-4 cell id
    good INT  NOT NULL,
    bad  INT  NOT NULL,
    PRIMARY KEY (day, hex)
);

CREATE TABLE IF NOT EXISTS layer_air (
    id       BIGSERIAL PRIMARY KEY,
    seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    icao24   TEXT NOT NULL,
    callsign TEXT NOT NULL DEFAULT '',
    country  TEXT NOT NULL DEFAULT '',
    box      TEXT NOT NULL,
    lat      DOUBLE PRECISION,
    lon      DOUBLE PRECISION,
    altitude REAL,
    velocity REAL,
    reason   TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS layer_air_time_idx ON layer_air (seen_at DESC);
CREATE INDEX IF NOT EXISTS layer_air_dedupe_idx ON layer_air (icao24, box, seen_at DESC);

CREATE TABLE IF NOT EXISTS layer_sea (
    id          BIGSERIAL PRIMARY KEY,
    detected_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    mmsi        BIGINT NOT NULL,
    ship_name   TEXT NOT NULL DEFAULT '',
    corridor    TEXT NOT NULL,
    lat         DOUBLE PRECISION,
    lon         DOUBLE PRECISION,
    sog         REAL,             -- speed over ground, knots
    event       TEXT NOT NULL,    -- loitering | ais-gap
    started_at  TIMESTAMPTZ
);
CREATE INDEX IF NOT EXISTS layer_sea_time_idx ON layer_sea (detected_at DESC);
CREATE INDEX IF NOT EXISTS layer_sea_dedupe_idx ON layer_sea (mmsi, corridor, event, detected_at DESC);
