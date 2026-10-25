package handler

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/MaxPa1/Gofermart/internal/model"
	"github.com/MaxPa1/Gofermart/internal/transport/rest/middleware"
)

const withdrawOrder = "2377225624"

func newTestBalanceHandler(t *testing.T) (*BalanceHandler, *MockBalanceService, *observer.ObservedLogs) {
	t.Helper()
	svc := NewMockBalanceService(gomock.NewController(t))
	core, logs := observer.New(zap.DebugLevel)
	return NewBalanceHandler(svc, zap.New(core)), svc, logs
}

func balanceRequest(method, path, body string) *http.Request {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	return req.WithContext(middleware.WithUserID(req.Context(), testUserID))
}

func TestBalanceHandler_Balance(t *testing.T) {
	tests := []struct {
		name     string
		balance  model.Balance
		wantJSON string
	}{
		{name: "example from specification", balance: model.Balance{Current: 50050, Withdrawn: 4200}, wantJSON: `{"current": 500.5, "withdrawn": 42}`},
		{name: "zero balance", balance: model.Balance{}, wantJSON: `{"current": 0, "withdrawn": 0}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, svc, _ := newTestBalanceHandler(t)
			svc.EXPECT().Balance(gomock.Any(), testUserID).Return(tt.balance, nil)

			rec := httptest.NewRecorder()
			h.Balance(rec, balanceRequest(http.MethodGet, "/api/user/balance", ""))

			require.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.wantJSON, rec.Body.String())
		})
	}

	t.Run("internal error", func(t *testing.T) {
		h, svc, logs := newTestBalanceHandler(t)
		svc.EXPECT().Balance(gomock.Any(), testUserID).Return(model.Balance{}, errInternal)

		rec := httptest.NewRecorder()
		h.Balance(rec, balanceRequest(http.MethodGet, "/api/user/balance", ""))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, 1, logs.Len())
	})
}

func TestBalanceHandler_Withdraw(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantSum    model.Amount
		svcErr     error
		wantStatus int
	}{
		{name: "example from specification", body: `{"order": "2377225624", "sum": 751}`, wantSum: 75100, wantStatus: http.StatusOK},
		{name: "fractional sum", body: `{"order": "2377225624", "sum": 500.5}`, wantSum: 50050, wantStatus: http.StatusOK},
		{name: "insufficient funds", body: `{"order": "2377225624", "sum": 751}`, wantSum: 75100, svcErr: model.ErrInsufficientFunds, wantStatus: http.StatusPaymentRequired},
		{name: "invalid order number", body: `{"order": "2377225624", "sum": 751}`, wantSum: 75100, svcErr: model.ErrInvalidOrderNumber, wantStatus: http.StatusUnprocessableEntity},
		{name: "non-positive sum", body: `{"order": "2377225624", "sum": 0}`, wantSum: 0, svcErr: fmt.Errorf("%w: sum must be positive", model.ErrInvalidInput), wantStatus: http.StatusBadRequest},
		{name: "internal error", body: `{"order": "2377225624", "sum": 751}`, wantSum: 75100, svcErr: errInternal, wantStatus: http.StatusInternalServerError},
		{name: "null sum", body: `{"order": "2377225624", "sum": null}`, wantSum: 0, svcErr: model.ErrInvalidInput, wantStatus: http.StatusBadRequest},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, svc, _ := newTestBalanceHandler(t)
			svc.EXPECT().Withdraw(gomock.Any(), testUserID, withdrawOrder, tt.wantSum).Return(tt.svcErr)

			rec := httptest.NewRecorder()
			h.Withdraw(rec, balanceRequest(http.MethodPost, "/api/user/balance/withdraw", tt.body))

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestBalanceHandler_Withdraw_BadRequest(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantStatus int
	}{
		{name: "invalid JSON", body: `{"order": `, wantStatus: http.StatusBadRequest},
		{name: "unknown field", body: `{"order": "2377225624", "summ": 751}`, wantStatus: http.StatusBadRequest},
		{name: "trailing data", body: `{"order": "2377225624", "sum": 751} garbage`, wantStatus: http.StatusBadRequest},
		{name: "two objects", body: `{"order": "2377225624", "sum": 751}{"order": "2377225624", "sum": 1}`, wantStatus: http.StatusBadRequest},
		{name: "sum as string", body: `{"order": "2377225624", "sum": "751"}`, wantStatus: http.StatusBadRequest},
		{name: "too many decimals", body: `{"order": "2377225624", "sum": 1.234}`, wantStatus: http.StatusBadRequest},
		{name: "exponent", body: `{"order": "2377225624", "sum": 7.51e2}`, wantStatus: http.StatusBadRequest},
		{name: "order as number", body: `{"order": 2377225624, "sum": 751}`, wantStatus: http.StatusBadRequest},
		{name: "body too large", body: `{"order": "` + strings.Repeat("1", maxJSONBodySize) + `", "sum": 1}`, wantStatus: http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _ := newTestBalanceHandler(t)

			rec := httptest.NewRecorder()
			h.Withdraw(rec, balanceRequest(http.MethodPost, "/api/user/balance/withdraw", tt.body))

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestBalanceHandler_Withdrawals(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)

	t.Run("withdrawals", func(t *testing.T) {
		h, svc, _ := newTestBalanceHandler(t)
		svc.EXPECT().Withdrawals(gomock.Any(), testUserID).Return([]model.Withdrawal{
			{UserID: testUserID, OrderNumber: withdrawOrder, Sum: 50000, ProcessedAt: time.Date(2020, 12, 9, 16, 9, 57, 123456789, msk)},
			{UserID: testUserID, OrderNumber: "79927398713", Sum: 1050, ProcessedAt: time.Date(2020, 12, 8, 10, 0, 0, 0, msk)},
		}, nil)

		rec := httptest.NewRecorder()
		h.Withdrawals(rec, balanceRequest(http.MethodGet, "/api/user/withdrawals", ""))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.JSONEq(t, `[
			{"order": "2377225624", "sum": 500, "processed_at": "2020-12-09T16:09:57+03:00"},
			{"order": "79927398713", "sum": 10.5, "processed_at": "2020-12-08T10:00:00+03:00"}
		]`, rec.Body.String())
	})

	t.Run("no withdrawals", func(t *testing.T) {
		h, svc, _ := newTestBalanceHandler(t)
		svc.EXPECT().Withdrawals(gomock.Any(), testUserID).Return(nil, nil)

		rec := httptest.NewRecorder()
		h.Withdrawals(rec, balanceRequest(http.MethodGet, "/api/user/withdrawals", ""))

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("internal error", func(t *testing.T) {
		h, svc, logs := newTestBalanceHandler(t)
		svc.EXPECT().Withdrawals(gomock.Any(), testUserID).Return(nil, errInternal)

		rec := httptest.NewRecorder()
		h.Withdrawals(rec, balanceRequest(http.MethodGet, "/api/user/withdrawals", ""))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.Equal(t, 1, logs.Len())
	})
}

func TestBalanceHandler_MissingUserID(t *testing.T) {
	handlers := map[string]func(h *BalanceHandler) http.HandlerFunc{
		"balance":     func(h *BalanceHandler) http.HandlerFunc { return h.Balance },
		"withdraw":    func(h *BalanceHandler) http.HandlerFunc { return h.Withdraw },
		"withdrawals": func(h *BalanceHandler) http.HandlerFunc { return h.Withdrawals },
	}
	for name, handle := range handlers {
		t.Run(name, func(t *testing.T) {
			h, _, logs := newTestBalanceHandler(t)

			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"order":"2377225624","sum":1}`))
			rec := httptest.NewRecorder()
			handle(h)(rec, req)

			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.Equal(t, 1, logs.Len())
		})
	}
}
