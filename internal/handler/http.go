package handler

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/achify/entain-test-task/internal/domain"
	"github.com/achify/entain-test-task/internal/metrics"
	"github.com/achify/entain-test-task/internal/service"
)

// API exposes HTTP endpoints for balance operations.
type API struct {
	svc     *service.BalanceService
	metrics *metrics.Collector
	logger  *slog.Logger
}

// NewAPI wires HTTP handlers with dependencies.
func NewAPI(svc *service.BalanceService, metrics *metrics.Collector, logger *slog.Logger) *API {
	return &API{svc: svc, metrics: metrics, logger: logger}
}

const (
	routeGetBalance      = "/user/:id/balance"
	routePostTransaction = "/user/:id/transaction"
)

// Register mounts routes on the provided mux.
func (a *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /healthz", a.handleHealth)
	mux.HandleFunc("GET /user/{userId}/balance", a.instrument(routeGetBalance, a.handleGetBalance))
	mux.HandleFunc("POST /user/{userId}/transaction", a.instrument(routePostTransaction, a.handlePostTransaction))
}

func (a *API) instrument(route string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next(rec, r)
		a.metrics.ObserveRequest(r.Method, route, rec.status, time.Since(start))
	}
}

func (a *API) handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func (a *API) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(r.PathValue("userId"))
	if err != nil {
		a.metrics.IncTxRejected("invalid_user_id")
		writeError(w, http.StatusBadRequest, err)
		return
	}

	user, err := a.svc.GetBalance(r.Context(), userID)
	if err != nil {
		writeDomainError(w, a.metrics, err)
		return
	}

	writeJSON(w, http.StatusOK, toBalanceResponseDTO(user))
}

func (a *API) handlePostTransaction(w http.ResponseWriter, r *http.Request) {
	userID, err := parseUserID(r.PathValue("userId"))
	if err != nil {
		a.metrics.IncTxRejected("invalid_user_id")
		writeError(w, http.StatusBadRequest, err)
		return
	}

	sourceType := strings.TrimSpace(r.Header.Get("Source-Type"))
	if sourceType == "" {
		a.metrics.IncTxRejected("missing_source_type")
		writeError(w, http.StatusBadRequest, errors.New("missing Source-Type header"))
		return
	}

	var body transactionRequestDTO
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		a.metrics.IncTxRejected("invalid_json")
		writeError(w, http.StatusBadRequest, errors.New("invalid json body"))
		return
	}

	user, err := a.svc.ProcessTransaction(r.Context(), service.ProcessTransactionCommand{
		UserID:        userID,
		SourceType:    sourceType,
		State:         body.State,
		Amount:        body.Amount,
		TransactionID: body.TransactionID,
	})
	if err != nil {
		writeDomainError(w, a.metrics, err)
		return
	}

	a.metrics.IncTxApplied()
	writeJSON(w, http.StatusOK, toBalanceResponseDTO(user))
}

func parseUserID(raw string) (uint64, error) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, service.ErrInvalidUserID
	}
	return id, nil
}

func writeDomainError(w http.ResponseWriter, m *metrics.Collector, err error) {
	switch {
	case errors.Is(err, domain.ErrUserNotFound):
		m.IncTxRejected("user_not_found")
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, domain.ErrDuplicateTransaction):
		m.IncTxRejected("duplicate_transaction")
		writeError(w, http.StatusConflict, err)
	case errors.Is(err, domain.ErrInsufficientFunds):
		m.IncTxRejected("insufficient_funds")
		m.IncInsufficientFunds()
		writeError(w, http.StatusPaymentRequired, err)
	case errors.Is(err, domain.ErrInvalidSourceType),
		errors.Is(err, domain.ErrInvalidState),
		errors.Is(err, service.ErrInvalidAmount),
		errors.Is(err, service.ErrInvalidTransactionID),
		errors.Is(err, service.ErrInvalidUserID):
		m.IncTxRejected("validation")
		writeError(w, http.StatusBadRequest, err)
	default:
		m.IncTxRejected("internal")
		writeError(w, http.StatusInternalServerError, errors.New("internal server error"))
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
