#!/usr/bin/env bash
# generate-license-keypair.sh
# Generates an ECDSA P-256 keypair for signing MCP Zero-Trust Proxy license JWTs.
#
# Usage: ./scripts/generate-license-keypair.sh
#
# Outputs:
#   - private_key.pem  (NEVER commit this — store in Supabase secrets)
#   - public_key.pem   (safe to commit — embed in proxy config)
#
# Run this ONCE in a secure environment. Store the private key as a Supabase
# Edge Function secret named LICENSE_SIGNING_KEY.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
OUT_DIR="${PROJECT_DIR}/keys"

echo "============================================================"
echo "  MCP Zero-Trust Proxy — License Keypair Generator"
echo "============================================================"
echo ""

# Check openssl is available
if ! command -v openssl &>/dev/null; then
  echo "ERROR: openssl is required but not found in PATH." >&2
  exit 1
fi

# Create output directory (gitignored)
mkdir -p "$OUT_DIR"

PRIVATE_KEY_FILE="${OUT_DIR}/private_key.pem"
PUBLIC_KEY_FILE="${OUT_DIR}/public_key.pem"

echo "Generating ECDSA P-256 keypair..."
echo ""

# Generate EC private key (P-256 / prime256v1)
openssl ecparam -genkey -name prime256v1 -noout -out "$PRIVATE_KEY_FILE" 2>/dev/null

# Convert to PKCS#8 format (required for jose/JWT libraries)
openssl pkcs8 -topk8 -nocrypt -in "$PRIVATE_KEY_FILE" -out "${PRIVATE_KEY_FILE}.pkcs8" 2>/dev/null
mv "${PRIVATE_KEY_FILE}.pkcs8" "$PRIVATE_KEY_FILE"

# Extract public key
openssl ec -in "$PRIVATE_KEY_FILE" -pubout -out "$PUBLIC_KEY_FILE" 2>/dev/null

echo "Keys written to:"
echo "  Private: $PRIVATE_KEY_FILE"
echo "  Public:  $PUBLIC_KEY_FILE"
echo ""
echo "============================================================"
echo "  NEXT STEPS"
echo "============================================================"
echo ""
echo "1. Set the PRIVATE KEY as a Supabase Edge Function secret:"
echo ""
echo "   supabase secrets set LICENSE_SIGNING_KEY=\"\$(cat keys/private_key.pem)\""
echo "   # Or via the Supabase Dashboard: Settings -> Edge Functions -> Secrets"
echo ""
echo "2. Copy the PUBLIC KEY into your proxy config YAML:"
echo ""
cat "$PUBLIC_KEY_FILE"
echo ""
echo "   Add to configs/config.yaml under 'license.public_key_pem'"
echo ""
echo "3. Add keys/ to .gitignore (if not already present):"
echo "   echo 'keys/' >> .gitignore"
echo ""
echo "4. NEVER commit private_key.pem to git."
echo ""
echo "============================================================"
echo "  PRIVATE KEY (for Supabase secret — handle with care)"
echo "============================================================"
echo ""
cat "$PRIVATE_KEY_FILE"
echo ""
echo "============================================================"
echo ""
echo "Done. Keep private_key.pem secure — delete it once stored in Supabase."
