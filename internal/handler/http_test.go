package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/achify/entain-test-task/internal/domain"
	"github.com/achify/entain-test-task/internal/metrics"
	"github.com/achify/entain-test-task/internal/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPI_GetBalance(t *testing.T) {
	t.Parallel()

	api := NewAPI(newTestService(decimal.RequireFromString("9.25")), metrics.New(), slog.Default())
	mux := http.NewServeMux()
	api.Register(mux)

	req := httptest.NewRequest(http.MethodGet, "/user/1/balance", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp balanceResponseDTO
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, uint64(1), resp.UserID)
	assert.Equal(t, "9.25", resp.Balance)
}

func TestAPI_PostTransaction_Win(t *testing.T) {
	t.Parallel()

	store := &handlerMockStore{balance: domain.User{ID: 1, Balance: decimal.RequireFromString("100.00")}}
	api := NewAPI(service.NewBalanceService(store), metrics.New(), slog.Default())
	mux := http.NewServeMux()
	api.Register(mux)

	body := `{"state":"win","amount":"10.15","transactionId":"tx-123"}`
	req := httptest.NewRequest(http.MethodPost, "/user/1/transaction", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Source-Type", "game")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)

	var resp balanceResponseDTO
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
	assert.Equal(t, "110.15", resp.Balance)
}

func TestAPI_PostTransaction_ValidationErrors(t *testing.T) {
	t.Parallel()

	api := NewAPI(newTestService(decimal.Zero), metrics.New(), slog.Default())
	mux := http.NewServeMux()
	api.Register(mux)

	req := httptest.NewRequest(http.MethodPost, "/user/1/transaction", bytes.NewBufferString(`{"state":"win","amount":"1.00","transactionId":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestAPI_PostTransaction_InsufficientFunds_Returns402(t *testing.T) {
	t.Parallel()

	store := &handlerMockStore{balance: domain.User{ID: 3, Balance: decimal.RequireFromString("0.00")}}
	api := NewAPI(service.NewBalanceService(store), metrics.New(), slog.Default())
	mux := http.NewServeMux()
	api.Register(mux)

	body := `{"state":"lose","amount":"1.00","transactionId":"tx-insufficient"}`
	req := httptest.NewRequest(http.MethodPost, "/user/3/transaction", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Source-Type", "payment")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusPaymentRequired, rec.Code)
}

func TestAPI_PostTransaction_Duplicate_Returns409(t *testing.T) {
	t.Parallel()

	store := &duplicateAwareStore{balance: domain.User{ID: 1, Balance: decimal.RequireFromString("10.00")}}
	api := NewAPI(service.NewBalanceService(store), metrics.New(), slog.Default())
	mux := http.NewServeMux()
	api.Register(mux)

	body := `{"state":"win","amount":"1.00","transactionId":"tx-dup"}`
	req := httptest.NewRequest(http.MethodPost, "/user/1/transaction", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Source-Type", "game")

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req2 := httptest.NewRequest(http.MethodPost, "/user/1/transaction", bytes.NewBufferString(body))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Source-Type", "game")

	rec2 := httptest.NewRecorder()
	mux.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusConflict, rec2.Code)
}

type handlerMockStore struct {
	balance domain.User
}

func (m *handlerMockStore) GetBalance(_ context.Context, userID uint64) (domain.User, error) {
	if m.balance.ID == 0 {
		m.balance.ID = userID
	}
	return m.balance, nil
}

func (m *handlerMockStore) ApplyTransaction(_ context.Context, tx domain.Transaction) (domain.User, error) {
	next, err := domain.ApplyDelta(m.balance.Balance, tx.State, tx.Amount)
	if err != nil {
		return domain.User{}, err
	}
	m.balance = domain.User{ID: tx.UserID, Balance: next}
	return m.balance, nil
}

type duplicateAwareStore struct {
	balance domain.User
	seen    map[string]struct{}
}

func (m *duplicateAwareStore) GetBalance(_ context.Context, userID uint64) (domain.User, error) {
	if m.balance.ID == 0 {
		m.balance.ID = userID
	}
	return m.balance, nil
}

func (m *duplicateAwareStore) ApplyTransaction(_ context.Context, tx domain.Transaction) (domain.User, error) {
	if m.seen == nil {
		m.seen = make(map[string]struct{})
	}
	if _, exists := m.seen[tx.TransactionID]; exists {
		return domain.User{}, domain.ErrDuplicateTransaction
	}
	next, err := domain.ApplyDelta(m.balance.Balance, tx.State, tx.Amount)
	if err != nil {
		return domain.User{}, err
	}
	m.seen[tx.TransactionID] = struct{}{}
	m.balance = domain.User{ID: tx.UserID, Balance: next}
	return m.balance, nil
}

func newTestService(balance decimal.Decimal) *service.BalanceService {
	return service.NewBalanceService(&handlerMockStore{balance: domain.User{ID: 1, Balance: balance}})
}
