package domain

import "fmt"

// TransactionState describes whether funds are credited or debited.
type TransactionState string

const (
	StateWin  TransactionState = "win"
	StateLose TransactionState = "lose"
)

var validTransactionStates = map[TransactionState]struct{}{
	StateWin:  {},
	StateLose: {},
}

// IsValid reports whether the state is part of the business vocabulary.
func (s TransactionState) IsValid() bool {
	_, ok := validTransactionStates[s]
	return ok
}

// ParseTransactionState parses an external string into a validated TransactionState.
func ParseTransactionState(value string) (TransactionState, error) {
	s := TransactionState(value)
	if !s.IsValid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidState, value)
	}
	return s, nil
}
