import { serve } from "https://deno.land/std@0.168.0/http/server.ts";
import { createClient } from "https://esm.sh/@supabase/supabase-js@2";

const corsHeaders = {
  "Access-Control-Allow-Origin": "*",
  "Access-Control-Allow-Headers":
    "authorization, x-client-info, apikey, content-type",
};

serve(async (req: Request) => {
  if (req.method === "OPTIONS") {
    return new Response("ok", { headers: corsHeaders });
  }

  try {
    const url = new URL(req.url);
    const sessionId = url.searchParams.get("session_id");
    const emailParam = url.searchParams.get("email");

    if (!sessionId && !emailParam) {
      return new Response(
        JSON.stringify({ error: "Missing session_id or email parameter" }),
        {
          status: 400,
          headers: { ...corsHeaders, "Content-Type": "application/json" },
        },
      );
    }

    const supabase = createClient(
      Deno.env.get("SUPABASE_URL")!,
      Deno.env.get("SUPABASE_SERVICE_ROLE_KEY")!,
    );

    let customerEmail = emailParam;

    // If we have a session_id, look up the customer email from Stripe
    if (sessionId) {
      const stripeKey = Deno.env.get("STRIPE_SECRET_KEY");
      if (!stripeKey) {
        return new Response(
          JSON.stringify({ error: "Server configuration error" }),
          {
            status: 500,
            headers: { ...corsHeaders, "Content-Type": "application/json" },
          },
        );
      }

      const stripeRes = await fetch(
        `https://api.stripe.com/v1/checkout/sessions/${sessionId}`,
        { headers: { Authorization: `Bearer ${stripeKey}` } },
      );

      if (!stripeRes.ok) {
        return new Response(
          JSON.stringify({ error: "Invalid checkout session" }),
          {
            status: 400,
            headers: { ...corsHeaders, "Content-Type": "application/json" },
          },
        );
      }

      const session = await stripeRes.json();
      customerEmail =
        session.customer_details?.email || session.customer_email;

      if (!customerEmail) {
        return new Response(
          JSON.stringify({
            error: "No email found for this checkout session",
          }),
          {
            status: 404,
            headers: { ...corsHeaders, "Content-Type": "application/json" },
          },
        );
      }
    }

    // Look up the most recent active license for this email
    const { data: license, error: dbError } = await supabase
      .from("licenses")
      .select("license_key, tier, email, expires_at, issued_at")
      .eq("email", customerEmail)
      .is("revoked_at", null)
      .order("issued_at", { ascending: false })
      .limit(1)
      .single();

    if (dbError || !license) {
      return new Response(
        JSON.stringify({
          error: "License not found yet",
          message:
            "Your license is being generated. Please refresh in a few seconds.",
          retry: true,
        }),
        {
          status: 202,
          headers: { ...corsHeaders, "Content-Type": "application/json" },
        },
      );
    }

    return new Response(
      JSON.stringify({
        license_key: license.license_key,
        tier: license.tier,
        email: license.email,
        expires_at: license.expires_at,
      }),
      {
        status: 200,
        headers: { ...corsHeaders, "Content-Type": "application/json" },
      },
    );
  } catch (_err) {
    return new Response(
      JSON.stringify({ error: "Internal server error" }),
      {
        status: 500,
        headers: { ...corsHeaders, "Content-Type": "application/json" },
      },
    );
  }
});
