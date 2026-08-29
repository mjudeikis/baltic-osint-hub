-- Sentinel-1 SAR change detection over monitored AOIs.

CREATE TABLE IF NOT EXISTS layer_sar (
    aoi             TEXT             NOT NULL,
    interval_start  TIMESTAMPTZ      NOT NULL,
    interval_end    TIMESTAMPTZ      NOT NULL,
    bright_fraction DOUBLE PRECISION NOT NULL,
    mean_db         DOUBLE PRECISION,
    sample_count    BIGINT           NOT NULL DEFAULT 0,
    PRIMARY KEY (aoi, interval_start)
);

CREATE INDEX IF NOT EXISTS layer_sar_aoi_idx ON layer_sar (aoi, interval_start DESC);

CREATE TABLE IF NOT EXISTS layer_sar_anomaly (
    id              BIGSERIAL PRIMARY KEY,
    aoi             TEXT             NOT NULL,
    interval_start  TIMESTAMPTZ      NOT NULL,
    bright_fraction DOUBLE PRECISION NOT NULL,
    baseline_median DOUBLE PRECISION NOT NULL,
    zscore          DOUBLE PRECISION NOT NULL,
    detected_at     TIMESTAMPTZ      NOT NULL DEFAULT now(),
    UNIQUE (aoi, interval_start)
);

CREATE INDEX IF NOT EXISTS layer_sar_anomaly_time_idx ON layer_sar_anomaly (detected_at DESC);
