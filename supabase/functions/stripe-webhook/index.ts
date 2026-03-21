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
//   LICENSE_SIGNING_KEY         — PKCS#8 PEM-encoded ECDSA P-256 private key (for renewals)
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

// ─── JWT helpers (for in-place license renewal) ───────────────────────────────

// Tier limits: 0 means unlimited
const TIER_LIMITS: Record<string, { max_upstreams: number; max_rpm: number }> = {
  pro: { max_upstreams: 5, max_rpm: 200 },
  enterprise: { max_upstreams: 0, max_rpm: 0 },
};

const ISSUER = "mcpzerotrust.dev";
const TOKEN_LIFETIME_DAYS = 30;

function pemToArrayBuffer(pem: string): ArrayBuffer {
  const lines = pem
    .split("\n")
    .filter((l) => !l.startsWith("-----"))
    .join("");
  const binary = atob(lines);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes.buffer;
}

async function importEcPrivateKey(pkcs8Pem: string): Promise<CryptoKey> {
  const keyData = pemToArrayBuffer(pkcs8Pem);
  return await crypto.subtle.importKey(
    "pkcs8",
    keyData,
    { name: "ECDSA", namedCurve: "P-256" },
    false,
    ["sign"],
  );
}

function base64urlEncode(data: Uint8Array | ArrayBuffer): string {
  const bytes =
    data instanceof ArrayBuffer ? new Uint8Array(data) : new Uint8Array(data);
  let str = "";
  for (const b of bytes) {
    str += String.fromCharCode(b);
  }
  return btoa(str).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

function encodeJsonPart(obj: Record<string, unknown>): string {
  return base64urlEncode(new TextEncoder().encode(JSON.stringify(obj)));
}

async function signJwt(
  header: Record<string, unknown>,
  payload: Record<string, unknown>,
  privateKey: CryptoKey,
): Promise<string> {
  const encodedHeader = encodeJsonPart(header);
  const encodedPayload = encodeJsonPart(payload);
  const signingInput = `${encodedHeader}.${encodedPayload}`;

  const signature = await crypto.subtle.sign(
    { name: "ECDSA", hash: { name: "SHA-256" } },
    privateKey,
    new TextEncoder().encode(signingInput),
  );

  return `${signingInput}.${base64urlEncode(signature)}`;
}

async function generateLicenseJwt(email: string, tier: string): Promise<{ licenseKey: string; expiresAt: string }> {
  const signingKeyPem = Deno.env.get("LICENSE_SIGNING_KEY");
  if (!signingKeyPem) {
    throw new Error("LICENSE_SIGNING_KEY secret is not set");
  }

  const privateKey = await importEcPrivateKey(signingKeyPem);
  const limits = TIER_LIMITS[tier] ?? { max_upstreams: 5, max_rpm: 200 };

  const now = Math.floor(Date.now() / 1000);
  const exp = now + TOKEN_LIFETIME_DAYS * 24 * 60 * 60;
  const expiresAt = new Date(exp * 1000).toISOString();

  const header = { alg: "ES256", typ: "JWT" };
  const payload = {
    iss: ISSUER,
    sub: email,
    iat: now,
    exp,
    tier,
    max_upstreams: limits.max_upstreams,
    max_rpm: limits.max_rpm,
  };

  const licenseKey = await signJwt(header, payload, privateKey);
  return { licenseKey, expiresAt };
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

// renewLicense updates the existing license row in-place with a fresh JWT and expiry.
// This is atomic — there is no window where the customer has no valid license.
// It avoids the unique constraint conflict that would arise from revoke-then-insert,
// and eliminates the coverage gap that revoke-then-create would cause.
async function renewLicense(
  supabase: ReturnType<typeof createClient>,
  stripeSubscriptionId: string,
  email: string,
  tier: string,
): Promise<void> {
  // Generate a fresh JWT (same signing path as create-license)
  const { licenseKey, expiresAt } = await generateLicenseJwt(email, tier);
  const limits = TIER_LIMITS[tier] ?? { max_upstreams: 5, max_rpm: 200 };

  // Update in-place: clears revoked_at, refreshes key + expiry atomically.
  // No INSERT means no unique-constraint conflict and no coverage gap.
  const { error } = await supabase
    .from("licenses")
    .update({
      license_key: licenseKey,
      tier,
      max_upstreams: limits.max_upstreams,
      max_rpm: limits.max_rpm,
      expires_at: expiresAt,
      revoked_at: null,
      issued_at: new Date().toISOString(),
    })
    .eq("stripe_subscription_id", stripeSubscriptionId);

  if (error) {
    throw new Error(`Failed to renew license: ${error.message}`);
  }
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

    await renewLicense(
      supabase,
      stripeSubscriptionId,
      effectiveEmail,
      effectiveTier,
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
  let event: { id: string; type: string; data: { object: Record<string, unknown> } };
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

  // ── Idempotency guard: skip duplicate Stripe event deliveries ──────────────
  // Stripe can deliver the same event multiple times (retries, network issues).
  // We check for the event ID in stripe_events before processing, and record it
  // after success, so duplicate deliveries are safely ignored.
  const { data: existingEvent } = await supabase
    .from("stripe_events")
    .select("event_id")
    .eq("event_id", event.id)
    .maybeSingle();

  if (existingEvent) {
    console.log(`Duplicate event ${event.id} — skipping`);
    return new Response(JSON.stringify({ received: true, duplicate: true }), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  }

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
    // Return 500 so Stripe retries the event — transient errors should be retried.
    // Do NOT record the event ID on failure so the retry will be processed.
    console.error(`Error processing Stripe event ${event.type}:`, err);
    return new Response(
      JSON.stringify({ error: "Processing error — check logs" }),
      { status: 500, headers: { "Content-Type": "application/json" } },
    );
  }

  // Record the processed event ID so duplicate deliveries are skipped
  await supabase.from("stripe_events").insert({
    event_id: event.id,
    event_type: event.type,
    processed_at: new Date().toISOString(),
  });

  return new Response(JSON.stringify({ received: true }), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
});
