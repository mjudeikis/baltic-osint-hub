-- Sanctioned and shadow-fleet vessels, from OpenSanctions' maritime dataset.
--
-- The sea layer could previously only say "a vessel loitered over a cable
-- corridor". That is a curiosity. "A vessel already listed as shadow-fleet
-- loitered over a cable corridor" is a finding, and the only thing standing
-- between the two was a lookup table.
--
-- Keyed by MMSI because that is what AIS broadcasts. Only about a third of the
-- dataset's vessels carry one — the rest are IMO-only and cannot be matched
-- against a live AIS position, which is a limitation of the join, not of the
-- source.
--
-- Source: https://www.opensanctions.org/datasets/maritime/ (CC BY-NC 4.0).

CREATE TABLE IF NOT EXISTS sanctioned_vessels (
    mmsi       BIGINT PRIMARY KEY,
    imo        TEXT NOT NULL DEFAULT '',
    name       TEXT NOT NULL DEFAULT '',
    -- OpenSanctions' own risk tags, semicolon-joined as published. The values
    -- that matter here are 'sanction', 'mare.shadow' (shadow fleet) and
    -- 'mare.detained'; an empty string means listed but untagged.
    risk       TEXT NOT NULL DEFAULT '',
    flag       TEXT NOT NULL DEFAULT '',
    countries  TEXT NOT NULL DEFAULT '',
    datasets   TEXT NOT NULL DEFAULT '',
    -- Entity page, so a claim on the dashboard is always one click from its
    -- evidence rather than asking the reader to take our word for it.
    url        TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS sanctioned_vessels_risk_idx ON sanctioned_vessels (risk);
