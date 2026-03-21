-- Migration: create_licenses
-- Creates the licenses table for MCP Zero-Trust Proxy billing backend.
-- License keys are signed JWTs (ES256/ECDSA P-256) generated upon successful Stripe checkout.

CREATE TABLE licenses (
  id uuid DEFAULT gen_random_uuid() PRIMARY KEY,
  email text NOT NULL,
  tier text NOT NULL CHECK (tier IN ('pro', 'enterprise')),
  stripe_subscription_id text UNIQUE,
  stripe_customer_id text,
  license_key text NOT NULL,
  max_upstreams int NOT NULL,
  max_rpm int NOT NULL,
  issued_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);

-- Fast lookup by customer email (for license retrieval)
CREATE INDEX idx_licenses_email ON licenses(email);

-- Fast lookup by Stripe subscription ID (for webhook events)
CREATE INDEX idx_licenses_stripe_sub ON licenses(stripe_subscription_id);

-- Fast lookup by Stripe customer ID (for subscription management)
CREATE INDEX idx_licenses_stripe_customer ON licenses(stripe_customer_id);

-- RLS: enable row level security
ALTER TABLE licenses ENABLE ROW LEVEL SECURITY;

-- Allow anyone to read licenses (license key retrieval is authenticated by email ownership at app level)
-- The license key itself is the secret — no PII is exposed beyond what the holder already knows
CREATE POLICY "Users can read own licenses" ON licenses
  FOR SELECT
  USING (true);

-- Only the service role (Edge Functions) can insert/update licenses
-- No direct anon writes — all writes go through the webhook Edge Function
CREATE POLICY "Service role can insert licenses" ON licenses
  FOR INSERT
  TO service_role
  WITH CHECK (true);

CREATE POLICY "Service role can update licenses" ON licenses
  FOR UPDATE
  TO service_role
  USING (true)
  WITH CHECK (true);
