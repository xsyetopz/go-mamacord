package scheduling

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeRestartsIntervalTasks(t *testing.T) {
	t.Parallel()

	runtime := NewRuntime(slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var first atomic.Int32
	runtime.Start(ctx, Plan{
		IntervalTasks: []IntervalTask{{
			ID:    "first",
			Every: 10 * time.Millisecond,
			Run: func(context.Context) {
				first.Add(1)
			},
		}},
	})

	waitFor(t, 250*time.Millisecond, func() bool {
		return first.Load() >= 2
	})
	beforeRestart := first.Load()

	var second atomic.Int32
	runtime.Restart(ctx, Plan{
		IntervalTasks: []IntervalTask{{
			ID:    "second",
			Every: 10 * time.Millisecond,
			Run: func(context.Context) {
				second.Add(1)
			},
		}},
	})

	waitFor(t, 250*time.Millisecond, func() bool {
		return second.Load() >= 2
	})
	time.Sleep(40 * time.Millisecond)
	afterRestart := first.Load()
	if afterRestart != beforeRestart {
		t.Fatalf("expected first interval task to stop on restart, before=%d after=%d", beforeRestart, afterRestart)
	}
}

func TestRuntimeKeepsIntervalsRunningWhenCronSpecIsInvalid(t *testing.T) {
	t.Parallel()

	runtime := NewRuntime(slog.Default())
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var runs atomic.Int32
	runtime.Start(ctx, Plan{
		CronTasks: []CronTask{{
			ID:       "bad-cron",
			Schedule: "not a cron",
			Run:      func(context.Context) {},
		}},
		IntervalTasks: []IntervalTask{{
			ID:    "poller",
			Every: 10 * time.Millisecond,
			Run: func(context.Context) {
				runs.Add(1)
			},
		}},
	})

	waitFor(t, 250*time.Millisecond, func() bool {
		return runs.Load() >= 2
	})
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not satisfied before timeout")
}
