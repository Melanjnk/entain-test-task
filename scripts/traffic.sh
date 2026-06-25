#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
REQUESTS="${REQUESTS:-30}"
# Spread requests over time so Grafana rate()/SLI panels can see them (not a sub-second burst).
INTERVAL_SEC="${INTERVAL_SEC:-1}"

ok=0
fail=0

count_post_requests() {
  curl -sf "${BASE_URL}/metrics" \
    | grep '^entain_http_requests_total.*route="/user/:id/transaction"' \
    | awk '{sum += $NF} END {print sum + 0}'
}

count_applied_tx() {
  curl -sf "${BASE_URL}/metrics" \
    | awk '/^entain_transactions_applied_total / {print $2; exit}'
}

echo "==> Checking ${BASE_URL}/healthz"
curl -sf "${BASE_URL}/healthz" >/dev/null || {
  echo "ERROR: service not reachable at ${BASE_URL}"
  echo "Run: docker compose up -d"
  exit 1
}

before_post="$(count_post_requests)"
before_applied="$(count_applied_tx)"

echo "==> Sending ${REQUESTS} transactions (${INTERVAL_SEC}s between each, POST + GET)"
echo "    Old script fired 30 requests in <1s — Grafana rate() drops to ~0 within a minute."
echo

for i in $(seq 1 "$REQUESTS"); do
  user_id=$(( (i % 3) + 1 ))
  state="win"
  if (( i % 4 == 0 )); then
    state="lose"
  fi

  tx_id="traffic-${i}-$$-${RANDOM}"

  post_code="$(curl -s -o /dev/null -w "%{http_code}" -X POST "${BASE_URL}/user/${user_id}/transaction" \
    -H 'Source-Type: game' \
    -H 'Content-Type: application/json' \
    -d "{\"state\":\"${state}\",\"amount\":\"1.15\",\"transactionId\":\"${tx_id}\"}" || echo "000")"

  if [[ "$post_code" == "200" ]]; then
    ((ok++)) || true
  else
    echo "  [${i}/${REQUESTS}] POST user=${user_id} -> HTTP ${post_code}"
    ((fail++)) || true
  fi

  get_code="$(curl -s -o /dev/null -w "%{http_code}" "${BASE_URL}/user/${user_id}/balance" || echo "000")"

  if [[ "$get_code" == "200" ]]; then
    ((ok++)) || true
  else
    echo "  [${i}/${REQUESTS}] GET user=${user_id} -> HTTP ${get_code}"
    ((fail++)) || true
  fi

  printf "\r  progress: %d/%d (ok=%d fail=%d)" "$i" "$REQUESTS" "$ok" "$fail"

  if (( i < REQUESTS )); then
    sleep "$INTERVAL_SEC"
  fi
done

echo
echo

after_post="$(count_post_requests)"
after_applied="$(count_applied_tx)"

echo "==> Done"
echo "    HTTP ok/fail: ${ok}/${fail}"
echo "    POST counter:  ${before_post} -> ${after_post} (+$((after_post - before_post)))"
echo "    TX applied:    ${before_applied} -> ${after_applied} (+$((after_applied - before_applied)))"
echo
echo "    Metrics: make metrics"
echo "    Grafana:  http://localhost:3000/d/entain-balance-slo"
echo "    Keep this dashboard open while the script runs (~${REQUESTS}s)."
