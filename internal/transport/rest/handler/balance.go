package handler

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/MaxPa1/Gofermart/internal/model"
)

//go:generate go tool mockgen -source=balance.go -destination=mocks_balance_test.go -package=handler -write_package_comment=false

const maxWithdrawBodySize = 4 << 10

type BalanceService interface {
	Balance(ctx context.Context, userID int64) (model.Balance, error)
	Withdraw(ctx context.Context, userID int64, orderNumber string, sum model.Amount) error
	Withdrawals(ctx context.Context, userID int64) ([]model.Withdrawal, error)
}

type balanceResponse struct {
	Current   jsonAmount `json:"current"`
	Withdrawn jsonAmount `json:"withdrawn"`
}

type withdrawRequest struct {
	Order string     `json:"order"`
	Sum   jsonAmount `json:"sum"`
}

type withdrawalResponse struct {
	Order       string     `json:"order"`
	Sum         jsonAmount `json:"sum"`
	ProcessedAt string     `json:"processed_at"`
}

type BalanceHandler struct {
	balance BalanceService
	logger  *zap.Logger
}

func NewBalanceHandler(balance BalanceService, logger *zap.Logger) *BalanceHandler {
	return &BalanceHandler{balance: balance, logger: logger}
}

func (h *BalanceHandler) Balance(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r, h.logger, "get balance")
	if !ok {
		return
	}

	b, err := h.balance.Balance(r.Context(), userID)
	if err != nil {
		writeInternalError(w, r, h.logger, "get balance", err)
		return
	}

	writeJSON(w, r, h.logger, "get balance", http.StatusOK, balanceResponse{
		Current:   jsonAmount(b.Current),
		Withdrawn: jsonAmount(b.Withdrawn),
	})
}

func (h *BalanceHandler) Withdraw(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r, h.logger, "withdraw")
	if !ok {
		return
	}

	req, ok := decodeWithdrawRequest(w, r)
	if !ok {
		return
	}

	err := h.balance.Withdraw(r.Context(), userID, req.Order, model.Amount(req.Sum))
	switch {
	case err == nil:
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, model.ErrInsufficientFunds):
		writeError(w, http.StatusPaymentRequired, "")
	case errors.Is(err, model.ErrInvalidOrderNumber):
		writeError(w, http.StatusUnprocessableEntity, "")
	case errors.Is(err, model.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeInternalError(w, r, h.logger, "withdraw", err)
	}
}

func (h *BalanceHandler) Withdrawals(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r, h.logger, "list withdrawals")
	if !ok {
		return
	}

	withdrawals, err := h.balance.Withdrawals(r.Context(), userID)
	if err != nil {
		writeInternalError(w, r, h.logger, "list withdrawals", err)
		return
	}
	if len(withdrawals) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	resp := make([]withdrawalResponse, 0, len(withdrawals))
	for _, wd := range withdrawals {
		resp = append(resp, withdrawalResponse{
			Order:       wd.OrderNumber,
			Sum:         jsonAmount(wd.Sum),
			ProcessedAt: wd.ProcessedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, r, h.logger, "list withdrawals", http.StatusOK, resp)
}

func decodeWithdrawRequest(w http.ResponseWriter, r *http.Request) (withdrawRequest, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxWithdrawBodySize)

	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req withdrawRequest
	err := dec.Decode(&req)
	if err == nil && !errors.Is(dec.Decode(&struct{}{}), io.EOF) {
		err = errors.New("unexpected data after JSON object")
	}
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		switch {
		case errors.As(err, &maxBytesErr):
			writeError(w, http.StatusRequestEntityTooLarge, "")
		case errors.Is(err, errInvalidAmount):
			writeError(w, http.StatusBadRequest, "sum must be a number with at most two decimal places")
		default:
			writeError(w, http.StatusBadRequest, "invalid JSON body")
		}
		return withdrawRequest{}, false
	}

	return req, true
}
