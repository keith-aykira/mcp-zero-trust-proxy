// stripe-webhook/index.ts
// Supabase Edge Function (Deno) — handles Stripe webhook events for MCP Zero-Trust Proxy billing.
//
// Handles:
//   checkout.session.completed    → generate and store a license key
//   customer.subscription.deleted → revoke the license
//   customer.subscription.updated → renew (active) or revoke (past_due/canceled)
//
// Secrets required:
//   STRIPE_WEBHOOK_SECRET       — Stripe webhook signing secret
//   SUPABASE_URL                — auto-provided by Supabase
//   SUPABASE_SERVICE_ROLE_KEY   — auto-provided by Supabase
//   CREATE_LICENSE_FUNCTION_URL — URL of the create-license Edge Function
//                                 e.g. https://<project>.supabase.co/functions/v1/create-license
//
// Stripe signature verification uses HMAC-SHA256 (raw body + Stripe-Signature header).
// No Stripe SDK — lightweight Deno crypto only.

import { serve } from "https://deno.land/std@0.168.0/http/server.ts";
import { createClient } from "https://esm.sh/@supabase/supabase-js@2";

// ─── Stripe signature verification ───────────────────────────────────────────

const STRIPE_SIGNATURE_TOLERANCE_SECONDS = 300; // 5 minutes

async function verifyStripeSignature(
  rawBody: string,
  signatureHeader: string,
  webhookSecret: string,
): Promise<boolean> {
  // Parse the Stripe-Signature header
  // Format: t=<timestamp>,v1=<signature>[,v1=<signature>...]
  const parts = signatureHeader.split(",");
  const timestampPart = parts.find((p) => p.startsWith("t="));
  const signatureParts = parts.filter((p) => p.startsWith("v1="));

  if (!timestampPart || signatureParts.length === 0) {
    console.error("Invalid Stripe-Signature header format");
    return false;
  }

  const timestamp = parseInt(timestampPart.slice(2), 10);
  const now = Math.floor(Date.now() / 1000);

  // Check timestamp is within tolerance
  if (Math.abs(now - timestamp) > STRIPE_SIGNATURE_TOLERANCE_SECONDS) {
    console.error("Stripe webhook timestamp outside tolerance window");
    return false;
  }

  // Compute expected signature: HMAC-SHA256(timestamp + "." + rawBody, webhookSecret)
  const signingPayload = `${timestamp}.${rawBody}`;
  const encoder = new TextEncoder();
  const keyData = encoder.encode(webhookSecret);
  const messageData = encoder.encode(signingPayload);

  const cryptoKey = await crypto.subtle.importKey(
    "raw",
    keyData,
    { name: "HMAC", hash: "SHA-256" },
    false,
    ["sign"],
  );

  const signatureBytes = await crypto.subtle.sign("HMAC", cryptoKey, messageData);

  // Convert to hex string
  const expectedSig = Array.from(new Uint8Array(signatureBytes))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("");

  // Constant-time comparison to prevent timing attacks
  function timingSafeEqual(a: string, b: string): boolean {
    if (a.length !== b.length) return false;
    const encoder = new TextEncoder();
    const bufA = encoder.encode(a);
    const bufB = encoder.encode(b);
    let result = 0;
    for (let i = 0; i < bufA.length; i++) {
      result |= bufA[i] ^ bufB[i];
    }
    return result === 0;
  }

  // Compare against any v1 signature in the header (Stripe rotates secrets)
  const receivedSigs = signatureParts.map((p) => p.slice(3));
  return receivedSigs.some((sig) => timingSafeEqual(sig, expectedSig));
}

// ─── License helpers ──────────────────────────────────────────────────────────

async function createLicense(
  email: string,
  tier: string,
  stripe_subscription_id: string,
  stripe_customer_id: string,
): Promise<void> {
  const createLicenseUrl = Deno.env.get("CREATE_LICENSE_FUNCTION_URL");
  const supabaseServiceKey = Deno.env.get("SUPABASE_SERVICE_ROLE_KEY");

  if (!createLicenseUrl) {
    throw new Error("CREATE_LICENSE_FUNCTION_URL secret is not set");
  }

  const response = await fetch(createLicenseUrl, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      "Authorization": `Bearer ${supabaseServiceKey}`,
    },
    body: JSON.stringify({
      email,
      tier,
      stripe_subscription_id,
      stripe_customer_id,
    }),
  });

  if (!response.ok) {
    const errBody = await response.text();
    throw new Error(`create-license failed: ${response.status} ${errBody}`);
  }
}

async function revokeLicense(
  supabase: ReturnType<typeof createClient>,
  stripeSubscriptionId: string,
): Promise<void> {
  const { error } = await supabase
    .from("licenses")
    .update({ revoked_at: new Date().toISOString() })
    .eq("stripe_subscription_id", stripeSubscriptionId)
    .is("revoked_at", null); // only revoke if not already revoked

  if (error) {
    throw new Error(`Failed to revoke license: ${error.message}`);
  }
}

async function renewLicense(
  supabase: ReturnType<typeof createClient>,
  stripeSubscriptionId: string,
  email: string,
  tier: string,
  stripeCustomerId: string,
): Promise<void> {
  // Revoke old license first
  await revokeLicense(supabase, stripeSubscriptionId);

  // Create a new license with a fresh 30-day expiry
  await createLicense(email, tier, stripeSubscriptionId, stripeCustomerId);
}

// ─── Event handlers ───────────────────────────────────────────────────────────

async function handleCheckoutSessionCompleted(
  supabase: ReturnType<typeof createClient>,
  session: Record<string, unknown>,
): Promise<void> {
  // Extract customer information from the completed checkout session
  const email = (session.customer_details as Record<string, unknown>)
    ?.email as string;
  const stripeCustomerId = session.customer as string;
  const stripeSubscriptionId = session.subscription as string;

  // Tier is passed via metadata on the payment link or checkout session
  const metadata = (session.metadata as Record<string, string>) || {};
  const tier = metadata.tier;

  if (!email || !stripeSubscriptionId || !stripeCustomerId) {
    console.error("checkout.session.completed: missing required fields", {
      email,
      stripeSubscriptionId,
      stripeCustomerId,
    });
    return;
  }

  if (!tier || !["pro", "enterprise"].includes(tier)) {
    console.error(
      "checkout.session.completed: invalid or missing tier in metadata",
      { tier },
    );
    return;
  }

  console.log(`checkout.session.completed: creating ${tier} license for ${email}`);
  await createLicense(email, tier, stripeSubscriptionId, stripeCustomerId);
  console.log(`License created for ${email} (${tier})`);
}

async function handleSubscriptionDeleted(
  supabase: ReturnType<typeof createClient>,
  subscription: Record<string, unknown>,
): Promise<void> {
  const stripeSubscriptionId = subscription.id as string;
  console.log(`customer.subscription.deleted: revoking license for sub ${stripeSubscriptionId}`);
  await revokeLicense(supabase, stripeSubscriptionId);
  console.log(`License revoked for subscription ${stripeSubscriptionId}`);
}

async function handleSubscriptionUpdated(
  supabase: ReturnType<typeof createClient>,
  subscription: Record<string, unknown>,
): Promise<void> {
  const stripeSubscriptionId = subscription.id as string;
  const status = subscription.status as string;
  const stripeCustomerId = subscription.customer as string;
  const metadata = (subscription.metadata as Record<string, string>) || {};
  const tier = metadata.tier;

  console.log(
    `customer.subscription.updated: sub ${stripeSubscriptionId} status=${status}`,
  );

  // Revoke on cancellation or non-payment
  if (status === "past_due" || status === "canceled" || status === "unpaid") {
    await revokeLicense(supabase, stripeSubscriptionId);
    console.log(
      `License revoked for subscription ${stripeSubscriptionId} (status: ${status})`,
    );
    return;
  }

  // Renew on reactivation (e.g. past_due resolved, manual renewal)
  if (status === "active") {
    // Fetch the license to get customer email and tier
    const { data: licenses, error } = await supabase
      .from("licenses")
      .select("email, tier, stripe_customer_id")
      .eq("stripe_subscription_id", stripeSubscriptionId)
      .limit(1);

    if (error || !licenses || licenses.length === 0) {
      console.error(
        `subscription.updated: no license found for sub ${stripeSubscriptionId}`,
        error,
      );
      return;
    }

    const existingLicense = licenses[0];
    const effectiveTier = tier || existingLicense.tier;
    const effectiveEmail = existingLicense.email;
    const effectiveCustomerId = stripeCustomerId || existingLicense.stripe_customer_id;

    await renewLicense(
      supabase,
      stripeSubscriptionId,
      effectiveEmail,
      effectiveTier,
      effectiveCustomerId,
    );
    console.log(
      `License renewed for ${effectiveEmail} (${effectiveTier}, sub ${stripeSubscriptionId})`,
    );
  }
  // All other status transitions (trialing, incomplete, etc.) — ignore
}

// ─── Main handler ─────────────────────────────────────────────────────────────

serve(async (req: Request) => {
  if (req.method !== "POST") {
    return new Response(JSON.stringify({ error: "Method not allowed" }), {
      status: 405,
      headers: { "Content-Type": "application/json" },
    });
  }

  // Read raw body (needed for signature verification)
  const rawBody = await req.text();
  const signatureHeader = req.headers.get("stripe-signature") || "";

  const webhookSecret = Deno.env.get("STRIPE_WEBHOOK_SECRET");
  if (!webhookSecret) {
    console.error("STRIPE_WEBHOOK_SECRET is not configured");
    return new Response(JSON.stringify({ error: "Webhook secret not configured" }), {
      status: 500,
      headers: { "Content-Type": "application/json" },
    });
  }

  // Verify Stripe signature
  const isValid = await verifyStripeSignature(rawBody, signatureHeader, webhookSecret);
  if (!isValid) {
    console.error("Invalid Stripe webhook signature");
    return new Response(JSON.stringify({ error: "Invalid signature" }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  }

  // Parse event
  let event: { type: string; data: { object: Record<string, unknown> } };
  try {
    event = JSON.parse(rawBody);
  } catch {
    return new Response(JSON.stringify({ error: "Invalid JSON" }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  }

  // Initialize Supabase service-role client
  const supabaseUrl = Deno.env.get("SUPABASE_URL");
  const supabaseServiceKey = Deno.env.get("SUPABASE_SERVICE_ROLE_KEY");

  if (!supabaseUrl || !supabaseServiceKey) {
    console.error("Missing Supabase environment variables");
    return new Response(JSON.stringify({ error: "Database not configured" }), {
      status: 500,
      headers: { "Content-Type": "application/json" },
    });
  }

  const supabase = createClient(supabaseUrl, supabaseServiceKey, {
    auth: { persistSession: false },
  });

  const eventObject = event.data.object;

  try {
    switch (event.type) {
      case "checkout.session.completed":
        await handleCheckoutSessionCompleted(supabase, eventObject);
        break;

      case "customer.subscription.deleted":
        await handleSubscriptionDeleted(supabase, eventObject);
        break;

      case "customer.subscription.updated":
        await handleSubscriptionUpdated(supabase, eventObject);
        break;

      default:
        // Acknowledge all other events without processing
        console.log(`Unhandled Stripe event type: ${event.type}`);
        break;
    }
  } catch (err) {
    // Return 500 so Stripe retries the event — transient errors should be retried
    console.error(`Error processing Stripe event ${event.type}:`, err);
    return new Response(
      JSON.stringify({ error: "Processing error — check logs" }),
      { status: 500, headers: { "Content-Type": "application/json" } },
    );
  }

  return new Response(JSON.stringify({ received: true }), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
});
