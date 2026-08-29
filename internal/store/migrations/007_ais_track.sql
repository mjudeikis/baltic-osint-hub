-- Archived AIS positions inside the cable corridors.
--
-- aisstream.io is realtime-only: it tells us a vessel is loitering *now*, but
-- keeps no history, and we stored none either. That meant a gap or a loitering
-- event could never be re-examined afterwards — no track to reconstruct, no
-- way to ask "what else was in that corridor that night", and no way to revise
-- a judgement once more became known. Every day without this table was history
-- permanently lost.
--
-- Source is Finnish Digitraffic (CC BY 4.0, no key). See the layer for what it
-- does and does not cover.

CREATE TABLE IF NOT EXISTS layer_ais_track (
    id        BIGSERIAL PRIMARY KEY,
    mmsi      BIGINT           NOT NULL,
    lat       DOUBLE PRECISION NOT NULL,
    lon       DOUBLE PRECISION NOT NULL,
    sog       REAL,             -- speed over ground, knots
    cog       REAL,             -- course over ground, degrees
    nav_stat  SMALLINT,         -- AIS navigational status (5 = moored)
    corridor  TEXT             NOT NULL,
    -- Position timestamp from the AIS message, not our fetch time, so tracks
    -- stay correct even when a poll is late or retried.
    seen_at   TIMESTAMPTZ      NOT NULL
);

-- Track reconstruction is always "this vessel, over this period".
CREATE INDEX IF NOT EXISTS ais_track_mmsi_idx ON layer_ais_track (mmsi, seen_at DESC);
CREATE INDEX IF NOT EXISTS ais_track_seen_idx ON layer_ais_track (seen_at DESC);
CREATE INDEX IF NOT EXISTS ais_track_corridor_idx ON layer_ais_track (corridor, seen_at DESC);

-- One row per vessel per position report. Re-polling before the source
-- updates would otherwise store the same fix repeatedly.
CREATE UNIQUE INDEX IF NOT EXISTS ais_track_unique_fix
    ON layer_ais_track (mmsi, seen_at);
