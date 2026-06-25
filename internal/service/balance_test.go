package service

import (
	"context"
	"testing"

	"github.com/achify/entain-test-task/internal/domain"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockBalanceRepository struct {
	balance     domain.User
	applyCalls  int
	lastRequest domain.Transaction
	applyErr    error
	getErr      error
}

func (m *mockBalanceRepository) GetBalance(_ context.Context, userID uint64) (domain.User, error) {
	if m.getErr != nil {
		return domain.User{}, m.getErr
	}
	if m.balance.ID == 0 {
		m.balance.ID = userID
	}
	return m.balance, nil
}

func (m *mockBalanceRepository) ApplyTransaction(_ context.Context, tx domain.Transaction) (domain.User, error) {
	m.applyCalls++
	m.lastRequest = tx
	if m.applyErr != nil {
		return domain.User{}, m.applyErr
	}
	next, err := domain.ApplyDelta(m.balance.Balance, tx.State, tx.Amount)
	if err != nil {
		return domain.User{}, err
	}
	m.balance = domain.User{ID: tx.UserID, Balance: next}
	return m.balance, nil
}

func TestBalanceService_GetBalance(t *testing.T) {
	t.Parallel()

	svc := NewBalanceService(&mockBalanceRepository{
		balance: domain.User{ID: 1, Balance: decimal.RequireFromString("9.25")},
	})

	user, err := svc.GetBalance(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, uint64(1), user.ID)
	assert.Equal(t, "9.25", user.Balance.StringFixed(2))
}

func TestBalanceService_ProcessTransaction(t *testing.T) {
	t.Parallel()

	repo := &mockBalanceRepository{
		balance: domain.User{ID: 1, Balance: decimal.RequireFromString("100.00")},
	}
	svc := NewBalanceService(repo)

	user, err := svc.ProcessTransaction(context.Background(), ProcessTransactionCommand{
		UserID:        1,
		SourceType:    "game",
		State:         "win",
		Amount:        "10.15",
		TransactionID: "tx-1",
	})
	require.NoError(t, err)
	assert.Equal(t, "110.15", user.Balance.StringFixed(2))
	assert.Equal(t, 1, repo.applyCalls)
	assert.Equal(t, "tx-1", repo.lastRequest.TransactionID)
	assert.Equal(t, domain.SourceGame, repo.lastRequest.SourceType)
}

func TestBalanceService_ProcessTransaction_InvalidInput(t *testing.T) {
	t.Parallel()

	svc := NewBalanceService(&mockBalanceRepository{})

	_, err := svc.ProcessTransaction(context.Background(), ProcessTransactionCommand{
		UserID: 0, SourceType: "game", State: "win", Amount: "1.00", TransactionID: "tx",
	})
	require.ErrorIs(t, err, ErrInvalidUserID)

	_, err = svc.ProcessTransaction(context.Background(), ProcessTransactionCommand{
		UserID: 1, SourceType: "bad", State: "win", Amount: "1.00", TransactionID: "tx",
	})
	require.ErrorIs(t, err, domain.ErrInvalidSourceType)

	_, err = svc.ProcessTransaction(context.Background(), ProcessTransactionCommand{
		UserID: 1, SourceType: "game", State: "bad", Amount: "1.00", TransactionID: "tx",
	})
	require.ErrorIs(t, err, domain.ErrInvalidState)

	_, err = svc.ProcessTransaction(context.Background(), ProcessTransactionCommand{
		UserID: 1, SourceType: "game", State: "win", Amount: "0", TransactionID: "tx",
	})
	require.ErrorIs(t, err, ErrInvalidAmount)

	_, err = svc.ProcessTransaction(context.Background(), ProcessTransactionCommand{
		UserID: 1, SourceType: "game", State: "win", Amount: "1.00", TransactionID: "  ",
	})
	require.ErrorIs(t, err, ErrInvalidTransactionID)
}

func TestBalanceService_ProcessTransaction_DuplicateReturnsError(t *testing.T) {
	t.Parallel()

	repo := &mockBalanceRepository{
		balance: domain.User{ID: 1, Balance: decimal.RequireFromString("50.00")},
	}
	svc := NewBalanceService(repo)

	_, err := svc.ProcessTransaction(context.Background(), ProcessTransactionCommand{
		UserID: 1, SourceType: "game", State: "win", Amount: "5.00", TransactionID: "dup-tx",
	})
	require.NoError(t, err)

	repo.applyErr = domain.ErrDuplicateTransaction
	_, err = svc.ProcessTransaction(context.Background(), ProcessTransactionCommand{
		UserID: 1, SourceType: "game", State: "win", Amount: "5.00", TransactionID: "dup-tx",
	})
	require.ErrorIs(t, err, domain.ErrDuplicateTransaction)
}
