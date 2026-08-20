package automation

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/host"
)

func TestSchedulerRuntimePlanIncludesReminderPollerAndPluginJobs(t *testing.T) {
	t.Parallel()

	var (
		reminderLease string
		ranJob        pluginhost.PluginJob
	)

	runtime := NewScheduler(
		slog.Default(),
		5*time.Second,
		func(_ context.Context, leaseID string) { reminderLease = leaseID },
		func() []pluginhost.PluginJob {
			return []pluginhost.PluginJob{{
				PluginID: "wellness",
				JobID:    "daily",
				Schedule: "0 9 * * *",
			}}
		},
		func(_ context.Context, job pluginhost.PluginJob) { ranJob = job },
	)

	plan := runtime.plan()
	if len(plan.IntervalTasks) != 1 {
		t.Fatalf("expected one reminder poller, got %d", len(plan.IntervalTasks))
	}
	if len(plan.CronTasks) != 1 {
		t.Fatalf("expected one plugin cron job, got %d", len(plan.CronTasks))
	}
	if plan.CronTasks[0].Schedule != "0 9 * * *" {
		t.Fatalf("unexpected cron schedule: %q", plan.CronTasks[0].Schedule)
	}

	plan.IntervalTasks[0].Run(context.Background())
	if reminderLease == "" {
		t.Fatal("expected reminder poller closure to capture a non-empty lease id")
	}

	plan.CronTasks[0].Run(context.Background())
	if ranJob.PluginID != "wellness" || ranJob.JobID != "daily" {
		t.Fatalf("unexpected plugin job execution payload: %#v", ranJob)
	}
}

func TestLifecycleUsesUnifiedSchedulerStartStop(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile(filepath.Join("..", "lifecycle.go"))
	if err != nil {
		t.Fatalf("read lifecycle.go: %v", err)
	}
	text := string(bytes)
	if strings.Contains(text, "pluginAuto.Start(ctx)") {
		t.Fatal("lifecycle.go still starts plugin automation cron directly instead of the unified scheduler")
	}
	if strings.Contains(text, "startReminderScheduler(ctx)") {
		t.Fatal("lifecycle.go still starts reminder polling directly instead of the unified scheduler")
	}
	if !strings.Contains(text, "b.scheduler.Start(ctx)") {
		t.Fatal("lifecycle.go should start the unified scheduler")
	}
	if !strings.Contains(text, "b.scheduler.Stop()") {
		t.Fatal("lifecycle.go should stop the unified scheduler")
	}
}

func TestCatalogRuntimeRestartsUnifiedScheduler(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile(filepath.Join("..", "catalog.go"))
	if err != nil {
		t.Fatalf("read catalog.go: %v", err)
	}
	text := string(bytes)
	if strings.Contains(text, "pluginAuto.Restart(ctx)") {
		t.Fatal("catalog.go still restarts plugin automation cron directly instead of the unified scheduler")
	}
	if !strings.Contains(text, "b.scheduler.Restart(ctx)") {
		t.Fatal("catalog.go should restart the unified scheduler after module reload")
	}
}

func TestRemindersGoDoesNotOwnTickerLifecycle(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("reminders.go")
	if err != nil {
		t.Fatalf("read reminders.go: %v", err)
	}
	text := string(bytes)
	if strings.Contains(text, "func (r Reminders) Start(") {
		t.Fatal("reminders.go still owns start lifecycle instead of the unified scheduler")
	}
	if strings.Contains(text, "time.NewTicker(") {
		t.Fatal("reminders.go still owns ticker lifecycle instead of the unified scheduler")
	}
	if !strings.Contains(text, "func (r Reminders) PollDue(") {
		t.Fatal("reminders.go should expose one reminder polling unit for the unified scheduler")
	}
}
