package handler

import (
	"github.com/achify/entain-test-task/internal/domain"
	"github.com/shopspring/decimal"
)

type transactionRequestDTO struct {
	State         string `json:"state"`
	Amount        string `json:"amount"`
	TransactionID string `json:"transactionId"`
}

type balanceResponseDTO struct {
	UserID  uint64 `json:"userId"`
	Balance string `json:"balance"`
}

func toBalanceResponseDTO(user domain.User) balanceResponseDTO {
	return balanceResponseDTO{
		UserID:  user.ID,
		Balance: formatBalance(user.Balance),
	}
}

func formatBalance(value decimal.Decimal) string {
	return value.Round(2).StringFixed(2)
}
