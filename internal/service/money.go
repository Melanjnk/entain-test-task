package service

import (
	"fmt"

	"github.com/achify/entain-test-task/internal/domain"
	"github.com/shopspring/decimal"
)

// parseMoney parses a monetary string from the transport layer.
func parseMoney(value string) (domain.Money, error) {
	amount, err := decimal.NewFromString(value)
	if err != nil {
		return domain.Money{}, fmt.Errorf("%w: %q", ErrInvalidAmount, value)
	}
	if amount.LessThanOrEqual(decimal.Zero) {
		return domain.Money{}, fmt.Errorf("%w: must be positive", ErrInvalidAmount)
	}
	if amount.Exponent() < -2 {
		return domain.Money{}, fmt.Errorf("%w: more than 2 decimal places", ErrInvalidAmount)
	}
	return amount.Round(2), nil
}
