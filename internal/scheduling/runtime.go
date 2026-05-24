package scheduling

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

type IntervalTask struct {
	ID    string
	Every time.Duration
	Run   func(context.Context)
}

type CronTask struct {
	ID       string
	Schedule string
	Run      func(context.Context)
}

type Plan struct {
	IntervalTasks []IntervalTask
	CronTasks     []CronTask
}

type Runtime struct {
	logger *slog.Logger

	mu     sync.Mutex
	cancel context.CancelFunc
	cron   *cron.Cron
}

func NewRuntime(logger *slog.Logger) *Runtime {
	if logger == nil {
		logger = slog.Default()
	}
	return &Runtime{logger: logger}
}

func (r *Runtime) Start(ctx context.Context, plan Plan) {
	if r == nil {
		return
	}

	r.mu.Lock()
	if r.cancel != nil || r.cron != nil {
		r.mu.Unlock()
		return
	}
	runCtx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	c := cron.New(cron.WithParser(parser))
	hasCronTasks := false
	for _, task := range plan.CronTasks {
		task := task
		if strings.TrimSpace(task.ID) == "" || strings.TrimSpace(task.Schedule) == "" || task.Run == nil {
			continue
		}
		if _, err := c.AddFunc(task.Schedule, func() {
			task.Run(runCtx)
		}); err != nil {
			r.logger.WarnContext(
				ctx,
				"invalid scheduler cron task",
				slog.String("task", task.ID),
				slog.String("schedule", task.Schedule),
				slog.String("err", err.Error()),
			)
			continue
		}
		hasCronTasks = true
	}
	if hasCronTasks {
		r.cron = c
	}
	r.mu.Unlock()

	for _, task := range plan.IntervalTasks {
		task := task
		if strings.TrimSpace(task.ID) == "" || task.Every <= 0 || task.Run == nil {
			continue
		}
		go r.runIntervalTask(runCtx, task)
	}

	if hasCronTasks {
		c.Start()
	}
}

func (r *Runtime) runIntervalTask(ctx context.Context, task IntervalTask) {
	ticker := time.NewTicker(task.Every)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			task.Run(ctx)
		}
	}
}

func (r *Runtime) Stop() {
	if r == nil {
		return
	}

	r.mu.Lock()
	cancel := r.cancel
	c := r.cron
	r.cancel = nil
	r.cron = nil
	r.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	if c != nil {
		<-c.Stop().Done()
	}
}

func (r *Runtime) Restart(ctx context.Context, plan Plan) {
	if r == nil {
		return
	}
	r.Stop()
	r.Start(ctx, plan)
}
