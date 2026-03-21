// create-license/index.ts
// Supabase Edge Function (Deno) — generates a signed JWT license key for MCP Zero-Trust Proxy.
//
// Input (JSON body):
//   { email: string, tier: "pro" | "enterprise", stripe_subscription_id: string, stripe_customer_id: string }
//
// Output (JSON):
//   { license_key: string, expires_at: string }
//
// Secrets required:
//   LICENSE_SIGNING_KEY — PKCS#8 PEM-encoded ECDSA P-256 private key
//   SUPABASE_URL        — auto-provided by Supabase
//   SUPABASE_SERVICE_ROLE_KEY — auto-provided by Supabase

import { serve } from "https://deno.land/std@0.168.0/http/server.ts";
import { createClient } from "https://esm.sh/@supabase/supabase-js@2";

// Tier limits: 0 means unlimited
const TIER_LIMITS: Record<string, { max_upstreams: number; max_rpm: number }> =
  {
    pro: { max_upstreams: 5, max_rpm: 200 },
    enterprise: { max_upstreams: 0, max_rpm: 0 },
  };

const ISSUER = "mcpzerotrust.dev";
const TOKEN_LIFETIME_DAYS = 30;

// ─── PEM helpers ────────────────────────────────────────────────────────────

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

// ─── JWT helpers ─────────────────────────────────────────────────────────────

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

// ─── Handler ─────────────────────────────────────────────────────────────────

serve(async (req: Request) => {
  // Verify caller is authorized via service role key
  const authHeader = req.headers.get("Authorization");
  const serviceRoleKey = Deno.env.get("SUPABASE_SERVICE_ROLE_KEY");
  if (!serviceRoleKey || !authHeader || authHeader !== `Bearer ${serviceRoleKey}`) {
    return new Response(JSON.stringify({ error: "Unauthorized" }), {
      status: 401,
      headers: { "Content-Type": "application/json" },
    });
  }

  // Only accept POST
  if (req.method !== "POST") {
    return new Response(JSON.stringify({ error: "Method not allowed" }), {
      status: 405,
      headers: { "Content-Type": "application/json" },
    });
  }

  let body: {
    email?: string;
    tier?: string;
    stripe_subscription_id?: string;
    stripe_customer_id?: string;
  };

  try {
    body = await req.json();
  } catch {
    return new Response(JSON.stringify({ error: "Invalid JSON body" }), {
      status: 400,
      headers: { "Content-Type": "application/json" },
    });
  }

  const { email, tier, stripe_subscription_id, stripe_customer_id } = body;

  // Validate required fields
  if (!email || !tier || !stripe_subscription_id || !stripe_customer_id) {
    return new Response(
      JSON.stringify({
        error: "Missing required fields: email, tier, stripe_subscription_id, stripe_customer_id",
      }),
      { status: 400, headers: { "Content-Type": "application/json" } },
    );
  }

  // Validate tier
  if (!TIER_LIMITS[tier]) {
    return new Response(
      JSON.stringify({ error: `Invalid tier: ${tier}. Must be 'pro' or 'enterprise'` }),
      { status: 400, headers: { "Content-Type": "application/json" } },
    );
  }

  const limits = TIER_LIMITS[tier];

  // Load private signing key from secret
  const signingKeyPem = Deno.env.get("LICENSE_SIGNING_KEY");
  if (!signingKeyPem) {
    console.error("LICENSE_SIGNING_KEY secret is not set");
    return new Response(
      JSON.stringify({ error: "License signing key not configured" }),
      { status: 500, headers: { "Content-Type": "application/json" } },
    );
  }

  let privateKey: CryptoKey;
  try {
    privateKey = await importEcPrivateKey(signingKeyPem);
  } catch (err) {
    console.error("Failed to import signing key:", err);
    return new Response(
      JSON.stringify({ error: "Failed to load signing key" }),
      { status: 500, headers: { "Content-Type": "application/json" } },
    );
  }

  // Build JWT claims
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

  // Sign the JWT
  let licenseKey: string;
  try {
    licenseKey = await signJwt(header, payload, privateKey);
  } catch (err) {
    console.error("Failed to sign JWT:", err);
    return new Response(
      JSON.stringify({ error: "Failed to generate license key" }),
      { status: 500, headers: { "Content-Type": "application/json" } },
    );
  }

  // Insert into Supabase licenses table using service role client
  const supabaseUrl = Deno.env.get("SUPABASE_URL");
  const supabaseServiceKey = Deno.env.get("SUPABASE_SERVICE_ROLE_KEY");

  if (!supabaseUrl || !supabaseServiceKey) {
    console.error("Missing Supabase environment variables");
    return new Response(
      JSON.stringify({ error: "Database configuration missing" }),
      { status: 500, headers: { "Content-Type": "application/json" } },
    );
  }

  const supabase = createClient(supabaseUrl, supabaseServiceKey, {
    auth: { persistSession: false },
  });

  const { error: dbError } = await supabase.from("licenses").insert({
    email,
    tier,
    stripe_subscription_id,
    stripe_customer_id,
    license_key: licenseKey,
    max_upstreams: limits.max_upstreams,
    max_rpm: limits.max_rpm,
    issued_at: new Date().toISOString(),
    expires_at: expiresAt,
  });

  if (dbError) {
    console.error("Failed to insert license:", dbError);
    return new Response(
      JSON.stringify({ error: "Failed to store license", details: dbError.message }),
      { status: 500, headers: { "Content-Type": "application/json" } },
    );
  }

  return new Response(
    JSON.stringify({ license_key: licenseKey, expires_at: expiresAt }),
    { status: 200, headers: { "Content-Type": "application/json" } },
  );
});
