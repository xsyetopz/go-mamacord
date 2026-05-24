package discordruntime

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

func TestSchedulerRuntimePlanIncludesReminderPollerAndPluginJobs(t *testing.T) {
	t.Parallel()

	var (
		reminderLease string
		ranJob        pluginhost.PluginJob
	)

	runtime := newSchedulerRuntime(
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

	bytes, err := os.ReadFile("lifecycle.go")
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

func TestModuleStateRestartsUnifiedScheduler(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("module_state.go")
	if err != nil {
		t.Fatalf("read module_state.go: %v", err)
	}
	text := string(bytes)
	if strings.Contains(text, "pluginAuto.Restart(ctx)") {
		t.Fatal("module_state.go still restarts plugin automation cron directly instead of the unified scheduler")
	}
	if !strings.Contains(text, "b.scheduler.Restart(ctx)") {
		t.Fatal("module_state.go should restart the unified scheduler after module reload")
	}
}
