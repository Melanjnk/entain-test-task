//go:build integration

package repository_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/achify/entain-test-task/internal/domain"
	"github.com/achify/entain-test-task/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func integrationPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://entain:entain@localhost:5432/entain?sslmode=disable"
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pool.Ping(ctx))
	return pool
}

func uniqueTxID(t *testing.T, suffix string) string {
	t.Helper()
	return fmt.Sprintf("integration-%s-%s-%d", t.Name(), suffix, time.Now().UnixNano())
}

func setUserBalance(t *testing.T, pool *pgxpool.Pool, userID uint64, balance decimal.Decimal) {
	t.Helper()
	_, err := pool.Exec(context.Background(), `
		UPDATE users SET balance = $1, updated_at = NOW() WHERE id = $2
	`, balance, userID)
	require.NoError(t, err)
}

func getBalanceDirect(t *testing.T, pool *pgxpool.Pool, userID uint64) decimal.Decimal {
	t.Helper()
	var balance decimal.Decimal
	err := pool.QueryRow(context.Background(), `
		SELECT balance FROM users WHERE id = $1
	`, userID).Scan(&balance)
	require.NoError(t, err)
	return balance
}

func transactionExists(t *testing.T, pool *pgxpool.Pool, transactionID string) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(context.Background(), `
		SELECT EXISTS(SELECT 1 FROM transactions WHERE transaction_id = $1)
	`, transactionID).Scan(&exists)
	require.NoError(t, err)
	return exists
}

func TestBalanceRepository_GetBalance_ExistingUser(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewBalanceRepository(pool)

	setUserBalance(t, pool, 1, decimal.RequireFromString("42.50"))

	user, err := repo.GetBalance(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), user.ID)
	assert.Equal(t, "42.50", user.Balance.StringFixed(2))
}

func TestBalanceRepository_GetBalance_MissingUser(t *testing.T) {
	repo := repository.NewBalanceRepository(integrationPool(t))

	_, err := repo.GetBalance(context.Background(), 99999)
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestBalanceRepository_ApplyTransaction_Win_IncreasesBalanceAndSavesTransaction(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewBalanceRepository(pool)
	ctx := context.Background()

	setUserBalance(t, pool, 1, decimal.RequireFromString("100.00"))
	txID := uniqueTxID(t, "win")

	user, err := repo.ApplyTransaction(ctx, domain.Transaction{
		UserID:        1,
		SourceType:    domain.SourceGame,
		State:         domain.StateWin,
		Amount:        decimal.RequireFromString("10.15"),
		TransactionID: txID,
	})
	require.NoError(t, err)
	assert.Equal(t, "110.15", user.Balance.StringFixed(2))
	assert.Equal(t, "110.15", getBalanceDirect(t, pool, 1).StringFixed(2))
	assert.True(t, transactionExists(t, pool, txID))
}

func TestBalanceRepository_ApplyTransaction_Lose_DecreasesBalance(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewBalanceRepository(pool)
	ctx := context.Background()

	setUserBalance(t, pool, 1, decimal.RequireFromString("100.00"))
	txID := uniqueTxID(t, "lose")

	user, err := repo.ApplyTransaction(ctx, domain.Transaction{
		UserID:        1,
		SourceType:    domain.SourcePayment,
		State:         domain.StateLose,
		Amount:        decimal.RequireFromString("25.50"),
		TransactionID: txID,
	})
	require.NoError(t, err)
	assert.Equal(t, "74.50", user.Balance.StringFixed(2))
	assert.Equal(t, "74.50", getBalanceDirect(t, pool, 1).StringFixed(2))
	assert.True(t, transactionExists(t, pool, txID))
}

func TestBalanceRepository_ApplyTransaction_InsufficientFunds_BalanceUnchanged(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewBalanceRepository(pool)
	ctx := context.Background()

	setUserBalance(t, pool, 1, decimal.RequireFromString("5.00"))
	before := getBalanceDirect(t, pool, 1)
	txID := uniqueTxID(t, "insufficient")

	_, err := repo.ApplyTransaction(ctx, domain.Transaction{
		UserID:        1,
		SourceType:    domain.SourceGame,
		State:         domain.StateLose,
		Amount:        decimal.RequireFromString("10.00"),
		TransactionID: txID,
	})
	require.ErrorIs(t, err, domain.ErrInsufficientFunds)
	assert.Equal(t, before.StringFixed(2), getBalanceDirect(t, pool, 1).StringFixed(2))
	assert.False(t, transactionExists(t, pool, txID))
}

func TestBalanceRepository_ApplyTransaction_Duplicate_BalanceUnchanged(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewBalanceRepository(pool)
	ctx := context.Background()

	setUserBalance(t, pool, 1, decimal.RequireFromString("50.00"))
	txID := uniqueTxID(t, "duplicate")

	first, err := repo.ApplyTransaction(ctx, domain.Transaction{
		UserID:        1,
		SourceType:    domain.SourceGame,
		State:         domain.StateWin,
		Amount:        decimal.RequireFromString("5.00"),
		TransactionID: txID,
	})
	require.NoError(t, err)

	_, err = repo.ApplyTransaction(ctx, domain.Transaction{
		UserID:        1,
		SourceType:    domain.SourceGame,
		State:         domain.StateWin,
		Amount:        decimal.RequireFromString("5.00"),
		TransactionID: txID,
	})
	require.ErrorIs(t, err, domain.ErrDuplicateTransaction)
	assert.Equal(t, first.Balance.StringFixed(2), getBalanceDirect(t, pool, 1).StringFixed(2))
}

func TestBalanceRepository_ApplyTransaction_UnknownUser(t *testing.T) {
	repo := repository.NewBalanceRepository(integrationPool(t))

	_, err := repo.ApplyTransaction(context.Background(), domain.Transaction{
		UserID:        99999,
		SourceType:    domain.SourceGame,
		State:         domain.StateWin,
		Amount:        decimal.RequireFromString("1.00"),
		TransactionID: uniqueTxID(t, "unknown-user"),
	})
	require.ErrorIs(t, err, domain.ErrUserNotFound)
}

func TestBalanceRepository_ApplyTransaction_ConcurrentLose_CannotGoNegative(t *testing.T) {
	pool := integrationPool(t)
	repo := repository.NewBalanceRepository(pool)
	ctx := context.Background()

	const userID = 2
	setUserBalance(t, pool, userID, decimal.RequireFromString("10.00"))

	const workers = 20
	loseAmount := decimal.RequireFromString("2.00")

	var wg sync.WaitGroup
	errCh := make(chan error, workers)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			_, err := repo.ApplyTransaction(ctx, domain.Transaction{
				UserID:        userID,
				SourceType:    domain.SourceGame,
				State:         domain.StateLose,
				Amount:        loseAmount,
				TransactionID: fmt.Sprintf("integration-concurrent-%s-%d-%d", t.Name(), worker, time.Now().UnixNano()),
			})
			errCh <- err
		}(i)
	}

	wg.Wait()
	close(errCh)

	var successes, insufficient int
	for err := range errCh {
		switch {
		case err == nil:
			successes++
		case errors.Is(err, domain.ErrInsufficientFunds):
			insufficient++
		default:
			t.Fatalf("unexpected error: %v", err)
		}
	}

	// 10.00 / 2.00 = 5 successful debits maximum under row locking.
	assert.Equal(t, 5, successes)
	assert.Equal(t, workers-successes, insufficient)

	final := getBalanceDirect(t, pool, userID)
	assert.False(t, final.IsNegative())
	assert.Equal(t, "0.00", final.StringFixed(2))
}
