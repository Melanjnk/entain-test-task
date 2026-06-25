package domain

import (
	"testing"

	"github.com/shopspring/decimal"
)

func BenchmarkApplyDelta(b *testing.B) {
	current := decimal.RequireFromString("1000.00")
	amount := decimal.RequireFromString("10.15")
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = ApplyDelta(current, StateWin, amount)
	}
}
