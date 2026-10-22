package accrual

import (
	"context"
	"sync"
	"time"
)

type gate struct {
	mu    sync.Mutex
	until time.Time
	now   func() time.Time
}

func newGate() *gate {
	return &gate{now: time.Now}
}

// Pause приостанавливает все вызовы Wait на d от текущего момента.
func (g *gate) Pause(d time.Duration) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if until := g.now().Add(d); until.After(g.until) {
		g.until = until
	}
}

// Wait блокируется до окончания паузы или отмены ctx; во втором случае возвращает ctx.Err().
func (g *gate) Wait(ctx context.Context) error {
	for {
		g.mu.Lock()
		d := g.until.Sub(g.now())
		g.mu.Unlock()
		if d <= 0 {
			return nil
		}

		t := time.NewTimer(d)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}
