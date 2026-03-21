-- Stripe event deduplication table
CREATE TABLE IF NOT EXISTS stripe_events (
  event_id TEXT PRIMARY KEY,
  event_type TEXT NOT NULL,
  processed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- RLS: service role only (Edge Functions bypass RLS)
ALTER TABLE stripe_events ENABLE ROW LEVEL SECURITY;

-- Index for cleanup of old events
CREATE INDEX idx_stripe_events_processed_at ON stripe_events (processed_at);
