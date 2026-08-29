-- Tone: direction of an incident for regional security. Existing rows default
-- to 'neutral' rather than 'negative' so a backfill cannot invent alarm; they
-- are re-classified by resetting their raw_items to status 'new'.
ALTER TABLE incidents ADD COLUMN IF NOT EXISTS tone TEXT NOT NULL DEFAULT 'neutral';

CREATE INDEX IF NOT EXISTS incidents_tone_idx ON incidents (tone);
CREATE INDEX IF NOT EXISTS incidents_tone_occurred_idx ON incidents (occurred_at DESC, tone);
