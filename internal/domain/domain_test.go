package domain

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplyDelta(t *testing.T) {
	t.Parallel()

	current := decimal.RequireFromString("100.00")

	t.Run("win increases balance", func(t *testing.T) {
		t.Parallel()
		next, err := ApplyDelta(current, StateWin, decimal.RequireFromString("10.15"))
		require.NoError(t, err)
		assert.Equal(t, "110.15", next.StringFixed(2))
	})

	t.Run("lose decreases balance", func(t *testing.T) {
		t.Parallel()
		next, err := ApplyDelta(current, StateLose, decimal.RequireFromString("25.50"))
		require.NoError(t, err)
		assert.Equal(t, "74.50", next.StringFixed(2))
	})

	t.Run("lose below zero rejected", func(t *testing.T) {
		t.Parallel()
		_, err := ApplyDelta(decimal.RequireFromString("5.00"), StateLose, decimal.RequireFromString("10.00"))
		require.ErrorIs(t, err, ErrInsufficientFunds)
	})
}

func TestParseSourceType(t *testing.T) {
	t.Parallel()

	for _, src := range []string{"game", "server", "payment"} {
		got, err := ParseSourceType(src)
		require.NoError(t, err)
		assert.Equal(t, SourceType(src), got)
	}

	_, err := ParseSourceType("unknown")
	require.ErrorIs(t, err, ErrInvalidSourceType)
}

func TestParseTransactionState(t *testing.T) {
	t.Parallel()

	for _, state := range []string{"win", "lose"} {
		got, err := ParseTransactionState(state)
		require.NoError(t, err)
		assert.Equal(t, TransactionState(state), got)
	}

	_, err := ParseTransactionState("draw")
	require.ErrorIs(t, err, ErrInvalidState)
}

func TestSourceType_IsValid(t *testing.T) {
	t.Parallel()

	assert.True(t, SourceGame.IsValid())
	assert.False(t, SourceType("bad").IsValid())
}

func TestTransactionState_IsValid(t *testing.T) {
	t.Parallel()

	assert.True(t, StateWin.IsValid())
	assert.False(t, TransactionState("bad").IsValid())
}
