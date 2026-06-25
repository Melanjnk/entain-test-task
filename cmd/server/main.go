package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/achify/entain-test-task/internal/handler"
	"github.com/achify/entain-test-task/internal/metrics"
	"github.com/achify/entain-test-task/internal/migrate"
	"github.com/achify/entain-test-task/internal/repository"
	"github.com/achify/entain-test-task/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	if err := run(logger); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("application stopped", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	ctx := context.Background()

	dsn := envOr("DATABASE_URL", "postgres://entain:entain@localhost:5432/entain?sslmode=disable")
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("parse database config: %w", err)
	}
	if maxConns := os.Getenv("DB_POOL_MAX_CONNS"); maxConns != "" {
		n, err := strconv.ParseInt(maxConns, 10, 32)
		if err != nil || n <= 0 {
			return fmt.Errorf("invalid DB_POOL_MAX_CONNS: %q", maxConns)
		}
		poolCfg.MaxConns = int32(n)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	defer pool.Close()

	if err := waitForDB(ctx, pool, 30*time.Second); err != nil {
		return err
	}

	migrationsDir := resolveMigrationsDir()
	if err := migrate.Run(ctx, pool, migrationsDir); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	logger.Info("migrations applied", "dir", migrationsDir)

	collector := metrics.New()
	repo := repository.NewBalanceRepository(pool)
	svc := service.NewBalanceService(repo)
	api := handler.NewAPI(svc, collector, logger)

	mux := http.NewServeMux()
	api.Register(mux)
	mux.Handle("GET /metrics", collector.Handler())
	mux.HandleFunc("GET /debug/pprof/", pprof.Index)
	mux.HandleFunc("GET /debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("GET /debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("GET /debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("GET /debug/pprof/trace", pprof.Trace)

	addr := envOr("HTTP_ADDR", ":8080")
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", addr)
		errCh <- server.ListenAndServe()
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-errCh:
		return err
	case sig := <-stop:
		logger.Info("shutdown signal received", "signal", sig.String())
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

func waitForDB(ctx context.Context, pool *pgxpool.Pool, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		if err := pool.Ping(ctx); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("database not ready after %s", timeout)
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func resolveMigrationsDir() string {
	if dir := os.Getenv("MIGRATIONS_DIR"); dir != "" {
		return dir
	}
	for _, candidate := range []string{"migrations", "/app/migrations"} {
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
	}
	return "migrations"
}
