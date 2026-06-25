# Entain Balance Service

Go + PostgreSQL service that processes third-party balance transactions with ACID guarantees, idempotency, and observability hooks.

## Quick start

No extra configuration required:

```bash
docker compose up -d
```

First run builds the Go 1.26 image automatically. To force rebuild:

```bash
docker compose up -d --build
```

Predefined users:

| userId | initial balance |
| ------ | --------------- |
| 1      | 100.00          |
| 2      | 250.50          |
| 3      | 0.00            |

Health check:

```bash
curl http://localhost:8080/healthz
```

Get balance:

```bash
curl http://localhost:8080/user/1/balance
```

Post transaction:

```bash
curl -X POST http://localhost:8080/user/1/transaction \
  -H 'Source-Type: game' \
  -H 'Content-Type: application/json' \
  -d '{"state":"win","amount":"10.15","transactionId":"tx-001"}'
```

## API

### `GET /user/{userId}/balance`

Response `200`:

```json
{
  "userId": 1,
  "balance": "110.15"
}
```

### `POST /user/{userId}/transaction`

Headers:

- `Source-Type`: `game` | `server` | `payment`
- `Content-Type`: `application/json`

Body:

```json
{
  "state": "win",
  "amount": "10.15",
  "transactionId": "unique-id"
}
```

`win` credits the account, `lose` debits it.

## Error handling

All error responses use JSON: `{"error":"..."}`.

### `POST /user/{userId}/transaction`

| HTTP  | Meaning                                                                         |
|-------|---------------------------------------------------------------------------------|
| `200` | Transaction applied exactly once                                                |
| `400` | Validation — invalid JSON, user id, `state`, `amount`, or missing `Source-Type` |
| `402` | Insufficient funds                                                              |
| `404` | User not found                                                                  |
| `409` | Duplicate `transactionId` — already applied, balance unchanged                  |
| `500` | Internal error                                                                  |
| `503` | Service unavailable (for example database unreachable)                          |

### `GET /user/{userId}/balance`

| HTTP   | Meaning          |
|--------|------------------|
| `200`  | Balance returned |
| `400`  | Invalid user id  |
| `404`  | User not found   |
| `500`  | Internal error   |

> `402 Payment Required` is used for insufficient funds (not `422`). Runtime dependency failures today usually surface as `500`; explicit `503` can be added for health-check integration.

## Retry contract

Third-party providers may retry when a call fails or times out. The service is **idempotent by `transactionId`**.

**Guarantee:** If the provider retries using the **same** `transactionId`, the transaction will **never** be applied twice.

| Situation                                       | Safe to retry with same `transactionId`?  |
|-------------------------------------------------|-------------------------------------------|
| Timeout / no response                           | Yes                                       |
| `5xx` or `503`                                  | Yes                                       |
| `200` already received, provider retries anyway | Yes — expect `409`; balance unchanged     |
| `400`, `402`, `404`                             | No — fix the request first                |

Enforced by `UNIQUE(transaction_id)` in PostgreSQL and a single database transaction per apply.

## Throughput

Current design comfortably satisfies the required **20–30 RPS**.

| Scenario                      | Command                           | Notes                                         |
|-------------------------------|-----------------------------------|-----------------------------------------------|
| Task requirement (20–30 RPS)  | `make load-25`                    | Baseline verification                         |
| Headroom                      | `make load-40`                    | Still within comfortable targets              |
| Stress / saturation           | `make load-100` … `make load-500` | Observe latency in Grafana                    |

See **Load tests** below for the full capacity ladder.

Implementation notes: `pgxpool` connection pooling, short transactions, indexed `transaction_id`.

Future improvements could include:

- partitioning by `user_id`
- read replicas for balance reads
- caching with careful invalidation

## Architecture decisions

### Balance updates

One PostgreSQL transaction guarantees atomic balance changes:

1. `SELECT ... FOR UPDATE` on the user row
2. validate non-negative balance
3. `INSERT` into `transactions` (`UNIQUE(transaction_id)`)
4. `UPDATE users.balance`

### Why not the Outbox pattern?

The [Transactional Outbox](https://microservices.io/patterns/data/transactional-outbox.html) solves **reliable async side effects** (e.g. publishing to Kafka after a DB write). This task is a synchronous HTTP balance API with no downstream event bus requirement. A single database transaction is simpler and sufficient at 20–30 RPS.

## Design trade-offs

| Chosen                               | Instead of             | Reason                                                                      |
|--------------------------------------|------------------------|-----------------------------------------------------------------------------|
| PostgreSQL `FOR UPDATE` row locking  | Optimistic locking     | Balance updates are short-lived; contention on individual users is expected |
| `shopspring/decimal`                 | `float64`              | Money must not suffer floating-point rounding errors                        |
| `UNIQUE(transaction_id)` + single TX | Application-only dedup | Database-enforced idempotency survives retries and crashes                  |
| Synchronous HTTP + one DB TX         | Transactional Outbox   | No async downstream consumers in scope                                      |

## Observability

### Prometheus metrics — `GET /metrics`

All application metrics use the `entain_` prefix:

| Metric                                  | Description                                               |
|-----------------------------------------|-----------------------------------------------------------|
| `entain_http_requests_total`            | HTTP count by `method`, `route`, `status`                 |
| `entain_http_request_duration_seconds`  | Request latency histogram                                 |
| `entain_transactions_applied_total`     | Committed balance changes                                 |
| `entain_transactions_rejected_total`    | Rejections by reason                                      |
| `entain_insufficient_funds_total`       | Insufficient balance rejections (HTTP 402)                |
| `entain_build_info`                     | Static service metadata                                   |

Quick check:

```bash
make metrics
# or
curl -s http://localhost:8080/metrics | grep '^entain_'
```

### Prometheus + Grafana

Included in `docker compose up -d`:

![Grafana Dashboard](docs/images/grafana-dashboard.png)

| Service     | URL                          | Credentials         |
|-------------|------------------------------|---------------------|
| Prometheus  | http://localhost:9090        | —                   |
| Grafana     | http://localhost:3000        | `admin` / `entain`  |

Direct dashboard URL: http://localhost:3000/d/entain-balance-slo

Or: **Dashboards → Browse → General** → `Entain Balance API — SLI/SLO`

Generate sample traffic (panels stay empty without requests):

```bash
make traffic
```

Wait ~30–60s for SLI recording rules, then refresh the dashboard.

### Service objectives

**SLI** (what we measure):

- Availability — share of `2xx` responses
- Latency — p95 / p99 of `entain_http_request_duration_seconds`
- Throughput — requests per second
- Transaction success — applied / (applied + rejected)

**Operational targets** (internal goals for this task):

- Availability ≥ **99.9%**
- p95 latency **< 100ms** at ~30 RPS
- Sustain **20–30 RPS** without error-budget burn

The Grafana **Error Budget** panel tracks `5xx` ratio against the 0.1% budget implied by the availability target.

Recording rules live in `observability/rules/sli.yml` (`entain:sli:*` metrics).

### pprof — `GET /debug/pprof/`

CPU profile example while the service is under load:

```bash
curl "http://localhost:8080/debug/pprof/profile?seconds=10" -o cpu.prof
go tool pprof -http=:0 cpu.prof
```

## Testing

Repository tests are integration tests and are excluded from the default `go test ./...` run using the `integration` build tag. That is why `make test` may show `? internal/repository [no test files]` — run `make integration` to execute them.

### Fast unit tests

```bash
make test
```

### Repository integration tests (requires PostgreSQL)

Start Postgres first (`docker compose up -d`), then:

```bash
make integration
```

### Run everything

```bash
make test-all
```

### Benchmarks

```bash
make bench
```

### Load tests (20–30 RPS requirement)

Open Grafana, then run the capacity ladder:

```bash
make load-25     # task baseline — expect WITHIN_SLO
make load-40     # headroom — still fast, NOT degradation
make load-100    # stress — watch p95 rise in Grafana
make load-150
make load-200    # find SATURATED vs GRACEFUL_DEGRADATION

# or full suite (~10 min): 25 → 40 → 100 → 150 → 200
make load-suite
```

**How to present in review:**

| RPS      | Expected verdict                       | What it proves                                   |
|----------|----------------------------------------|--------------------------------------------------|
| 25       | `WITHIN_SLO`                           | Meets task requirement (20–30 RPS)               |
| 40       | `WITHIN_SLO`                           | Headroom with snappy p95                         |
| 100–200  | `GRACEFUL_DEGRADATION` or `SATURATED`  | DB pool + `FOR UPDATE` limits; service stays up  |

Grafana: **Latency percentiles vs SLO** — flat at 25–40, climbs at 100+. Optional: `make load-contention` for single-user lock demo only.

## Project layout

```
cmd/server/           HTTP entrypoint
internal/domain/      pure business rules
internal/service/     use cases
internal/repository/  PostgreSQL access
internal/handler/     HTTP transport
internal/metrics/     Prometheus instrumentation
migrations/           schema + seed users
```

## Environment variables

| Variable        | Default                                                            | Description     |
|-----------------|--------------------------------------------------------------------|-----------------|
| `HTTP_ADDR`     | `:8080`                                                            | Listen address  |
| `DATABASE_URL`  | `postgres://entain:entain@localhost:5432/entain?sslmode=disable`   | Postgres DSN    |
