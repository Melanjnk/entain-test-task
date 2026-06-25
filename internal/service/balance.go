package service

import (
	"context"
	"strings"

	"github.com/achify/entain-test-task/internal/domain"
)

// BalanceRepository defines persistence operations used by the service layer.
type BalanceRepository interface {
	GetBalance(ctx context.Context, userID uint64) (domain.User, error)
	ApplyTransaction(ctx context.Context, tx domain.Transaction) (domain.User, error)
}

// BalanceService implements balance and transaction use cases.
type BalanceService struct {
	repo BalanceRepository
}

// NewBalanceService wires a balance service with its dependencies.
func NewBalanceService(repo BalanceRepository) *BalanceService {
	return &BalanceService{repo: repo}
}

// GetBalance returns the current user account state.
func (s *BalanceService) GetBalance(ctx context.Context, userID uint64) (domain.User, error) {
	if userID == 0 {
		return domain.User{}, ErrInvalidUserID
	}

	return s.repo.GetBalance(ctx, userID)
}

// ProcessTransaction validates input and applies a balance change.
func (s *BalanceService) ProcessTransaction(ctx context.Context, cmd ProcessTransactionCommand) (domain.User, error) {
	if cmd.UserID == 0 {
		return domain.User{}, ErrInvalidUserID
	}

	transactionID := strings.TrimSpace(cmd.TransactionID)
	if transactionID == "" {
		return domain.User{}, ErrInvalidTransactionID
	}

	src, err := domain.ParseSourceType(cmd.SourceType)
	if err != nil {
		return domain.User{}, err
	}

	state, err := domain.ParseTransactionState(cmd.State)
	if err != nil {
		return domain.User{}, err
	}

	amount, err := parseMoney(cmd.Amount)
	if err != nil {
		return domain.User{}, err
	}

	return s.repo.ApplyTransaction(ctx, domain.Transaction{
		UserID:        cmd.UserID,
		SourceType:    src,
		State:         state,
		Amount:        amount,
		TransactionID: transactionID,
	})
}
