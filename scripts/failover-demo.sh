#!/usr/bin/env bash
# HA demo: run a load test, stop one API node, crash one worker mid-job, and
# show that the service keeps answering and the job is reclaimed by the other
# worker. Watch Grafana (http://localhost:3000) while this runs.
set -euo pipefail
cd "$(dirname "$0")/.."

# shellcheck disable=SC1091
set -a; [ -f .env ] && . ./.env; set +a
PORT="${HTTP_PORT:-8080}"
BASE="http://localhost:${PORT}/api/v1"
PASSWORD="loadtest-password"
DURATION="${DURATION:-4m}"

say() { printf '\n\033[1;34m==> %s\033[0m\n' "$*"; }
psql_q() { docker exec postgres psql -U "${POSTGRES_USER}" -d "${POSTGRES_DB}" -tAc "$1"; }
healthz() { curl -s --max-time 2 "${BASE}/healthz" || echo '{"error":"no response"}'; }

cleanup() {
  say "Restoring nodes"
  docker start api-1 worker-1 >/dev/null 2>&1 || true
}
trap cleanup EXIT

say "Preparing load-test users (Business plan, no quota limit)"
USERS="${VUS:-10}" PASSWORD="${PASSWORD}" ./scripts/prepare-loadtest-user.sh

say "Starting k6 load test for ${DURATION} in the background"
LOG="$(mktemp -t pixelcloud-k6.XXXXXX)"
docker run --rm --network pixelcloud_default -v "$PWD":/work -w /work/scripts \
  -e BASE_URL=http://nginx -e PASSWORD="${PASSWORD}" -e DURATION="${DURATION}" \
  -e VUS="${VUS:-10}" -e JOB_RATIO="${JOB_RATIO:-0.3}" \
  grafana/k6:latest run --quiet loadtest.js >"${LOG}" 2>&1 &
K6_PID=$!
sleep 30

say "Both API nodes answer (round-robin):"
for _ in 1 2 3 4; do healthz; echo; done

say "docker stop api-1"
docker stop api-1 >/dev/null
sleep 3
say "Requests keep succeeding, now all from api-2:"
for _ in 1 2 3 4; do healthz; echo; done
sleep 30

say "Waiting for worker-1 to pick up a job, then killing it mid-job (simulated crash)"
JOB=""
for _ in $(seq 1 120); do
  JOB="$(psql_q "SELECT id FROM jobs WHERE status = 'processing' AND worker_id = 'worker-1' LIMIT 1")"
  [ -n "${JOB}" ] && break
  sleep 0.25
done
if [ -n "${JOB}" ]; then
  docker kill worker-1 >/dev/null
  echo "killed worker-1 while it was processing job ${JOB}"
  say "Waiting for another worker to reclaim it (RECLAIM_IDLE_SECONDS=${RECLAIM_IDLE_SECONDS:-15})"
  for _ in $(seq 1 90); do
    ROW="$(psql_q "SELECT status || ' by ' || coalesce(worker_id, '-') || ' (attempts ' || attempts || ')' FROM jobs WHERE id = '${JOB}'")"
    echo "  job ${JOB:0:8}: ${ROW}"
    case "${ROW}" in done*worker-2*) break ;; esac
    sleep 1
  done
else
  echo "worker-1 did not get a job in time; killing it anyway"
  docker kill worker-1 >/dev/null
fi

sleep 20
say "Bringing api-1 and worker-1 back"
docker start api-1 worker-1 >/dev/null
trap - EXIT

say "Waiting for k6 to finish…"
wait "${K6_PID}" || true
tail -n 40 "${LOG}"
say "Done. Open Grafana → PixelCloud — Overview to see the traffic shift and the reclaimed job."
