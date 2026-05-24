package discordruntime

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	jobscheduling "github.com/xsyetopz/go-mamacord/internal/scheduling"
)

const reminderPollInterval = 5 * time.Second

type schedulerRuntime struct {
	runtime *jobscheduling.Runtime

	reminderEvery     time.Duration
	runReminderPoll   func(context.Context, string)
	enabledPluginJobs func() []pluginhost.PluginJob
	runPluginJob      func(context.Context, pluginhost.PluginJob)
}

func newSchedulerRuntime(
	logger *slog.Logger,
	reminderEvery time.Duration,
	runReminderPoll func(context.Context, string),
	enabledPluginJobs func() []pluginhost.PluginJob,
	runPluginJob func(context.Context, pluginhost.PluginJob),
) *schedulerRuntime {
	return &schedulerRuntime{
		runtime:           jobscheduling.NewRuntime(logger),
		reminderEvery:     reminderEvery,
		runReminderPoll:   runReminderPoll,
		enabledPluginJobs: enabledPluginJobs,
		runPluginJob:      runPluginJob,
	}
}

func (s *schedulerRuntime) Start(ctx context.Context) {
	if s == nil || s.runtime == nil {
		return
	}
	s.runtime.Start(ctx, s.plan())
}

func (s *schedulerRuntime) Stop() {
	if s == nil || s.runtime == nil {
		return
	}
	s.runtime.Stop()
}

func (s *schedulerRuntime) Restart(ctx context.Context) {
	if s == nil || s.runtime == nil {
		return
	}
	s.runtime.Restart(ctx, s.plan())
}

func (s *schedulerRuntime) plan() jobscheduling.Plan {
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
