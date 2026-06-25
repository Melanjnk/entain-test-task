package service

import (
	"context"
	"testing"

	"github.com/achify/entain-test-task/internal/domain"
	"github.com/shopspring/decimal"
)

func BenchmarkProcessTransaction(b *testing.B) {
	repo := &mockBalanceRepository{
		balance: domain.User{ID: 1, Balance: decimal.RequireFromString("10000.00")},
	}
	svc := NewBalanceService(repo)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = svc.ProcessTransaction(ctx, ProcessTransactionCommand{
			UserID: 1, SourceType: "game", State: "win", Amount: "1.00", TransactionID: "bench-tx",
		})
	}
}
