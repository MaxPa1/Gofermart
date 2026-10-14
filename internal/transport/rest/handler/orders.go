package handler

import (
	"context"
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"

	"github.com/MaxPa1/Gofermart/internal/model"
)

//go:generate go tool mockgen -source=orders.go -destination=mocks_orders_test.go -package=handler -write_package_comment=false

const maxOrderBodySize = 1 << 10

type OrdersService interface {
	Upload(ctx context.Context, userID int64, number string) error
	List(ctx context.Context, userID int64) ([]model.Order, error)
}

type orderResponse struct {
	Number     string     `json:"number"`
	Status     string     `json:"status"`
	Accrual    jsonAmount `json:"accrual,omitempty"`
	UploadedAt string     `json:"uploaded_at"`
}

type OrdersHandler struct {
	orders OrdersService
	logger *zap.Logger
}

func NewOrdersHandler(orders OrdersService, logger *zap.Logger) *OrdersHandler {
	return &OrdersHandler{orders: orders, logger: logger}
}

func (h *OrdersHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r, h.logger, "upload order")
	if !ok {
		return
	}

	if !isTextPlain(r.Header.Get("Content-Type")) {
		writeError(w, http.StatusBadRequest, "content type must be text/plain")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxOrderBodySize)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		var maxBytesErr *http.MaxBytesError
		if errors.As(err, &maxBytesErr) {
			writeError(w, http.StatusRequestEntityTooLarge, "")
			return
		}
		writeError(w, http.StatusBadRequest, "cannot read request body")
		return
	}

	number := strings.TrimSpace(string(body))

	err = h.orders.Upload(r.Context(), userID, number)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusAccepted)
	case errors.Is(err, model.ErrOrderAlreadyUploaded):
		w.WriteHeader(http.StatusOK)
	case errors.Is(err, model.ErrOrderOwnedByAnother):
		writeError(w, http.StatusConflict, "")
	case errors.Is(err, model.ErrInvalidOrderNumber):
		writeError(w, http.StatusUnprocessableEntity, "")
	case errors.Is(err, model.ErrInvalidInput):
		writeError(w, http.StatusBadRequest, err.Error())
	default:
		writeInternalError(w, r, h.logger, "upload order", err)
	}
}

func (h *OrdersHandler) List(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDFromRequest(w, r, h.logger, "list orders")
	if !ok {
		return
	}

	orders, err := h.orders.List(r.Context(), userID)
	if err != nil {
		writeInternalError(w, r, h.logger, "list orders", err)
		return
	}
	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	resp := make([]orderResponse, 0, len(orders))
	for _, o := range orders {
		resp = append(resp, orderResponse{
			Number:     o.Number,
			Status:     string(o.Status),
			Accrual:    jsonAmount(o.Accrual),
			UploadedAt: o.UploadedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, r, h.logger, "list orders", http.StatusOK, resp)
}

func isTextPlain(contentType string) bool {
	if contentType == "" {
		return true
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	return err == nil && mediaType == "text/plain"
}
