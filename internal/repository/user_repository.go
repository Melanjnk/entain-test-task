package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/achify/entain-test-task/internal/domain"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// UserRepository loads and updates user account rows.
type UserRepository struct {
	pool *pgxpool.Pool
}

// NewUserRepository creates a PostgreSQL-backed user repository.
func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

// FindByID returns the current balance for a user without taking a row lock.
func (r *UserRepository) FindByID(ctx context.Context, userID uint64) (domain.User, error) {
	var balance decimal.Decimal
	err := r.pool.QueryRow(ctx, `
		SELECT balance
		FROM users
		WHERE id = $1
	`, userID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("query user balance: %w", err)
	}

	return domain.User{ID: userID, Balance: balance}, nil
}

// FindByIDForUpdate loads a user row with FOR UPDATE inside an open database transaction.
func (r *UserRepository) FindByIDForUpdate(ctx context.Context, tx pgx.Tx, userID uint64) (domain.User, error) {
	var balance decimal.Decimal
	err := tx.QueryRow(ctx, `
		SELECT balance
		FROM users
		WHERE id = $1
		FOR UPDATE
	`, userID).Scan(&balance)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrUserNotFound
	}
	if err != nil {
		return domain.User{}, fmt.Errorf("lock user row: %w", err)
	}

	return domain.User{ID: userID, Balance: balance}, nil
}

// UpdateBalance persists a new balance for the user inside an open database transaction.
func (r *UserRepository) UpdateBalance(ctx context.Context, tx pgx.Tx, userID uint64, balance decimal.Decimal) error {
	tag, err := tx.Exec(ctx, `
		UPDATE users
		SET balance = $1, updated_at = NOW()
		WHERE id = $2
	`, balance, userID)
	if err != nil {
		return fmt.Errorf("update user balance: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("update user balance: unexpected rows affected")
	}
	return nil
}
