package accrualpoller

import (
	"context"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGate_OpenByDefault(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := new(gate)

		start := time.Now()
		require.NoError(t, g.Wait(t.Context()))
		assert.Zero(t, time.Since(start))
	})
}

func TestGate_PauseBlocksUntilDeadline(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := new(gate)
		g.Pause(time.Minute)

		start := time.Now()
		require.NoError(t, g.Wait(t.Context()))
		assert.Equal(t, time.Minute, time.Since(start))
	})
}

func TestGate_PauseIsOnlyExtended(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		now := time.Now()
		g := new(gate)

		g.Pause(time.Minute)
		g.Pause(5 * time.Second)
		assert.Equal(t, now.Add(time.Minute), g.until, "shorter pause must not shorten the longer one")

		g.Pause(2 * time.Minute)
		assert.Equal(t, now.Add(2*time.Minute), g.until, "longer pause extends")
	})
}

func TestGate_ExtendedWhileWaiting(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := new(gate)
		g.Pause(30 * time.Second)

		go func() {
			time.Sleep(10 * time.Second)
			g.Pause(80 * time.Second)
		}()

		start := time.Now()
		require.NoError(t, g.Wait(t.Context()))
		assert.Equal(t, 90*time.Second, time.Since(start), "wait ends 80s after the pause was extended at 10s")
	})
}

func TestGate_WaitIsCanceledByContext(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		g := new(gate)
		g.Pause(time.Hour)

		ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
		defer cancel()

		start := time.Now()
		err := g.Wait(ctx)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		assert.Equal(t, 20*time.Second, time.Since(start), "shutdown must not wait for the pause to end")
	})
}
