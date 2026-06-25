package domain

import "errors"

var (
	// Business errors.
	ErrUserNotFound         = errors.New("user not found")
	ErrInsufficientFunds    = errors.New("insufficient funds")
	ErrDuplicateTransaction = errors.New("duplicate transaction")
)

var (
	// Vocabulary errors — invalid values for domain enums.
	ErrInvalidState      = errors.New("invalid state")
	ErrInvalidSourceType = errors.New("invalid source type")
)
