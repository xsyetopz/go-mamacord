package automation

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/host"
	jobscheduling "github.com/xsyetopz/go-mamacord/internal/scheduling"
)

const DefaultReminderPollInterval = 5 * time.Second

type Scheduler struct {
	runtime *jobscheduling.Runtime

	reminderEvery     time.Duration
	runReminderPoll   func(context.Context, string)
	enabledPluginJobs func() []pluginhost.PluginJob
	runPluginJob      func(context.Context, pluginhost.PluginJob)
}

func NewScheduler(
	logger *slog.Logger,
	reminderEvery time.Duration,
	runReminderPoll func(context.Context, string),
	enabledPluginJobs func() []pluginhost.PluginJob,
	runPluginJob func(context.Context, pluginhost.PluginJob),
) *Scheduler {
	return &Scheduler{
		runtime:           jobscheduling.NewRuntime(logger),
		reminderEvery:     reminderEvery,
		runReminderPoll:   runReminderPoll,
		enabledPluginJobs: enabledPluginJobs,
		runPluginJob:      runPluginJob,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	if s == nil || s.runtime == nil {
		return
	}
	s.runtime.Start(ctx, s.plan())
}

func (s *Scheduler) Stop() {
	if s == nil || s.runtime == nil {
		return
	}
	s.runtime.Stop()
}

func (s *Scheduler) Restart(ctx context.Context) {
	if s == nil || s.runtime == nil {
		return
	}
	s.runtime.Restart(ctx, s.plan())
}

func (s *Scheduler) plan() jobscheduling.Plan {
	var plan jobscheduling.Plan

	if s.runReminderPoll != nil && s.reminderEvery > 0 {
		leaseID := uuid.NewString()
		plan.IntervalTasks = append(plan.IntervalTasks, jobscheduling.IntervalTask{
			ID:    "reminders.poll_due",
			Every: s.reminderEvery,
			Run: func(ctx context.Context) {
				s.runReminderPoll(ctx, leaseID)
			},
		})
	}

	if s.enabledPluginJobs != nil && s.runPluginJob != nil {
		for _, job := range s.enabledPluginJobs() {
			job := job
			if strings.TrimSpace(job.PluginID) == "" || strings.TrimSpace(job.JobID) == "" || strings.TrimSpace(job.Schedule) == "" {
				continue
			}
			plan.CronTasks = append(plan.CronTasks, jobscheduling.CronTask{
				ID:       "plugin_job:" + strings.TrimSpace(job.PluginID) + ":" + strings.TrimSpace(job.JobID),
				Schedule: job.Schedule,
				Run: func(ctx context.Context) {
					s.runPluginJob(ctx, job)
				},
			})
		}
	}

	return plan
}
