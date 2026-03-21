#!/usr/bin/env bash
# setup-stripe-products.sh
# Creates Stripe products, recurring prices, and payment links for MCP Zero-Trust Proxy.
#
# Usage: STRIPE_SECRET_KEY=sk_test_... ./scripts/setup-stripe-products.sh
#
# Requires:
#   - stripe CLI (installed and available in PATH)
#   - STRIPE_SECRET_KEY environment variable (test or live key)
#
# Outputs:
#   - Product IDs
#   - Price IDs (set these as STRIPE_PRICE_PRO and STRIPE_PRICE_ENTERPRISE env vars)
#   - Payment link URLs (add these to the landing page checkout buttons)
#
# Run ONCE per environment (test / live). Re-running creates duplicate products.

set -euo pipefail

# ─── Preflight ────────────────────────────────────────────────────────────────

if ! command -v stripe &>/dev/null; then
  echo "ERROR: stripe CLI is not installed or not in PATH." >&2
  echo "       Install from: https://stripe.com/docs/stripe-cli" >&2
  exit 1
fi

if [ -z "${STRIPE_SECRET_KEY:-}" ]; then
  echo "ERROR: STRIPE_SECRET_KEY environment variable is not set." >&2
  echo "       Export it before running: export STRIPE_SECRET_KEY=sk_test_..." >&2
  exit 1
fi

# Detect test vs live mode
if [[ "$STRIPE_SECRET_KEY" == sk_live_* ]]; then
  MODE="LIVE"
  MODE_COLOR="\033[0;31m"  # Red for live
else
  MODE="TEST"
  MODE_COLOR="\033[0;33m"  # Yellow for test
fi

RESET="\033[0m"

echo "============================================================"
echo -e "  MCP Zero-Trust Proxy — Stripe Setup (${MODE_COLOR}${MODE} MODE${RESET})"
echo "============================================================"
echo ""

if [ "$MODE" = "LIVE" ]; then
  echo -e "${MODE_COLOR}WARNING: You are creating LIVE Stripe products. Real money will be involved.${RESET}"
  echo ""
  read -r -p "Continue with LIVE mode? (type 'yes' to confirm): " CONFIRM
  if [ "$CONFIRM" != "yes" ]; then
    echo "Aborted."
    exit 0
  fi
  echo ""
fi

# ─── Helper: call stripe API ─────────────────────────────────────────────────

stripe_api() {
  local method="$1"
  local endpoint="$2"
  shift 2
  curl -sS -X "$method" "https://api.stripe.com/v1/${endpoint}" \
    -u "${STRIPE_SECRET_KEY}:" \
    -H "Content-Type: application/x-www-form-urlencoded" \
    "$@"
}

# ─── Create Pro product ───────────────────────────────────────────────────────

echo "Creating Pro product..."
PRO_PRODUCT=$(stripe_api POST products \
  -d "name=MCP Zero-Trust Proxy Pro" \
  -d "description=Protect up to 5 MCP servers with OAuth 2.1 PKCE, RBAC, audit logging, and rate limiting. Up to 200 req/min." \
  -d "metadata[tier]=pro")

PRO_PRODUCT_ID=$(echo "$PRO_PRODUCT" | grep -o '"id": "prod_[^"]*"' | head -1 | awk -F'"' '{print $4}')
if [ -z "$PRO_PRODUCT_ID" ]; then
  echo "ERROR: Failed to create Pro product. Response:"
  echo "$PRO_PRODUCT"
  exit 1
fi
echo "  Pro product ID: $PRO_PRODUCT_ID"

# ─── Create Enterprise product ────────────────────────────────────────────────

echo "Creating Enterprise product..."
ENT_PRODUCT=$(stripe_api POST products \
  -d "name=MCP Zero-Trust Proxy Enterprise" \
  -d "description=Unlimited MCP servers, unlimited req/min. OAuth 2.1 PKCE, RBAC, audit logging, rate limiting, and priority support." \
  -d "metadata[tier]=enterprise")

ENT_PRODUCT_ID=$(echo "$ENT_PRODUCT" | grep -o '"id": "prod_[^"]*"' | head -1 | awk -F'"' '{print $4}')
if [ -z "$ENT_PRODUCT_ID" ]; then
  echo "ERROR: Failed to create Enterprise product. Response:"
  echo "$ENT_PRODUCT"
  exit 1
fi
echo "  Enterprise product ID: $ENT_PRODUCT_ID"

# ─── Create Pro price ($49/mo) ────────────────────────────────────────────────

echo "Creating Pro price (\$49/mo)..."
PRO_PRICE=$(stripe_api POST prices \
  -d "product=${PRO_PRODUCT_ID}" \
  -d "unit_amount=4900" \
  -d "currency=usd" \
  -d "recurring[interval]=month" \
  -d "nickname=Pro Monthly" \
  -d "metadata[tier]=pro")

PRO_PRICE_ID=$(echo "$PRO_PRICE" | grep -o '"id": "price_[^"]*"' | head -1 | awk -F'"' '{print $4}')
if [ -z "$PRO_PRICE_ID" ]; then
  echo "ERROR: Failed to create Pro price. Response:"
  echo "$PRO_PRICE"
  exit 1
fi
echo "  Pro price ID: $PRO_PRICE_ID"

# ─── Create Enterprise price ($199/mo) ───────────────────────────────────────

echo "Creating Enterprise price (\$199/mo)..."
ENT_PRICE=$(stripe_api POST prices \
  -d "product=${ENT_PRODUCT_ID}" \
  -d "unit_amount=19900" \
  -d "currency=usd" \
  -d "recurring[interval]=month" \
  -d "nickname=Enterprise Monthly" \
  -d "metadata[tier]=enterprise")

ENT_PRICE_ID=$(echo "$ENT_PRICE" | grep -o '"id": "price_[^"]*"' | head -1 | awk -F'"' '{print $4}')
if [ -z "$ENT_PRICE_ID" ]; then
  echo "ERROR: Failed to create Enterprise price. Response:"
  echo "$ENT_PRICE"
  exit 1
fi
echo "  Enterprise price ID: $ENT_PRICE_ID"

# ─── Create payment links ─────────────────────────────────────────────────────

echo "Creating Pro payment link..."
PRO_LINK=$(stripe_api POST payment_links \
  -d "line_items[0][price]=${PRO_PRICE_ID}" \
  -d "line_items[0][quantity]=1" \
  -d "metadata[tier]=pro" \
  -d "after_completion[type]=redirect" \
  -d "after_completion[redirect][url]=https://mcpzerotrust.dev/checkout-success?tier=pro")

PRO_LINK_URL=$(echo "$PRO_LINK" | grep -o '"url": "https://buy.stripe.com/[^"]*"' | head -1 | awk -F'"' '{print $4}')
if [ -z "$PRO_LINK_URL" ]; then
  # Try alternate format
  PRO_LINK_URL=$(echo "$PRO_LINK" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('url',''))" 2>/dev/null || echo "")
fi
echo "  Pro payment link: ${PRO_LINK_URL:-[check Stripe Dashboard]}"

echo "Creating Enterprise payment link..."
ENT_LINK=$(stripe_api POST payment_links \
  -d "line_items[0][price]=${ENT_PRICE_ID}" \
  -d "line_items[0][quantity]=1" \
  -d "metadata[tier]=enterprise" \
  -d "after_completion[type]=redirect" \
  -d "after_completion[redirect][url]=https://mcpzerotrust.dev/checkout-success?tier=enterprise")

ENT_LINK_URL=$(echo "$ENT_LINK" | grep -o '"url": "https://buy.stripe.com/[^"]*"' | head -1 | awk -F'"' '{print $4}')
if [ -z "$ENT_LINK_URL" ]; then
  ENT_LINK_URL=$(echo "$ENT_LINK" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('url',''))" 2>/dev/null || echo "")
fi
echo "  Enterprise payment link: ${ENT_LINK_URL:-[check Stripe Dashboard]}"

# ─── Summary ──────────────────────────────────────────────────────────────────

echo ""
echo "============================================================"
echo "  DONE — Save these values"
echo "============================================================"
echo ""
echo "Set these environment variables for the billing backend:"
echo ""
echo "  export STRIPE_PRICE_PRO=${PRO_PRICE_ID}"
echo "  export STRIPE_PRICE_ENTERPRISE=${ENT_PRICE_ID}"
echo ""
echo "Add to Supabase secrets (supabase secrets set ...):"
echo "  STRIPE_PRICE_PRO=${PRO_PRICE_ID}"
echo "  STRIPE_PRICE_ENTERPRISE=${ENT_PRICE_ID}"
echo ""
echo "Checkout links to add to the landing page:"
echo "  Pro:        ${PRO_LINK_URL:-[check Stripe Dashboard -> Payment links]}"
echo "  Enterprise: ${ENT_LINK_URL:-[check Stripe Dashboard -> Payment links]}"
echo ""
echo "Next steps:"
echo "  1. Deploy Supabase Edge Functions (supabase functions deploy stripe-webhook)"
echo "  2. Register the webhook endpoint in Stripe Dashboard:"
echo "     URL: https://\$(supabase status | grep API).supabase.co/functions/v1/stripe-webhook"
echo "     Events: checkout.session.completed, customer.subscription.deleted, customer.subscription.updated"
echo "  3. Copy the signing secret and set: supabase secrets set STRIPE_WEBHOOK_SECRET=whsec_..."
echo ""
