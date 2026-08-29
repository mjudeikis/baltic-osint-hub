-- Daily volume of domains added to CERT.PL's warning list.
--
-- The list itself is a phishing/malware blocklist for Poland — 140,000 entries
-- and growing by several hundred a day. The individual domains are not
-- incidents and do not belong in the feed; what is useful is the *rate*, as a
-- cyber-activity signal with a real baseline.
--
-- Every record carries its insert date, so one download reconstructs the whole
-- daily series back to 2020. That matters: it is the only cyber signal in this
-- project that arrives with years of history rather than starting from the day
-- we happened to switch it on.
--
-- Counts only, never the domains: republishing a blocklist is not this
-- project's job, and storing it would invite mistaking a phishing domain for a
-- state-attributed incident.

CREATE TABLE IF NOT EXISTS layer_certpl (
    day     DATE PRIMARY KEY,
    -- Domains first listed that day.
    added   INT NOT NULL DEFAULT 0,
    -- Domains whose listing ended that day (delisted / expired).
    removed INT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS certpl_day_idx ON layer_certpl (day DESC);
