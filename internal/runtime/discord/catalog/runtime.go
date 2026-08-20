package catalog

import (
	"context"
	"errors"
	"fmt"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/catalog/builtin"
	"log/slog"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/commands"
	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/guildconfig"
	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	discordpluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/host"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/projection"
	pluginstore "github.com/xsyetopz/go-mamacord/internal/storage/plugins"
)

type Snapshot struct {
	Modules               map[string]moduleapi.Info
	Commands              map[string]slashcmd.Command
	Order                 []slashcmd.Command
	PluginCommands        map[string]discordpluginbridge.Route
	PluginUserCommands    map[string]discordpluginbridge.Route
	PluginMessageCommands map[string]discordpluginbridge.Route
	PluginRoutes          map[string]discordpluginbridge.Route
}

type Runtime struct {
	logger       *slog.Logger
	moduleSeed   config.ModulesFile
	moduleStates pluginstore.ModuleStateStore
	pluginKV     pluginstore.PluginKVStore
	pluginHost   *pluginhost.Host
	snapshot     atomic.Pointer[Snapshot]
}

func NewRuntime(logger *slog.Logger, seed config.ModulesFile, states pluginstore.ModuleStateStore, kv pluginstore.PluginKVStore, host *pluginhost.Host) *Runtime {
	return &Runtime{logger: logger, moduleSeed: seed, moduleStates: states, pluginKV: kv, pluginHost: host}
}

func (runtime *Runtime) Snapshot() *Snapshot {
	if runtime != nil {
		if snapshot := runtime.snapshot.Load(); snapshot != nil {
			return snapshot
		}
	}
	return emptySnapshot()
}

func emptySnapshot() *Snapshot {
	return &Snapshot{
		Modules: map[string]moduleapi.Info{}, Commands: map[string]slashcmd.Command{},
		PluginCommands: map[string]discordpluginbridge.Route{}, PluginUserCommands: map[string]discordpluginbridge.Route{},
		PluginMessageCommands: map[string]discordpluginbridge.Route{}, PluginRoutes: map[string]discordpluginbridge.Route{},
	}
}

func (runtime *Runtime) Refresh(ctx context.Context) (Stats, error) {
	states, err := runtime.loadModuleStates(ctx)
	if err != nil {
		return Stats{}, err
	}
	modules := map[string]moduleapi.Info{}
	builtinCommands := map[string]slashcmd.Command{}
	order := []slashcmd.Command{}
	pluginCommands := map[string]discordpluginbridge.Route{}
	pluginUserCommands := map[string]discordpluginbridge.Route{}
	pluginMessageCommands := map[string]discordpluginbridge.Route{}
	pluginRoutes := map[string]discordpluginbridge.Route{}

	for _, descriptor := range commands.Catalog() {
		commands := builtin.Commands(descriptor)
		defaultEnabled := BuiltinDefaultEnabled(descriptor, runtime.moduleSeed)
		enabled := ResolveBuiltinModuleEnabled(descriptor, runtime.moduleSeed, states)
		info := moduleapi.Info{
			ID: descriptor.ID, Name: descriptor.Name, Kind: moduleapi.KindCoreBuiltin, Runtime: moduleapi.RuntimeGo,
			Enabled: enabled, DefaultEnabled: defaultEnabled, Toggleable: descriptor.Toggleable,
			Source: ModuleSourceBuiltin, Commands: SlashCommandNames(commands),
		}
		modules[info.ID] = info
		if !enabled {
			continue
		}
		for _, command := range commands {
			name := strings.TrimSpace(command.Name)
			if name == "" {
				continue
			}
			if _, exists := builtinCommands[name]; exists {
				runtime.warn(ctx, "duplicate builtin command, skipping", name, descriptor.ID)
				continue
			}
			order = append(order, command)
			builtinCommands[name] = command
		}
	}

	runtime.appendPluginModules(ctx, modules, pluginRoutes, pluginCommands, pluginUserCommands, pluginMessageCommands, builtinCommands, states)
	snapshot := &Snapshot{
		Modules: modules, Commands: builtinCommands, Order: order, PluginCommands: pluginCommands,
		PluginUserCommands: pluginUserCommands, PluginMessageCommands: pluginMessageCommands, PluginRoutes: pluginRoutes,
	}
	runtime.snapshot.Store(snapshot)
	return RuntimeStats(modules, order, len(pluginCommands), len(pluginUserCommands), len(pluginMessageCommands)), nil
}

func (runtime *Runtime) appendPluginModules(ctx context.Context, modules map[string]moduleapi.Info, pluginRoutes, pluginCommands, pluginUserCommands, pluginMessageCommands map[string]discordpluginbridge.Route, builtinCommands map[string]slashcmd.Command, states map[string]pluginstore.ModuleState) {
	host := runtime.pluginHost
	if host == nil {
		return
	}
	for _, info := range host.Infos() {
		pluginRoutes[info.ID] = discordpluginbridge.Route{Host: host, PluginID: info.ID}
		defaultEnabled := PluginDefaultEnabled(info.ID, runtime.moduleSeed)
		enabled := defaultEnabled
		if state, ok := states[info.ID]; ok {
			enabled = state.Enabled
		}
		module := moduleapi.Info{
			ID: info.ID, Name: strings.TrimSpace(info.Name), Kind: ModuleKindForPlugin(info.ID), Runtime: moduleapi.RuntimeStarlark,
			Enabled: enabled, DefaultEnabled: defaultEnabled, Toggleable: true, Signed: info.Signed,
			Source: ModuleSourcePlugin, Commands: PluginCommandNames(info.Commands),
		}
		if module.Name == "" {
			module.Name = info.ID
		}
		modules[info.ID] = module
		if !enabled {
			continue
		}
		for _, command := range info.Commands {
			name := strings.TrimSpace(command.Name)
			if name == "" {
				continue
			}
			route := discordpluginbridge.Route{Host: host, PluginID: info.ID}
			switch projection.NormalizeCommandType(command.Type) {
			case projection.CommandTypeUser:
				if _, exists := pluginUserCommands[name]; exists {
					runtime.warn(ctx, "duplicate plugin user command, skipping", name, info.ID)
					continue
				}
				pluginUserCommands[name] = route
			case projection.CommandTypeMessage:
				if _, exists := pluginMessageCommands[name]; exists {
					runtime.warn(ctx, "duplicate plugin message command, skipping", name, info.ID)
					continue
				}
				pluginMessageCommands[name] = route
			default:
				if _, exists := builtinCommands[name]; exists {
					runtime.warn(ctx, "plugin command conflicts with builtin command, skipping", name, info.ID)
					continue
				}
				if _, exists := pluginCommands[name]; exists {
					runtime.warn(ctx, "duplicate plugin command, skipping", name, info.ID)
					continue
				}
				pluginCommands[name] = route
			}
		}
	}
}

func (runtime *Runtime) warn(ctx context.Context, message, command, module string) {
	if runtime.logger != nil {
		runtime.logger.WarnContext(ctx, message, slog.String("command", command), slog.String("module", module))
	}
}

func (runtime *Runtime) loadModuleStates(ctx context.Context) (map[string]pluginstore.ModuleState, error) {
	if runtime == nil || runtime.moduleStates == nil {
		return nil, errors.New("store not configured")
	}
	states, err := runtime.moduleStates.ListModuleStates(ctx)
	if err != nil {
		return nil, err
	}
	out := make(map[string]pluginstore.ModuleState, len(states))
	for _, state := range states {
		if id := strings.TrimSpace(state.ModuleID); id != "" {
			out[id] = state
		}
	}
	return out, nil
}

func (runtime *Runtime) ModuleInfos() []moduleapi.Info {
	modules := runtime.Snapshot().Modules
	out := make([]moduleapi.Info, 0, len(modules))
	for _, info := range modules {
		info.Commands = append([]string(nil), info.Commands...)
		out = append(out, info)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (runtime *Runtime) ModuleInfo(moduleID string) (moduleapi.Info, bool) {
	info, ok := runtime.Snapshot().Modules[strings.TrimSpace(moduleID)]
	return info, ok
}

func (runtime *Runtime) ModuleEnabled(moduleID string) bool {
	info, ok := runtime.ModuleInfo(moduleID)
	return ok && info.Enabled
}

func (runtime *Runtime) SetModuleEnabled(ctx context.Context, moduleID string, enabled bool, actorID uint64) error {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return errors.New("module id is required")
	}
	info, ok := runtime.ModuleInfo(moduleID)
	if !ok {
		return fmt.Errorf("unknown module %q", moduleID)
	}
	if !info.Toggleable {
		return fmt.Errorf("module %q is required and cannot be disabled", moduleID)
	}
	state := pluginstore.ModuleState{ModuleID: moduleID, Enabled: enabled, UpdatedAt: time.Now().UTC()}
	if actorID != 0 {
		state.UpdatedBy = &actorID
	}
	return runtime.moduleStates.PutModuleState(ctx, state)
}

func (runtime *Runtime) ResetModule(ctx context.Context, moduleID string) error {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return errors.New("module id is required")
	}
	info, ok := runtime.ModuleInfo(moduleID)
	if !ok {
		return fmt.Errorf("unknown module %q", moduleID)
	}
	if !info.Toggleable {
		return fmt.Errorf("module %q is required and cannot be reset", moduleID)
	}
	return runtime.moduleStates.DeleteModuleState(ctx, moduleID)
}

func (runtime *Runtime) GuildCommandEnabled(ctx context.Context, guildID uint64, pluginID, commandName string) (bool, error) {
	if guildID == 0 {
		return true, nil
	}
	if !runtime.ModuleEnabled(pluginID) {
		return false, nil
	}
	return guildconfig.CommandEnabled(ctx, runtime.pluginKV, guildID, pluginID, commandName)
}

func (runtime *Runtime) GuildPluginEnabled(ctx context.Context, guildID uint64, pluginID string) (bool, error) {
	if guildID == 0 {
		return true, nil
	}
	if !runtime.ModuleEnabled(pluginID) {
		return false, nil
	}
	return guildconfig.PluginEnabled(ctx, runtime.pluginKV, guildID, pluginID)
}

func (runtime *Runtime) PluginRoute(pluginID string) (discordpluginbridge.Route, bool) {
	route, ok := runtime.Snapshot().PluginRoutes[strings.TrimSpace(pluginID)]
	return route, ok
}

func (runtime *Runtime) EnabledPluginJobs() []pluginhost.PluginJob {
	out := []pluginhost.PluginJob{}
	if runtime.pluginHost != nil {
		for _, job := range runtime.pluginHost.Jobs() {
			if runtime.ModuleEnabled(job.PluginID) {
				out = append(out, job)
			}
		}
	}
	return out
}

func (runtime *Runtime) EnabledPluginEventSubscribers(eventName string) []discordpluginbridge.Route {
	out := []discordpluginbridge.Route{}
	if runtime.pluginHost != nil {
		for _, pluginID := range runtime.pluginHost.EventSubscribers(eventName) {
			if runtime.ModuleEnabled(pluginID) {
				out = append(out, discordpluginbridge.Route{Host: runtime.pluginHost, PluginID: pluginID})
			}
		}
	}
	return out
}

func (runtime *Runtime) EnabledPluginIDs(host *pluginhost.Host) map[string]struct{} {
	if host == nil {
		return nil
	}
	out := map[string]struct{}{}
	for moduleID, route := range runtime.Snapshot().PluginRoutes {
		if route.Host == host && runtime.ModuleEnabled(moduleID) {
			out[moduleID] = struct{}{}
		}
	}
	return out
}
