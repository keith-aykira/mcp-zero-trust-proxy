-- Migration: fix_licenses_rls
-- Removes the overly permissive SELECT policy that allowed any anonymous caller
-- to read all license keys, emails, and Stripe IDs from the licenses table.
--
-- Service role (used by Edge Functions) bypasses RLS automatically, so no
-- explicit SELECT policy is needed for current functionality.
-- If authenticated user self-service is added later, re-add a scoped policy.

-- Drop the overly permissive SELECT policy
DROP POLICY IF EXISTS "Users can read own licenses" ON licenses;

-- No anon/authenticated SELECT policy needed.
-- Service role (used by Edge Functions) bypasses RLS automatically.
-- If you need authenticated users to read their own licenses later:
-- CREATE POLICY "Users can read own licenses" ON licenses
--   FOR SELECT TO authenticated
--   USING (email = auth.jwt() ->> 'email');
