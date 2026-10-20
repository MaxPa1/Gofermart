package rest

import (
	"compress/gzip"
	"net/http"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"go.uber.org/zap"

	"github.com/MaxPa1/Gofermart/internal/transport/rest/handler"
	"github.com/MaxPa1/Gofermart/internal/transport/rest/middleware"
)

func NewRouter(
	logger *zap.Logger,
	tokens middleware.TokenParser,
	auth *handler.AuthHandler,
	orders *handler.OrdersHandler,
	balance *handler.BalanceHandler,
) http.Handler {
	r := chi.NewRouter()

	r.Use(
		chimw.RequestID,
		middleware.Logger(logger),
		middleware.Recoverer(logger),
		middleware.Decompress,
		chimw.Compress(gzip.DefaultCompression),
	)

	r.Route("/api/user", func(r chi.Router) {
		r.Post("/register", auth.Register)
		r.Post("/login", auth.Login)

		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(tokens))

			r.Post("/orders", orders.Upload)
			r.Get("/orders", orders.List)

			r.Get("/balance", balance.Balance)
			r.Post("/balance/withdraw", balance.Withdraw)
			r.Get("/withdrawals", balance.Withdrawals)
		})
	})

	return r
}
