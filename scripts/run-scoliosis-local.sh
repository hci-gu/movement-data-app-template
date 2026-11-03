#!/usr/bin/env bash
set -euo pipefail

# A dedicated database and keyring; never loads the template's api/.env.
study_root="$(cd "$(dirname "$0")/.." && pwd)"
study_local="$study_root/api/.local/scoliosis"
mkdir -p "$study_local/secrets"
if [[ ! -f "$study_local/secrets/study-keys.json" ]]; then
  python3 "$study_root/scripts/create-study-secrets.py" --out "$study_local/secrets/study-keys.json"
fi
export BANKID_ENVIRONMENT=test
export BANKID_APP_IDENTIFIER=se.gu.skoliosrorelse
export BANKID_RETURN_URL=researchsteps://bankid/return
export BANKID_CERT_FILE="$study_root/api/secrets/client.pem"
export BANKID_KEY_FILE="$study_root/api/secrets/client-key.pem"
export BANKID_CA_FILE="$study_root/api/secrets/bankid-server-ca.pem"
export STUDY_SECRETS_FILE="$study_local/secrets/study-keys.json"
# A public HTTPS tunnel is required to open guardian links on another device.
# Set BANKID_PUBLIC_URL to that origin before launching when testing guardians.
export BANKID_PUBLIC_URL="${BANKID_PUBLIC_URL:-}"
export TRUSTED_PROXY_CIDRS="${TRUSTED_PROXY_CIDRS:-127.0.0.1/32,::1/128}"
unset STUDY_ENCRYPTION_KEYS STUDY_ACTIVE_ENCRYPTION_KEY
cd "$study_local"
exec "$study_root/api/app" serve --http=127.0.0.1:8091 --dir="$study_local/pb_data"
