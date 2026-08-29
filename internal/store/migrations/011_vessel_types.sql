-- AIS ship types, so the sea layer can tell a tanker from a pilot boat.
--
-- The loitering detector fired 97 times in one week, of which ~17 were pilot
-- boats, SAR craft, tugs and coast guard — vessels whose *job* is to hold
-- station. Measured against Finnish AIS, 229 of 1,002 vessels (23%) are
-- service craft of exactly that kind. Alerting on them is not a detection, it
-- is a description of a working harbour.
--
-- Populated from two places: Finnish Digitraffic's vessel registry (keyless,
-- polled with the AIS archive, good coverage in the Gulf of Finland) and
-- aisstream's ShipStaticData messages (covers the wider Baltic). Either may
-- write; whichever saw the vessel most recently wins.

CREATE TABLE IF NOT EXISTS vessel_types (
    mmsi       BIGINT PRIMARY KEY,
    -- AIS ship-and-cargo type. 50-59 are service and special craft, 70-79
    -- cargo, 80-89 tankers. 0 means the vessel broadcasts no type, which is
    -- itself worth not filtering on.
    ship_type  SMALLINT NOT NULL,
    name       TEXT NOT NULL DEFAULT '',
    imo        TEXT NOT NULL DEFAULT '',
    call_sign  TEXT NOT NULL DEFAULT '',
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS vessel_types_type_idx ON vessel_types (ship_type);
