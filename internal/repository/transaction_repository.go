package repository

import (
	"context"
	"fmt"

	"github.com/achify/entain-test-task/internal/domain"
	"github.com/jackc/pgx/v5"
)

// TransactionRepository records balance-changing transaction events.
type TransactionRepository struct{}

// NewTransactionRepository creates a transaction event repository.
func NewTransactionRepository() *TransactionRepository {
	return &TransactionRepository{}
}

// Record inserts a transaction row inside an open database transaction.
// Idempotency is enforced by the unique constraint on transaction_id.
func (r *TransactionRepository) Record(ctx context.Context, tx pgx.Tx, change domain.Transaction) error {
	tag, err := tx.Exec(ctx, `
		INSERT INTO transactions (transaction_id, user_id, source_type, state, amount)
		VALUES ($1, $2, $3, $4, $5)
	`, change.TransactionID, change.UserID, string(change.SourceType), string(change.State), change.Amount)
	if err != nil {
		if isUniqueViolation(err) {
			return domain.ErrDuplicateTransaction
		}
		return fmt.Errorf("insert transaction: %w", err)
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("insert transaction: unexpected rows affected")
	}
	return nil
}
