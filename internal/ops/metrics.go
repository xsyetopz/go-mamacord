package ops

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"
)

type Snapshot struct {
	Ready            bool
	StartedAt        time.Time
	MigrationVersion int
	ProdMode         bool
	// DiscordStartError is a dev-focused diagnostic string when the Discord bot
	// fails to connect (bad token, missing intents, etc). Empty means no error.
	DiscordStartError   string
	ModuleCount         int
	EnabledModuleCount  int
	PluginCount         int
	EnabledPluginCount  int
	BuiltinCommandCount int
	SlashCommandCount   int
	UserCommandCount    int
	MessageCommandCount int

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
	if s.StartedAt.IsZero() {
		s.StartedAt = m.startedAt
	}
	s.InteractionsTotal = m.interactionsTotal.Load()
	s.InteractionFailures = m.interactionFailures.Load()
	s.PluginFailures = m.pluginFailures.Load()
	s.AutomationFailures = m.automationFailures.Load()
	s.ReminderFailures = m.reminderFailures.Load()
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

	startedAt := s.StartedAt.UTC()
	if startedAt.IsZero() {
		startedAt = now.UTC()
	}
	uptime := now.UTC().Sub(startedAt)
	if uptime < 0 {
		uptime = 0
	}

	ready := 0
	if s.Ready {
		ready = 1
	}
	prodMode := 0
	if s.ProdMode {
		prodMode = 1
	}

	writeMetric("Whether the application is ready to serve traffic.", "gauge", "mamacord_ready", ready)
	writeMetric("Whether the application is running in production trust mode.", "gauge", "mamacord_prod_mode", prodMode)
	writeMetric("Process uptime in seconds.", "gauge", "mamacord_uptime_seconds", uptime.Seconds())
	writeMetric("Current SQLite migration version.", "gauge", "mamacord_migration_version", s.MigrationVersion)
	writeMetric("Current module count.", "gauge", "mamacord_modules", s.ModuleCount)
	writeMetric("Current enabled module count.", "gauge", "mamacord_enabled_modules", s.EnabledModuleCount)
	writeMetric("Current plugin count.", "gauge", "mamacord_plugins", s.PluginCount)
	writeMetric("Current enabled plugin count.", "gauge", "mamacord_enabled_plugins", s.EnabledPluginCount)
	writeMetric("Current built-in command count.", "gauge", "mamacord_builtin_commands", s.BuiltinCommandCount)
	writeMetric("Current slash command count.", "gauge", "mamacord_slash_commands", s.SlashCommandCount)
	writeMetric("Current user command count.", "gauge", "mamacord_user_commands", s.UserCommandCount)
	writeMetric("Current message command count.", "gauge", "mamacord_message_commands", s.MessageCommandCount)
	writeMetric("Total Discord interaction entries seen by the runtime.", "counter", "mamacord_interactions_total", s.InteractionsTotal)
	writeMetric("Total Discord interaction failures.", "counter", "mamacord_interaction_failures_total", s.InteractionFailures)
	writeMetric("Total plugin execution failures.", "counter", "mamacord_plugin_failures_total", s.PluginFailures)
	writeMetric("Total plugin automation failures.", "counter", "mamacord_plugin_automation_failures_total", s.AutomationFailures)
	writeMetric("Total reminder scheduler failures.", "counter", "mamacord_reminder_failures_total", s.ReminderFailures)
	return b.String()
}
