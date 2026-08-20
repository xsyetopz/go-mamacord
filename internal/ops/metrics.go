package ops

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type Snapshot struct {
	Runtime  RuntimeSnapshot
	Modules  ModuleSnapshot
	Plugins  PluginSnapshot
	Commands CommandSnapshot
	Activity ActivitySnapshot
}

type RuntimeSnapshot struct {
	Ready            bool
	StartedAt        time.Time
	MigrationVersion int
	ProdMode         bool
	// DiscordStartError is a dev-focused diagnostic string when the Discord bot
	// fails to connect (bad token, missing intents, etc). Empty means no error.
	DiscordStartError string
}

type ModuleSnapshot struct {
	Total   int
	Enabled int
}

type PluginSnapshot struct {
	Total   int
	Enabled int
}

type CommandSnapshot struct {
	Builtin int
	Slash   int
	User    int
	Message int
}

type ActivitySnapshot struct {
	InteractionsTotal   uint64
	InteractionFailures uint64
	PluginFailures      uint64
	AutomationFailures  uint64
	ReminderFailures    uint64
}

type SnapshotFunc func() Snapshot

type Metrics struct {
	startedAt time.Time

	interactionsTotal   atomic.Uint64
	interactionFailures atomic.Uint64
	pluginFailures      atomic.Uint64
	automationFailures  atomic.Uint64
	reminderFailures    atomic.Uint64
}

func NewMetrics() *Metrics {
	return &Metrics{startedAt: time.Now().UTC()}
}

func (m *Metrics) IncInteractions() {
	if m == nil {
		return
	}
	m.interactionsTotal.Add(1)
}

func (m *Metrics) IncInteractionFailures() {
	if m == nil {
		return
	}
	m.interactionFailures.Add(1)
}

func (m *Metrics) IncPluginFailures() {
	if m == nil {
		return
	}
	m.pluginFailures.Add(1)
}

func (m *Metrics) IncAutomationFailures() {
	if m == nil {
		return
	}
	m.automationFailures.Add(1)
}

func (m *Metrics) IncReminderFailures() {
	if m == nil {
		return
	}
	m.reminderFailures.Add(1)
}

func (m *Metrics) FillSnapshot(s *Snapshot) {
	if m == nil || s == nil {
		return
	}
	if s.Runtime.StartedAt.IsZero() {
		s.Runtime.StartedAt = m.startedAt
	}
	s.Activity.InteractionsTotal = m.interactionsTotal.Load()
	s.Activity.InteractionFailures = m.interactionFailures.Load()
	s.Activity.PluginFailures = m.pluginFailures.Load()
	s.Activity.AutomationFailures = m.automationFailures.Load()
	s.Activity.ReminderFailures = m.reminderFailures.Load()
}

func RenderPrometheus(s Snapshot, now time.Time) string {
	var b strings.Builder
	writeMetric := func(help, typ, name string, value any) {
		b.WriteString("# HELP ")
		b.WriteString(name)
		b.WriteByte(' ')
		b.WriteString(help)
		b.WriteByte('\n')
		b.WriteString("# TYPE ")
		b.WriteString(name)
		b.WriteByte(' ')
		b.WriteString(typ)
		b.WriteByte('\n')
		fmt.Fprintf(&b, "%s %v\n", name, value)
	}

	startedAt := s.Runtime.StartedAt.UTC()
	if startedAt.IsZero() {
		startedAt = now.UTC()
	}
	uptime := now.UTC().Sub(startedAt)
	if uptime < 0 {
		uptime = 0
	}

	ready := 0
	if s.Runtime.Ready {
		ready = 1
	}
	prodMode := 0
	if s.Runtime.ProdMode {
		prodMode = 1
	}

	writeMetric("Whether the application is ready to serve traffic.", "gauge", "mamacord_ready", ready)
	writeMetric("Whether the application is running in production trust mode.", "gauge", "mamacord_prod_mode", prodMode)
	writeMetric("Process uptime in seconds.", "gauge", "mamacord_uptime_seconds", uptime.Seconds())
	writeMetric("Current database migration version.", "gauge", "mamacord_migration_version", s.Runtime.MigrationVersion)
	writeMetric("Current module count.", "gauge", "mamacord_modules", s.Modules.Total)
	writeMetric("Current enabled module count.", "gauge", "mamacord_enabled_modules", s.Modules.Enabled)
	writeMetric("Current plugin count.", "gauge", "mamacord_plugins", s.Plugins.Total)
	writeMetric("Current enabled plugin count.", "gauge", "mamacord_enabled_plugins", s.Plugins.Enabled)
	writeMetric("Current built-in command count.", "gauge", "mamacord_builtin_commands", s.Commands.Builtin)
	writeMetric("Current slash command count.", "gauge", "mamacord_slash_commands", s.Commands.Slash)
	writeMetric("Current user command count.", "gauge", "mamacord_user_commands", s.Commands.User)
	writeMetric("Current message command count.", "gauge", "mamacord_message_commands", s.Commands.Message)
	writeMetric("Total Discord interaction entries seen by the runtime.", "counter", "mamacord_interactions_total", s.Activity.InteractionsTotal)
	writeMetric("Total Discord interaction failures.", "counter", "mamacord_interaction_failures_total", s.Activity.InteractionFailures)
	writeMetric("Total plugin execution failures.", "counter", "mamacord_plugin_failures_total", s.Activity.PluginFailures)
	writeMetric("Total plugin automation failures.", "counter", "mamacord_plugin_automation_failures_total", s.Activity.AutomationFailures)
	writeMetric("Total reminder scheduler failures.", "counter", "mamacord_reminder_failures_total", s.Activity.ReminderFailures)
	return b.String()
}
