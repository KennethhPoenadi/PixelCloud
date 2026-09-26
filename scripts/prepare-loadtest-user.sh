#!/usr/bin/env bash
# Create USERS load-test accounts (loadtest-1@…, loadtest-2@…) on the Business
# plan (no quota) so the load test measures the system, not the quota check.
# One account per k6 VU keeps each under the per-user rate limit.
set -euo pipefail
cd "$(dirname "$0")/.."
set -a; [ -f .env ] && . ./.env; set +a
BASE="http://localhost:${HTTP_PORT:-8080}/api/v1"
USERS="${USERS:-10}"
PASSWORD="${PASSWORD:-loadtest-password}"
psql_q() { docker exec postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -tAc "$1"; }

for i in $(seq 1 "${USERS}"); do
  email="loadtest-${i}@pixelcloud.local"
  if [ -z "$(psql_q "SELECT 1 FROM users WHERE email = '${email}'")" ]; then
    curl -s -o /dev/null -X POST "${BASE}/auth/register" -H 'Content-Type: application/json' \
      -d "{\"email\":\"${email}\",\"password\":\"${PASSWORD}\"}"
  fi
done
psql_q "UPDATE users SET plan_id = (SELECT id FROM plans WHERE code = 'business') WHERE email LIKE 'loadtest-%@pixelcloud.local'" >/dev/null
echo "load-test users ready: loadtest-{1..${USERS}}@pixelcloud.local"
