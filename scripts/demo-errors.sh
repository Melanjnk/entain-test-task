#!/usr/bin/env bash
# Generates a mix of HTTP statuses and rejection reasons for Grafana demo.
# Run with Grafana open: http://localhost:3000/d/entain-balance-slo
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
INTERVAL_SEC="${INTERVAL_SEC:-1}"
# Repeat the error mix so rate()[1m] panels stay visible longer in Grafana.
ROUNDS="${ROUNDS:-2}"

post_tx() {
  local user_id="$1" state="$2" amount="$3" tx_id="$4" source="${5:-game}"
  local code
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${BASE_URL}/user/${user_id}/transaction" \
    -H "Source-Type: ${source}" \
    -H "Content-Type: application/json" \
    -d "{\"state\":\"${state}\",\"amount\":\"${amount}\",\"transactionId\":\"${tx_id}\"}")
  echo "  POST user=${user_id} state=${state} amount=${amount} tx=${tx_id} -> HTTP ${code}"
}

echo "==> Demo errors for Grafana (${ROUNDS} round(s), interval ${INTERVAL_SEC}s)"
echo "    Open: http://localhost:3000/d/entain-balance-slo"
curl -sf "${BASE_URL}/healthz" >/dev/null || {
  echo "ERROR: service not reachable. Run: docker compose up -d"
  exit 1
}

DEMO_TX="demo-seed-$(date +%s)"

run_round() {
  local round="$1"
  local prefix="${DEMO_TX}-r${round}"

  echo
  echo "========== Round ${round}/${ROUNDS} =========="

  echo "==> 200 OK"
  post_tx 1 win 1.00 "${prefix}-ok"
  sleep "$INTERVAL_SEC"

  echo "==> duplicate_transaction (409)"
  post_tx 1 win 2.00 "${prefix}-dup"
  sleep "$INTERVAL_SEC"
  post_tx 1 win 2.00 "${prefix}-dup"
  sleep "$INTERVAL_SEC"

  echo "==> insufficient_funds (402) — lose more than any balance"
  post_tx 3 lose 99999.99 "${prefix}-402"
  sleep "$INTERVAL_SEC"

  echo "==> validation (400) — bad amount"
  post_tx 1 win "not-a-number" "${prefix}-val-amt"
  sleep "$INTERVAL_SEC"

  echo "==> validation (400) — bad state"
  post_tx 1 "not-win-or-lose" 1.00 "${prefix}-val-state"
  sleep "$INTERVAL_SEC"

  echo "==> missing_source_type (400)"
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${BASE_URL}/user/1/transaction" \
    -H "Content-Type: application/json" \
    -d "{\"state\":\"win\",\"amount\":\"1.00\",\"transactionId\":\"${prefix}-no-src\"}")
  echo "  POST user=1 (no Source-Type) -> HTTP ${code}"
  sleep "$INTERVAL_SEC"

  echo "==> invalid_json (400)"
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${BASE_URL}/user/1/transaction" \
    -H "Source-Type: game" \
    -H "Content-Type: application/json" \
    -d '{broken json')
  echo "  POST user=1 (malformed JSON) -> HTTP ${code}"
  sleep "$INTERVAL_SEC"

  echo "==> invalid_user_id (400) — user id 0"
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${BASE_URL}/user/0/transaction" \
    -H "Source-Type: game" \
    -H "Content-Type: application/json" \
    -d "{\"state\":\"win\",\"amount\":\"1.00\",\"transactionId\":\"${prefix}-uid0\"}")
  echo "  POST user=0 -> HTTP ${code}"
  sleep "$INTERVAL_SEC"

  echo "==> invalid_user_id (400) — non-numeric id"
  code=$(curl -s -o /dev/null -w "%{http_code}" -X GET "${BASE_URL}/user/abc/balance")
  echo "  GET user=abc/balance -> HTTP ${code}"
  sleep "$INTERVAL_SEC"

  echo "==> user_not_found (404)"
  code=$(curl -s -o /dev/null -w "%{http_code}" -X POST "${BASE_URL}/user/999/transaction" \
    -H "Source-Type: game" \
    -H "Content-Type: application/json" \
    -d "{\"state\":\"win\",\"amount\":\"1.00\",\"transactionId\":\"${prefix}-404\"}")
  echo "  POST user=999 -> HTTP ${code}"
  sleep "$INTERVAL_SEC"
}

for round in $(seq 1 "$ROUNDS"); do
  run_round "$round"
done

echo
echo "==> Done. Bar gauge should show all business reasons except internal (stays 0)."
echo "    Rejection reasons expected:"
echo "      duplicate_transaction, insufficient_funds, validation,"
echo "      user_not_found, invalid_user_id, missing_source_type, invalid_json"
echo "    Wait ~30s or keep Grafana on 5s refresh."
