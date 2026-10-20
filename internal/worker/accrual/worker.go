package accrual

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"

	client "github.com/MaxPa1/Gofermart/internal/client/accrual"
	"github.com/MaxPa1/Gofermart/internal/model"
)

//go:generate go tool mockgen -source=worker.go -destination=mocks_test.go -package=accrual -write_package_comment=false

type Client interface {
	GetOrder(ctx context.Context, number string) (client.Result, error)
}

type Repository interface {
	TakeOrdersToCheck(ctx context.Context, limit int) ([]model.Order, error)
	ApplyAccrual(ctx context.Context, number string, status model.OrderStatus, accrual model.Amount) (applied bool, err error)
}

type Config struct {
	PollInterval time.Duration
	BatchSize    int
	Workers      int
}

type Worker struct {
	client Client
	repo   Repository
	cfg    Config
	logger *zap.Logger
	gate   *gate
}

func New(accrualClient Client, repo Repository, cfg Config, logger *zap.Logger) *Worker {
	return &Worker{
		client: accrualClient,
		repo:   repo,
		cfg:    cfg,
		logger: logger,
		gate:   newGate(),
	}
}

func (w *Worker) Run(ctx context.Context) error {
	w.logger.Info("accrual worker started",
		zap.Duration("poll_interval", w.cfg.PollInterval),
		zap.Int("batch_size", w.cfg.BatchSize),
		zap.Int("workers", w.cfg.Workers),
	)

	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	for {
		w.poll(ctx)

		select {
		case <-ctx.Done():
			w.logger.Info("accrual worker stopped")
			return nil
		case <-ticker.C:
		}
	}
}

func (w *Worker) poll(ctx context.Context) {
	if err := w.gate.Wait(ctx); err != nil {
		return
	}

	orders, err := w.repo.TakeOrdersToCheck(ctx, w.cfg.BatchSize)
	if err != nil {
		if ctx.Err() == nil {
			w.logger.Error("take orders to check", zap.Error(err))
		}
		return
	}
	if len(orders) == 0 {
		return
	}

	g := new(errgroup.Group)
	g.SetLimit(w.cfg.Workers)
	for _, o := range orders {
		if ctx.Err() != nil {
			break
		}
		g.Go(func() error {
			w.process(ctx, o)
			return nil
		})
	}
	_ = g.Wait()
}

func (w *Worker) process(ctx context.Context, o model.Order) {
	if err := w.gate.Wait(ctx); err != nil {
		return
	}

	log := w.logger.With(zap.String("order", o.Number))

	res, err := w.client.GetOrder(ctx, o.Number)
	var rateLimit *client.RateLimitError
	switch {
	case errors.As(err, &rateLimit):
		w.gate.Pause(rateLimit.RetryAfter)
		log.Warn("accrual rate limit exceeded, pausing requests", zap.Duration("retry_after", rateLimit.RetryAfter))
		return
	case errors.Is(err, client.ErrOrderNotRegistered):
		log.Debug("order is not registered in accrual yet")
		return
	case err != nil:
		if ctx.Err() == nil {
			log.Warn("request accrual", zap.Error(err))
		}
		return
	}

	status, accrual, ok := nextState(res)
	if !ok {
		log.Error("unknown accrual status", zap.String("status", string(res.Status)))
		return
	}
	if status == o.Status && accrual == o.Accrual {
		return
	}

	applied, err := w.repo.ApplyAccrual(ctx, o.Number, status, accrual)
	if err != nil {
		if ctx.Err() == nil {
			log.Error("apply accrual", zap.Error(err))
		}
		return
	}
	if !applied {
		log.Debug("order is already final, accrual result skipped")
		return
	}
	log.Info("order accrual updated",
		zap.String("status", string(status)),
		zap.Stringer("accrual", accrual),
	)
}

func nextState(res client.Result) (model.OrderStatus, model.Amount, bool) {
	switch res.Status {
	case client.StatusRegistered, client.StatusProcessing:
		return model.OrderStatusProcessing, 0, true
	case client.StatusInvalid:
		return model.OrderStatusInvalid, 0, true
	case client.StatusProcessed:
		return model.OrderStatusProcessed, res.Accrual, true
	default:
		return "", 0, false
	}
}
