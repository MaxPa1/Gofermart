package handler

import (
	"context"
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

const (
	testUserID  = int64(42)
	orderNumber = "12345678903"
)

func newTestOrdersHandler(t *testing.T) (*OrdersHandler, *MockOrdersService, *observer.ObservedLogs) {
	t.Helper()
	svc := NewMockOrdersService(gomock.NewController(t))
	core, logs := observer.New(zap.DebugLevel)
	return NewOrdersHandler(svc, zap.New(core)), svc, logs
}

func ordersRequest(method, contentType, body string) *http.Request {
	req := httptest.NewRequest(method, "/api/user/orders", strings.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return req.WithContext(middleware.WithUserID(req.Context(), testUserID))
}

func TestOrdersHandler_Upload(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantNumber  string
		svcErr      error
		wantStatus  int
	}{
		{name: "new order", contentType: "text/plain", body: orderNumber, wantNumber: orderNumber, wantStatus: http.StatusAccepted},
		{name: "already uploaded by user", contentType: "text/plain", body: orderNumber, wantNumber: orderNumber, svcErr: model.ErrOrderAlreadyUploaded, wantStatus: http.StatusOK},
		{name: "uploaded by another user", contentType: "text/plain", body: orderNumber, wantNumber: orderNumber, svcErr: model.ErrOrderOwnedByAnother, wantStatus: http.StatusConflict},
		{name: "invalid number", contentType: "text/plain", body: "12345678900", wantNumber: "12345678900", svcErr: model.ErrInvalidOrderNumber, wantStatus: http.StatusUnprocessableEntity},
		{name: "empty number", contentType: "text/plain", body: "", wantNumber: "", svcErr: fmt.Errorf("%w: order number is empty", model.ErrInvalidInput), wantStatus: http.StatusBadRequest},
		{name: "internal error", contentType: "text/plain", body: orderNumber, wantNumber: orderNumber, svcErr: errInternal, wantStatus: http.StatusInternalServerError},

		{name: "trailing newline is trimmed", contentType: "text/plain", body: orderNumber + "\n", wantNumber: orderNumber, wantStatus: http.StatusAccepted},
		{name: "charset parameter is accepted", contentType: "text/plain; charset=utf-8", body: orderNumber, wantNumber: orderNumber, wantStatus: http.StatusAccepted},
		{name: "missing content type is accepted", body: orderNumber, wantNumber: orderNumber, wantStatus: http.StatusAccepted},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, svc, _ := newTestOrdersHandler(t)
			svc.EXPECT().Upload(gomock.Any(), testUserID, tt.wantNumber).Return(tt.svcErr)

			rec := httptest.NewRecorder()
			h.Upload(rec, ordersRequest(http.MethodPost, tt.contentType, tt.body))

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestOrdersHandler_Upload_BadRequest(t *testing.T) {
	tests := []struct {
		name        string
		contentType string
		body        string
		wantStatus  int
	}{
		{name: "json content type", contentType: "application/json", body: `"12345678903"`, wantStatus: http.StatusBadRequest},
		{name: "malformed content type", contentType: "text/plain; =", body: orderNumber, wantStatus: http.StatusBadRequest},
		{name: "body too large", contentType: "text/plain", body: strings.Repeat("1", maxOrderBodySize+1), wantStatus: http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, _, _ := newTestOrdersHandler(t)

			rec := httptest.NewRecorder()
			h.Upload(rec, ordersRequest(http.MethodPost, tt.contentType, tt.body))

			assert.Equal(t, tt.wantStatus, rec.Code)
		})
	}
}

func TestOrdersHandler_List(t *testing.T) {
	msk := time.FixedZone("MSK", 3*60*60)

	t.Run("orders", func(t *testing.T) {
		h, svc, _ := newTestOrdersHandler(t)
		svc.EXPECT().List(gomock.Any(), testUserID).Return([]model.Order{
			{
				Number: "9278923470", Status: model.OrderStatusProcessed, Accrual: 50000,
				UploadedAt: time.Date(2020, 12, 10, 15, 15, 45, 123456789, msk),
			},
			{Number: "12345678903", Status: model.OrderStatusProcessing, UploadedAt: time.Date(2020, 12, 10, 15, 12, 1, 0, msk)},
			{Number: "346436439", Status: model.OrderStatusInvalid, UploadedAt: time.Date(2020, 12, 9, 16, 9, 53, 0, msk)},
		}, nil)

		rec := httptest.NewRecorder()
		h.List(rec, ordersRequest(http.MethodGet, "", ""))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
		assert.JSONEq(t, `[
			{"number": "9278923470", "status": "PROCESSED", "accrual": 500, "uploaded_at": "2020-12-10T15:15:45+03:00"},
			{"number": "12345678903", "status": "PROCESSING", "uploaded_at": "2020-12-10T15:12:01+03:00"},
			{"number": "346436439", "status": "INVALID", "uploaded_at": "2020-12-09T16:09:53+03:00"}
		]`, rec.Body.String())
	})

	t.Run("fractional accrual", func(t *testing.T) {
		h, svc, _ := newTestOrdersHandler(t)
		svc.EXPECT().List(gomock.Any(), testUserID).Return([]model.Order{
			{Number: orderNumber, Status: model.OrderStatusProcessed, Accrual: 72998, UploadedAt: time.Date(2020, 12, 10, 15, 0, 0, 0, time.UTC)},
		}, nil)

		rec := httptest.NewRecorder()
		h.List(rec, ordersRequest(http.MethodGet, "", ""))

		require.Equal(t, http.StatusOK, rec.Code)
		assert.JSONEq(t, `[{"number": "12345678903", "status": "PROCESSED", "accrual": 729.98, "uploaded_at": "2020-12-10T15:00:00Z"}]`, rec.Body.String())
	})

	t.Run("no orders", func(t *testing.T) {
		h, svc, _ := newTestOrdersHandler(t)
		svc.EXPECT().List(gomock.Any(), testUserID).Return(nil, nil)

		rec := httptest.NewRecorder()
		h.List(rec, ordersRequest(http.MethodGet, "", ""))

		assert.Equal(t, http.StatusNoContent, rec.Code)
		assert.Empty(t, rec.Body.String())
	})

	t.Run("internal error is logged but hidden from client", func(t *testing.T) {
		h, svc, logs := newTestOrdersHandler(t)
		svc.EXPECT().List(gomock.Any(), testUserID).Return(nil, errInternal)

		rec := httptest.NewRecorder()
		h.List(rec, ordersRequest(http.MethodGet, "", ""))

		assert.Equal(t, http.StatusInternalServerError, rec.Code)
		assert.NotContains(t, rec.Body.String(), errInternal.Error())
		require.Equal(t, 1, logs.Len())
		assert.Equal(t, errInternal.Error(), logs.All()[0].ContextMap()["error"])
	})
}

func TestOrdersHandler_MissingUserID(t *testing.T) {
	handlers := map[string]func(h *OrdersHandler) http.HandlerFunc{
		"upload": func(h *OrdersHandler) http.HandlerFunc { return h.Upload },
		"list":   func(h *OrdersHandler) http.HandlerFunc { return h.List },
	}
	for name, handle := range handlers {
		t.Run(name, func(t *testing.T) {
			h, _, logs := newTestOrdersHandler(t)

			req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(orderNumber))
			rec := httptest.NewRecorder()
			handle(h)(rec, req.WithContext(context.Background()))

			assert.Equal(t, http.StatusInternalServerError, rec.Code)
			assert.Equal(t, 1, logs.Len())
		})
	}
}
