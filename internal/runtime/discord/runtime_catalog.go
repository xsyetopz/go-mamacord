package discordruntime

import (
	"context"
	"log/slog"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/commands"
	commandapi "github.com/xsyetopz/go-mamacord/internal/commands/api"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/catalog"
	discordplugin "github.com/xsyetopz/go-mamacord/internal/runtime/discord/plugin"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

func (b *Bot) refreshRuntimeCatalog(ctx context.Context) error {
	states, err := b.loadModuleStates(ctx)
	if err != nil {
		return err
	}

	modules := map[string]commandapi.ModuleInfo{}
	builtinCommands := map[string]commandapi.SlashCommand{}
	order := []commandapi.SlashCommand{}
	pluginCommands := map[string]discordplugin.Route{}
	pluginUserCommands := map[string]discordplugin.Route{}
	pluginMessageCommands := map[string]discordplugin.Route{}
	pluginRoutes := map[string]discordplugin.Route{}

	for _, desc := range commands.Catalog() {
		cmds := desc.Commands()
		defaultEnabled := catalog.BuiltinDefaultEnabled(desc, b.moduleSeed)
		enabled := catalog.ResolveBuiltinModuleEnabled(desc, b.moduleSeed, states)
		info := commandapi.ModuleInfo{
			ID:             desc.ID,
			Name:           desc.Name,
			Kind:           commandapi.ModuleKindCoreBuiltin,
			Runtime:        commandapi.ModuleRuntimeGo,
			Enabled:        enabled,
			DefaultEnabled: defaultEnabled,
			Toggleable:     desc.Toggleable,
			Source:         catalog.ModuleSourceBuiltin,
			Commands:       catalog.SlashCommandNames(cmds),
		}
		modules[info.ID] = info
		if !enabled {
			continue
		}
		for _, cmd := range cmds {
			name := strings.TrimSpace(cmd.Name)
			if name == "" {
				continue
			}
			if _, exists := builtinCommands[name]; exists {
				b.logger.WarnContext(ctx, "duplicate builtin command, skipping", slog.String("command", name), slog.String("module", desc.ID))
				continue
			}
			order = append(order, cmd)
			builtinCommands[name] = cmd
		}
	}

	b.appendPluginModules(
		ctx,
		modules,
		pluginRoutes,
		pluginCommands,
		pluginUserCommands,
		pluginMessageCommands,
		builtinCommands,
		b.pluginHost,
		states,
	)

	b.modules = modules
	b.commands = builtinCommands
	b.order = order
	b.pluginCommands = pluginCommands
	b.pluginUserCommands = pluginUserCommands
	b.pluginMessageCommands = pluginMessageCommands
	b.pluginRoutes = pluginRoutes
	b.stats.Store(catalog.RuntimeStats(modules, order, len(pluginCommands), len(pluginUserCommands), len(pluginMessageCommands)))
	return nil
}

func (b *Bot) appendPluginModules(
	ctx context.Context,
	modules map[string]commandapi.ModuleInfo,
	pluginRoutes map[string]discordplugin.Route,
	pluginCommands map[string]discordplugin.Route,
	pluginUserCommands map[string]discordplugin.Route,
	pluginMessageCommands map[string]discordplugin.Route,
	builtinCommands map[string]commandapi.SlashCommand,
	host *pluginhost.Host,
	states map[string]store.ModuleState,
) {
	if host == nil {
		return
	}

	for _, info := range host.Infos() {
		pluginRoutes[info.ID] = discordplugin.Route{Host: host, PluginID: info.ID}
		kind := catalog.ModuleKindForPlugin(info.ID)

		defaultEnabled := catalog.PluginDefaultEnabled(info.ID, b.moduleSeed)
		enabled := defaultEnabled
		if state, ok := states[info.ID]; ok {
			enabled = state.Enabled
		}

		moduleInfo := commandapi.ModuleInfo{
			ID:             info.ID,
			Name:           strings.TrimSpace(info.Name),
			Kind:           kind,
			Runtime:        commandapi.ModuleRuntimeLua,
			Enabled:        enabled,
			DefaultEnabled: defaultEnabled,
			Toggleable:     true,
			Signed:         info.Signed,
			Source:         catalog.ModuleSourcePlugin,
			Commands:       catalog.PluginCommandNames(info.Commands),
		}
		if moduleInfo.Name == "" {
			moduleInfo.Name = info.ID
		}
		modules[info.ID] = moduleInfo
		if !enabled {
			continue
		}

		for _, cmd := range info.Commands {
			name := strings.TrimSpace(cmd.Name)
			if name == "" {
				continue
			}
			switch pluginhost.NormalizeCommandType(cmd.Type) {
			case pluginhost.CommandTypeUser:
				if _, exists := pluginUserCommands[name]; exists {
					b.logger.WarnContext(ctx, "duplicate plugin user command, skipping", slog.String("command", name), slog.String("module", info.ID))
					continue
				}
				pluginUserCommands[name] = discordplugin.Route{Host: host, PluginID: info.ID}
				continue
			case pluginhost.CommandTypeMessage:
				if _, exists := pluginMessageCommands[name]; exists {
					b.logger.WarnContext(ctx, "duplicate plugin message command, skipping", slog.String("command", name), slog.String("module", info.ID))
					continue
				}
				pluginMessageCommands[name] = discordplugin.Route{Host: host, PluginID: info.ID}
				continue
			}
			if _, exists := builtinCommands[name]; exists {
				b.logger.WarnContext(ctx, "plugin command conflicts with builtin command, skipping", slog.String("command", name), slog.String("module", info.ID))
				continue
			}
			if _, exists := pluginCommands[name]; exists {
				b.logger.WarnContext(ctx, "duplicate plugin command, skipping", slog.String("command", name), slog.String("module", info.ID))
				continue
			}
			pluginCommands[name] = discordplugin.Route{Host: host, PluginID: info.ID}
		}
	}
}
