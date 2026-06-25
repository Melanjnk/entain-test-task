.PHONY: test test-all bench integration up down logs load-profile traffic metrics demo-errors \
	load-25 load-40 load-100 load-150 load-200 load-500 load-suite load-contention loadgen

LOAD_DURATION ?= 2m
# Stress tests hammer user 1 — serializes on FOR UPDATE (realistic hot-account scenario).
STRESS_USER ?= 1

test:
	go test ./...

bench:
	go test -bench=. -benchmem ./internal/domain ./internal/service

integration:
	go test -tags=integration ./internal/repository/...

test-all: test integration

up:
	docker compose up -d --build

down:
	docker compose down -v

logs:
	docker compose logs -f app

traffic:
	chmod +x scripts/traffic.sh
	./scripts/traffic.sh

demo-errors:
	chmod +x scripts/demo-errors.sh
	./scripts/demo-errors.sh

metrics:
	@curl -s http://localhost:8080/metrics | grep '^entain_' || true

obs-up:
	docker compose up -d --build prometheus grafana app
	@echo "Grafana: http://localhost:3000/d/entain-balance-slo (admin/entain)"
	@echo "Prometheus: http://localhost:9090/-/healthy"

# --- Capacity tests (run obs-up first, open Grafana) ---

load-25:
	go run ./cmd/loadgen -rate 25 -duration $(LOAD_DURATION) -label task-baseline

load-40:
	go run ./cmd/loadgen -rate 40 -duration $(LOAD_DURATION) -label headroom

# Stress: single hot user — row lock serializes writes; degradation visible 200+.
load-100:
	go run ./cmd/loadgen -rate 100 -duration $(LOAD_DURATION) -user $(STRESS_USER) -label stress-100-hot

load-150:
	go run ./cmd/loadgen -rate 150 -duration $(LOAD_DURATION) -user $(STRESS_USER) -label stress-150-hot

load-200:
	go run ./cmd/loadgen -rate 200 -duration $(LOAD_DURATION) -user $(STRESS_USER) -label stress-200-hot

load-500:
	go run ./cmd/loadgen -rate 500 -duration $(LOAD_DURATION) -user $(STRESS_USER) -label saturation-500-hot

load-suite:
	chmod +x scripts/load-suite.sh
	./scripts/load-suite.sh

load-contention:
	go run ./cmd/loadgen -rate 40 -duration $(LOAD_DURATION) -user 1 -label lock-contention

loadgen:
	@echo "Task zone: load-25 load-40 (users rotated)"
	@echo "Stress:    load-100 load-150 load-200 load-500 (hot user $(STRESS_USER))"
	@echo "Fix Grafana: make obs-up"

load-profile:
	@echo "Collecting 10s CPU profile from running server..."
	curl -s "http://localhost:8080/debug/pprof/profile?seconds=10" -o cpu.prof
	@echo "Saved cpu.prof — run: go tool pprof cpu.prof"
