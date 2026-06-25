package repository

import (
	"context"
	"fmt"

	"github.com/achify/entain-test-task/internal/domain"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BalanceRepository coordinates user and transaction persistence for balance use cases.
type BalanceRepository struct {
	pool         *pgxpool.Pool
	users        *UserRepository
	transactions *TransactionRepository
}

// NewBalanceRepository wires aggregate persistence backed by a connection pool.
func NewBalanceRepository(pool *pgxpool.Pool) *BalanceRepository {
	return &BalanceRepository{
		pool:         pool,
		users:        NewUserRepository(pool),
		transactions: NewTransactionRepository(),
	}
}

// GetBalance returns the current balance for a user.
func (r *BalanceRepository) GetBalance(ctx context.Context, userID uint64) (domain.User, error) {
	return r.users.FindByID(ctx, userID)
}

// ApplyTransaction atomically records a transaction and updates the user balance.
func (r *BalanceRepository) ApplyTransaction(ctx context.Context, change domain.Transaction) (domain.User, error) {
	dbTx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin tx: %w", err)
	}
	defer dbTx.Rollback(ctx)

	user, err := r.users.FindByIDForUpdate(ctx, dbTx, change.UserID)
	if err != nil {
		return domain.User{}, err
	}

	next, err := domain.ApplyDelta(user.Balance, change.State, change.Amount)
	if err != nil {
		return domain.User{}, err
	}

	if err := r.transactions.Record(ctx, dbTx, change); err != nil {
		return domain.User{}, err
	}

	if err := r.users.UpdateBalance(ctx, dbTx, change.UserID, next); err != nil {
		return domain.User{}, err
	}

	if err := dbTx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("commit tx: %w", err)
	}

	return domain.User{ID: change.UserID, Balance: next}, nil
}
