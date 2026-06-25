package service

import "errors"

var (
	ErrInvalidUserID        = errors.New("invalid user id")
	ErrInvalidTransactionID = errors.New("invalid transaction id")
	ErrInvalidAmount        = errors.New("invalid amount")
)
