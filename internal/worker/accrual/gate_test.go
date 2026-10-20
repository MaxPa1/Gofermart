package accrual

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGate_OpenByDefault(t *testing.T) {
	g := newGate()

	start := time.Now()
	require.NoError(t, g.Wait(context.Background()))
	assert.Less(t, time.Since(start), 10*time.Millisecond)
}

func TestGate_PauseBlocksUntilDeadline(t *testing.T) {
	g := newGate()
	g.Pause(50 * time.Millisecond)

	start := time.Now()
	require.NoError(t, g.Wait(context.Background()))
	assert.GreaterOrEqual(t, time.Since(start), 45*time.Millisecond)
}

func TestGate_PauseIsOnlyExtended(t *testing.T) {
	now := time.Date(2026, 10, 18, 12, 0, 0, 0, time.UTC)
	g := newGate()
	g.now = func() time.Time { return now }

	g.Pause(time.Minute)
	g.Pause(5 * time.Second)
	assert.Equal(t, now.Add(time.Minute), g.until, "shorter pause must not shorten the longer one")

	g.Pause(2 * time.Minute)
	assert.Equal(t, now.Add(2*time.Minute), g.until, "longer pause extends")
}

func TestGate_ExtendedWhileWaiting(t *testing.T) {
	g := newGate()
	g.Pause(30 * time.Millisecond)

	go func() {
		time.Sleep(10 * time.Millisecond)
		g.Pause(80 * time.Millisecond)
	}()

	start := time.Now()
	require.NoError(t, g.Wait(context.Background()))
	assert.GreaterOrEqual(t, time.Since(start), 80*time.Millisecond)
}

func TestGate_WaitIsCanceledByContext(t *testing.T) {
	g := newGate()
	g.Pause(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	start := time.Now()
	err := g.Wait(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
	assert.Less(t, time.Since(start), time.Second, "shutdown must not wait for the pause to end")
}
