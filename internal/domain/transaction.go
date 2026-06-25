package domain

// Transaction is a balance-changing operation in the domain model.
type Transaction struct {
	UserID        uint64
	SourceType    SourceType
	State         TransactionState
	Amount        Money
	TransactionID string
}

// ApplyDelta returns the new balance after applying a win or lose transaction.
func ApplyDelta(current Money, state TransactionState, amount Money) (Money, error) {
	switch state {
	case StateWin:
		return current.Add(amount), nil
	case StateLose:
		next := current.Sub(amount)
		if next.IsNegative() {
			return Money{}, ErrInsufficientFunds
		}
		return next, nil
	default:
		return Money{}, ErrInvalidState
	}
}
