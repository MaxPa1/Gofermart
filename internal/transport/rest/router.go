// Package rest — HTTP-транспорт сервиса: маршрутизатор на http.ServeMux с маршрутами API «Гофермарт» и
// общей цепочкой middleware.
package rest

import (
	"net/http"
	"slices"

	"go.uber.org/zap"

	"github.com/MaxPa1/Gofermart/internal/transport/rest/handler"
	"github.com/MaxPa1/Gofermart/internal/transport/rest/middleware"
)

// NewRouter собирает HTTP-обработчик API. Каждый запрос проходит цепочку RequestID → Logger →
// Recoverer → Decompress → Compress
func NewRouter(
	logger *zap.Logger,
	tokens middleware.TokenParser,
	auth *handler.AuthHandler,
	orders *handler.OrdersHandler,
	balance *handler.BalanceHandler,
) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/user/register", auth.Register)
	mux.HandleFunc("POST /api/user/login", auth.Login)

	protected := middleware.Auth(tokens)
	mux.Handle("POST /api/user/orders", protected(http.HandlerFunc(orders.Upload)))
	mux.Handle("GET /api/user/orders", protected(http.HandlerFunc(orders.List)))
	mux.Handle("GET /api/user/balance", protected(http.HandlerFunc(balance.Balance)))
	mux.Handle("POST /api/user/balance/withdraw", protected(http.HandlerFunc(balance.Withdraw)))
	mux.Handle("GET /api/user/withdrawals", protected(http.HandlerFunc(balance.Withdrawals)))

	return chain(mux,
		middleware.RequestID,
		middleware.Logger(logger),
		middleware.Recoverer(logger),
		middleware.Decompress,
		middleware.Compress,
	)
}

func chain(h http.Handler, middlewares ...func(http.Handler) http.Handler) http.Handler {
	for _, mw := range slices.Backward(middlewares) {
		h = mw(h)
	}
	return h
}
