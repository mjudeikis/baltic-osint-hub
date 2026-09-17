-- Per-site refresh bookkeeping for the SAR layer.
--
-- The layer used to gate on the whole watchlist succeeding, so one failing
-- site re-requested all 48 every hour and drained the Copernicus
-- processing-unit budget. Recording when each site last refreshed lets the
-- collector work through the list in small batches and skip sites that are
-- already fresh.
CREATE TABLE IF NOT EXISTS layer_sar_fetch (
    aoi        TEXT        PRIMARY KEY,
    fetched_at TIMESTAMPTZ NOT NULL
);
