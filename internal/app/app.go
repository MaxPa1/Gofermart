// Package app — создаёт и связывает зависимости (логгер, пул соединений с
// PostgreSQL, миграции, сервисы, хендлеры, клиент и воркер accrual), запускает HTTP-сервер и фоновый
// воркер и обеспечивает их корректное завершение.
package app

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/sync/errgroup"

	"github.com/MaxPa1/Gofermart/internal/auth/password"
	"github.com/MaxPa1/Gofermart/internal/auth/token"
	"github.com/MaxPa1/Gofermart/internal/client/accrual"
	"github.com/MaxPa1/Gofermart/internal/config"
	"github.com/MaxPa1/Gofermart/internal/repository"
	"github.com/MaxPa1/Gofermart/internal/service/auth"
	"github.com/MaxPa1/Gofermart/internal/service/balance"
	"github.com/MaxPa1/Gofermart/internal/service/orders"
	"github.com/MaxPa1/Gofermart/internal/transport/rest"
	"github.com/MaxPa1/Gofermart/internal/transport/rest/handler"
	"github.com/MaxPa1/Gofermart/internal/worker/accrualpoller"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 10 * time.Second
	writeTimeout      = 10 * time.Second
	idleTimeout       = 60 * time.Second
	shutdownTimeout   = 10 * time.Second
)

// Run
// При старте подключается к PostgreSQL и применяет миграции, затем параллельно запускает HTTP-сервер и
// воркер опроса accrual.
func Run(ctx context.Context, cfg config.Config) error {
	logger, err := newLogger(cfg.LogLevel)
	if err != nil {
		return fmt.Errorf("create logger: %w", err)
	}
	defer func() { _ = logger.Sync() }()

	pool, err := repository.NewPool(ctx, cfg.DatabaseURI, repository.PoolConfig{
		MaxConns:        cfg.DB.MaxConns,
		MinConns:        cfg.DB.MinConns,
		MaxConnLifetime: cfg.DB.MaxConnLifetime,
		MaxConnIdleTime: cfg.DB.MaxConnIdleTime,
	})
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	if err := repository.Migrate(ctx, pool); err != nil {
		return fmt.Errorf("migrate database: %w", err)
	}

	secret, err := jwtSecret(cfg.JWTSecret, logger)
	if err != nil {
		return err
	}

	hasher, err := password.NewHasher(bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("create password hasher: %w", err)
	}
	tokens, err := token.NewManager(secret, cfg.TokenTTL)
	if err != nil {
		return fmt.Errorf("create token manager: %w", err)
	}

	users := repository.NewUserRepository(pool)
	authService, err := auth.NewService(users, hasher, tokens)
	if err != nil {
		return fmt.Errorf("create auth service: %w", err)
	}
	authHandler := handler.NewAuthHandler(authService, logger, cfg.SecureCookie)

	ordersRepo := repository.NewOrderRepository(pool)
	ordersService := orders.NewService(ordersRepo)
	ordersHandler := handler.NewOrdersHandler(ordersService, logger)

	balanceRepo := repository.NewBalanceRepository(pool)
	balanceService := balance.NewService(balanceRepo)
	balanceHandler := handler.NewBalanceHandler(balanceService, logger)

	accrualClient, err := accrual.New(cfg.AccrualAddress, cfg.Accrual.RequestTimeout)
	if err != nil {
		return fmt.Errorf("create accrual client: %w", err)
	}
	worker := accrualpoller.New(accrualClient, repository.NewAccrualRepository(pool), accrualpoller.Config{
		PollInterval: cfg.Accrual.PollInterval,
		BatchSize:    cfg.Accrual.BatchSize,
		Workers:      cfg.Accrual.Workers,
	}, logger)

	srv := &http.Server{
		Addr:              cfg.RunAddress,
		Handler:           rest.NewRouter(logger, tokens, authHandler, ordersHandler, balanceHandler),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		ErrorLog:          zap.NewStdLog(logger),
	}

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error { return serve(gctx, srv, logger) })
	g.Go(func() error { return worker.Run(gctx) })
	return g.Wait()
}

func serve(ctx context.Context, srv *http.Server, logger *zap.Logger) error {
	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		logger.Info("starting HTTP server", zap.String("address", srv.Addr))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listen and serve: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gctx.Done()
		logger.Info("shutting down HTTP server")

		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		logger.Info("HTTP server stopped")
		return nil
	})

	return g.Wait()
}

func jwtSecret(configured string, logger *zap.Logger) ([]byte, error) {
	if configured != "" {
		return []byte(configured), nil
	}

	secret := make([]byte, token.MinSecretLength)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("generate JWT secret: %w", err)
	}
	logger.Warn("JWT_SECRET is not set, using a random secret: tokens will be invalidated on restart")
	return secret, nil
}
