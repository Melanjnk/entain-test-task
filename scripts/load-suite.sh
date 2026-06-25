#!/usr/bin/env bash
set -euo pipefail

DURATION="${DURATION:-90s}"
PAUSE="${PAUSE:-15s}"
BASE_URL="${BASE_URL:-http://localhost:8080}"
STRESS_USER="${STRESS_USER:-1}"

run_step() {
  local rate="$1"
  local label="$2"
  local user="${3:-0}"
  local extra=()
  if [[ "$user" != "0" ]]; then
    extra=(-user "$user")
  fi

  echo
  echo "############################################"
  echo "# ${label}: ${rate} RPS for ${DURATION}"
  [[ "$user" != "0" ]] && echo "# hot user ${user} (FOR UPDATE serialization)"
  echo "############################################"
  go run ./cmd/loadgen -rate "$rate" -duration "$DURATION" -label "$label" -url "$BASE_URL" "${extra[@]}"
  echo "Pause ${PAUSE}..."
  sleep "$PAUSE"
}

curl -sf "${BASE_URL}/healthz" >/dev/null || {
  echo "Service not up. Run: docker compose up -d --build"
  exit 1
}

curl -sf "http://localhost:9090/-/healthy" >/dev/null || {
  echo "WARN: Prometheus not healthy — Grafana panels will be empty."
  echo "Run: docker compose up -d prometheus grafana"
}

echo "Capacity suite — Grafana: http://localhost:3000/d/entain-balance-slo"

run_step 25 "task-baseline" 0
run_step 40 "headroom" 0
run_step 100 "stress-hot" "$STRESS_USER"
run_step 200 "stress-hot" "$STRESS_USER"
run_step 500 "saturation-hot" "$STRESS_USER"

echo
echo "Expect: 25–40 WITHIN_SLO | 200+ hot user → rising p95 (GRACEFUL_DEGRADATION) | 500 → SATURATED"
