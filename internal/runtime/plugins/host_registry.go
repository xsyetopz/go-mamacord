package pluginhost

import (
	"context"
	"log/slog"
	"maps"
	"sort"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/permissions"
)

func addCommands(
	ctx context.Context,
	logger *slog.Logger,
	nextCommands map[string]PluginCommand,
	pluginID string,
	cmds []PluginCommand,
) {
	for _, cmd := range cmds {
		if cmd.Command.Name == "" {
			continue
		}
		key := commandLookupKey(cmd.Command.Type, cmd.Command.Name)
		if _, exists := nextCommands[key]; exists {
			logger.WarnContext(
				ctx,
				"duplicate command name, skipping",
				slog.String("command", cmd.Command.Name),
				slog.String("type", NormalizeCommandType(cmd.Command.Type)),
				slog.String("plugin", pluginID),
			)
			continue
		}
		nextCommands[key] = cmd
	}
}

func commandLookupKey(kind, name string) string {
	return NormalizeCommandType(kind) + ":" + strings.ToLower(strings.TrimSpace(name))
}

func (m *Host) swapState(
	nextPlugins map[string]*Plugin,
	nextCommands map[string]PluginCommand,
	nextEvents map[string][]string,
	nextJobs []PluginJob,
	policy permissions.Policy,
) map[string]*Plugin {
	m.mu.Lock()
	oldPlugins := m.plugins
	m.plugins = nextPlugins
	m.commands = nextCommands
	m.eventSubs = nextEvents
	m.jobs = nextJobs
	m.policy = policy
	m.mu.Unlock()
	return oldPlugins
}

func closePlugins(oldPlugins map[string]*Plugin) {
	for _, pl := range oldPlugins {
		if pl != nil && pl.VM != nil {
			pl.VM.Close()
		}
	}
}

func buildSubscriptions(pls map[string]*Plugin) (map[string][]string, []PluginJob) {
	ev := map[string][]string{}
	var jobs []PluginJob

	for _, pl := range pls {
		if pl == nil {
			continue
		}

		for _, raw := range pl.Events {
			name := strings.ToLower(strings.TrimSpace(raw))
			if name == "" {
				continue
			}
			ev[name] = append(ev[name], pl.ID)
		}

		for _, job := range pl.Jobs {
			id := strings.TrimSpace(job.ID)
			spec := strings.TrimSpace(job.Schedule)
			if id == "" || spec == "" {
				continue
			}
			jobs = append(jobs, PluginJob{
				PluginID: pl.ID,
				JobID:    id,
				Schedule: spec,
			})
		}
	}

	for name := range ev {
		sort.Strings(ev[name])
	}
	sort.Slice(jobs, func(i, j int) bool {
		if jobs[i].PluginID != jobs[j].PluginID {
			return jobs[i].PluginID < jobs[j].PluginID
		}
		return jobs[i].JobID < jobs[j].JobID
	})

	return ev, jobs
}
func (m *Host) Commands() map[string]PluginCommand {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make(map[string]PluginCommand, len(m.commands))
	maps.Copy(out, m.commands)
	return out
}

func (m *Host) Jobs() []PluginJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return append([]PluginJob(nil), m.jobs...)
}

func (m *Host) EventSubscribers(eventName string) []string {
	eventName = strings.ToLower(strings.TrimSpace(eventName))
	if eventName == "" {
		return nil
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	return append([]string(nil), m.eventSubs[eventName]...)
}

func (m *Host) EffectivePermissions(pluginID string) (permissions.Permissions, bool) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return permissions.Permissions{}, false
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	pl, ok := m.plugins[pluginID]
	if !ok || pl == nil {
		return permissions.Permissions{}, false
	}
	return pl.Effective, true
}

type PluginInfo struct {
	ID        string
	Name      string
	Version   string
	Dir       string
	Signed    bool
	Bundled   bool
	Effective permissions.Permissions
	Commands  []Command
}

func (m *Host) Infos() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]PluginInfo, 0, len(m.plugins))
	for _, pl := range m.plugins {
		if pl == nil {
			continue
		}
		out = append(out, PluginInfo{
			ID:        pl.ID,
			Name:      pl.Manifest.Name,
			Version:   pl.Manifest.Version,
			Dir:       pl.Dir,
			Signed:    pl.Signature != nil,
			Bundled:   pl.Bundled,
			Effective: pl.Effective,
			Commands:  append([]Command(nil), pl.Commands...),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
