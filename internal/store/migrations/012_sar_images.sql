-- Rendered Sentinel-1 passes for flagged sites, so a reader can see the
-- change on the page instead of being sent to the Copernicus Browser to
-- reconstruct it by hand. One 'before' and one 'after' rendering per
-- anomalous interval, keyed by that interval.
CREATE TABLE IF NOT EXISTS layer_sar_image (
  aoi            TEXT NOT NULL,
  interval_start TIMESTAMPTZ NOT NULL,
  kind           TEXT NOT NULL CHECK (kind IN ('before', 'after')),
  -- The interval actually rendered: for 'after' it equals the anomalous
  -- interval, for 'before' it is the representative baseline pass.
  captured_start TIMESTAMPTZ NOT NULL,
  captured_end   TIMESTAMPTZ NOT NULL,
  png            BYTEA NOT NULL,
  fetched_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (aoi, interval_start, kind)
);
