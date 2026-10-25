package accrualpoller

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

	"github.com/MaxPa1/Gofermart/internal/client/accrual"
	"github.com/MaxPa1/Gofermart/internal/model"
)

const testNumber = "12345678903"

var errInternal = errors.New("boom")

func newTestWorker(t *testing.T, cfg Config) (*Worker, *MockClient, *MockRepository) {
	t.Helper()
	ctrl := gomock.NewController(t)
	c := NewMockClient(ctrl)
	repo := NewMockRepository(ctrl)
	if cfg.PollInterval == 0 {
		cfg = Config{PollInterval: time.Hour, BatchSize: 10, Workers: 2}
	}
	return New(c, repo, cfg, zap.NewNop()), c, repo
}

func TestWorker_Process(t *testing.T) {
	tests := []struct {
		name    string
		order   model.Order
		result  accrual.Result
		err     error
		wantApp bool

		wantStatus  model.OrderStatus
		wantAccrual model.Amount
	}{
		{
			name:       "processed with accrual",
			order:      model.Order{Number: testNumber, Status: model.OrderStatusNew},
			result:     accrual.Result{Order: testNumber, Status: accrual.StatusProcessed, Accrual: 50000},
			wantApp:    true,
			wantStatus: model.OrderStatusProcessed, wantAccrual: 50000,
		},
		{
			name:       "processed without accrual",
			order:      model.Order{Number: testNumber, Status: model.OrderStatusProcessing},
			result:     accrual.Result{Order: testNumber, Status: accrual.StatusProcessed},
			wantApp:    true,
			wantStatus: model.OrderStatusProcessed,
		},
		{
			name:       "invalid",
			order:      model.Order{Number: testNumber, Status: model.OrderStatusNew},
			result:     accrual.Result{Order: testNumber, Status: accrual.StatusInvalid},
			wantApp:    true,
			wantStatus: model.OrderStatusInvalid,
		},
		{
			name:       "registered moves new order to processing",
			order:      model.Order{Number: testNumber, Status: model.OrderStatusNew},
			result:     accrual.Result{Order: testNumber, Status: accrual.StatusRegistered},
			wantApp:    true,
			wantStatus: model.OrderStatusProcessing,
		},
		{
			name:       "processing moves new order to processing",
			order:      model.Order{Number: testNumber, Status: model.OrderStatusNew},
			result:     accrual.Result{Order: testNumber, Status: accrual.StatusProcessing},
			wantApp:    true,
			wantStatus: model.OrderStatusProcessing,
		},
		{
			name:       "accrual in intermediate status is ignored",
			order:      model.Order{Number: testNumber, Status: model.OrderStatusNew},
			result:     accrual.Result{Order: testNumber, Status: accrual.StatusProcessing, Accrual: 100},
			wantApp:    true,
			wantStatus: model.OrderStatusProcessing, wantAccrual: 0,
		},
		{
			name:   "unchanged processing order is not written",
			order:  model.Order{Number: testNumber, Status: model.OrderStatusProcessing},
			result: accrual.Result{Order: testNumber, Status: accrual.StatusProcessing},
		},
		{
			name:  "not registered in accrual",
			order: model.Order{Number: testNumber, Status: model.OrderStatusNew},
			err:   accrual.ErrOrderNotRegistered,
		},
		{
			name:  "accrual error",
			order: model.Order{Number: testNumber, Status: model.OrderStatusNew},
			err:   errInternal,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w, c, repo := newTestWorker(t, Config{})
			c.EXPECT().GetOrder(gomock.Any(), testNumber).Return(tt.result, tt.err)
			if tt.wantApp {
				repo.EXPECT().ApplyAccrual(gomock.Any(), testNumber, tt.wantStatus, tt.wantAccrual).Return(true, nil)
			}

			w.process(context.Background(), tt.order)
		})
	}
}

func TestWorker_Process_AlreadyFinalAndApplyError(t *testing.T) {
	for name, applyErr := range map[string]error{"already final": nil, "apply error": errInternal} {
		t.Run(name, func(t *testing.T) {
			w, c, repo := newTestWorker(t, Config{})
			c.EXPECT().GetOrder(gomock.Any(), testNumber).
				Return(accrual.Result{Order: testNumber, Status: accrual.StatusProcessed, Accrual: 100}, nil)
			repo.EXPECT().ApplyAccrual(gomock.Any(), testNumber, model.OrderStatusProcessed, model.Amount(100)).
				Return(false, applyErr)

			w.process(context.Background(), model.Order{Number: testNumber, Status: model.OrderStatusNew})
		})
	}
}

func TestWorker_Process_RateLimitPausesAllRequests(t *testing.T) {
	w, c, _ := newTestWorker(t, Config{})
	c.EXPECT().GetOrder(gomock.Any(), testNumber).
		Return(accrual.Result{}, &accrual.RateLimitError{RetryAfter: time.Hour})

	w.process(context.Background(), model.Order{Number: testNumber, Status: model.OrderStatusNew})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	w.process(ctx, model.Order{Number: "79927398713", Status: model.OrderStatusNew})
	require.ErrorIs(t, w.gate.Wait(ctx), context.DeadlineExceeded)
}

func TestWorker_Poll(t *testing.T) {
	orders := []model.Order{
		{Number: "12345678903", Status: model.OrderStatusNew},
		{Number: "79927398713", Status: model.OrderStatusNew},
		{Number: "4561261212345467", Status: model.OrderStatusNew},
	}
	w, c, repo := newTestWorker(t, Config{PollInterval: time.Hour, BatchSize: 10, Workers: 2})

	repo.EXPECT().TakeOrdersToCheck(gomock.Any(), 10).Return(orders, nil)

	var inFlight, maxInFlight atomic.Int32
	var mu sync.Mutex
	seen := map[string]bool{}
	c.EXPECT().GetOrder(gomock.Any(), gomock.Any()).Times(len(orders)).
		DoAndReturn(func(_ context.Context, number string) (accrual.Result, error) {
			n := inFlight.Add(1)
			for {
				m := maxInFlight.Load()
				if n <= m || maxInFlight.CompareAndSwap(m, n) {
					break
				}
			}
			time.Sleep(20 * time.Millisecond)
			inFlight.Add(-1)

			mu.Lock()
			seen[number] = true
			mu.Unlock()
			return accrual.Result{Order: number, Status: accrual.StatusInvalid}, nil
		})
	repo.EXPECT().ApplyAccrual(gomock.Any(), gomock.Any(), model.OrderStatusInvalid, model.Amount(0)).
		Times(len(orders)).Return(true, nil)

	w.poll(context.Background())

	assert.Len(t, seen, len(orders), "every claimed order is processed")
	assert.LessOrEqual(t, maxInFlight.Load(), int32(2), "concurrency is bounded by Workers")
}

func TestWorker_Run_StopsOnContextCancel(t *testing.T) {
	w, _, repo := newTestWorker(t, Config{PollInterval: 10 * time.Millisecond, BatchSize: 10, Workers: 2})

	var polls atomic.Int32
	repo.EXPECT().TakeOrdersToCheck(gomock.Any(), 10).AnyTimes().
		DoAndReturn(func(context.Context, int) ([]model.Order, error) {
			polls.Add(1)
			return nil, nil
		})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- w.Run(ctx) }()

	require.Eventually(t, func() bool { return polls.Load() >= 3 }, time.Second, 5*time.Millisecond,
		"worker polls repeatedly with PollInterval")

	cancel()
	select {
	case err := <-done:
		require.NoError(t, err, "cancellation is a normal shutdown, not an error")
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after context cancellation")
	}
}

func TestNextState(t *testing.T) {
	tests := []struct {
		name        string
		in          accrual.Result
		wantStatus  model.OrderStatus
		wantAccrual model.Amount
		wantOK      bool
	}{
		{name: "registered", in: accrual.Result{Status: accrual.StatusRegistered}, wantStatus: model.OrderStatusProcessing, wantOK: true},
		{name: "processing", in: accrual.Result{Status: accrual.StatusProcessing}, wantStatus: model.OrderStatusProcessing, wantOK: true},
		{name: "processing ignores accrual", in: accrual.Result{Status: accrual.StatusProcessing, Accrual: 100}, wantStatus: model.OrderStatusProcessing, wantOK: true},
		{name: "invalid", in: accrual.Result{Status: accrual.StatusInvalid}, wantStatus: model.OrderStatusInvalid, wantOK: true},
		{name: "processed", in: accrual.Result{Status: accrual.StatusProcessed, Accrual: 50050}, wantStatus: model.OrderStatusProcessed, wantAccrual: 50050, wantOK: true},
		{name: "unknown", in: accrual.Result{Status: "UNKNOWN"}, wantOK: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, amount, ok := nextState(tt.in)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantStatus, status)
			assert.Equal(t, tt.wantAccrual, amount)
		})
	}
}
