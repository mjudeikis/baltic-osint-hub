-- The name that was geocoded, so a map pin can say what it refers to rather
-- than being an unexplained dot.
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS place TEXT NOT NULL DEFAULT '';
