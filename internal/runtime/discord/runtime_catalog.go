package discordruntime

import (
	"context"
	"log/slog"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/commands"
	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/catalog"
	discordpluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

func (b *Bot) refreshRuntimeCatalog(ctx context.Context) error {
	states, err := b.loadModuleStates(ctx)
	if err != nil {
		return err
	}

	modules := map[string]moduleapi.Info{}
	builtinCommands := map[string]slashcmd.Command{}
	order := []slashcmd.Command{}
	pluginCommands := map[string]discordpluginbridge.Route{}
	pluginUserCommands := map[string]discordpluginbridge.Route{}
	pluginMessageCommands := map[string]discordpluginbridge.Route{}
	pluginRoutes := map[string]discordpluginbridge.Route{}

	for _, desc := range commands.Catalog() {
		cmds := catalog.BuiltinCommands(desc)
		defaultEnabled := catalog.BuiltinDefaultEnabled(desc, b.moduleSeed)
		enabled := catalog.ResolveBuiltinModuleEnabled(desc, b.moduleSeed, states)
		info := moduleapi.Info{
			ID:             desc.ID,
			Name:           desc.Name,
			Kind:           moduleapi.KindCoreBuiltin,
			Runtime:        moduleapi.RuntimeGo,
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
	modules map[string]moduleapi.Info,
	pluginRoutes map[string]discordpluginbridge.Route,
	pluginCommands map[string]discordpluginbridge.Route,
	pluginUserCommands map[string]discordpluginbridge.Route,
	pluginMessageCommands map[string]discordpluginbridge.Route,
	builtinCommands map[string]slashcmd.Command,
	host *pluginhost.Host,
	states map[string]store.ModuleState,
) {
	if host == nil {
		return
	}

	for _, info := range host.Infos() {
		pluginRoutes[info.ID] = discordpluginbridge.Route{Host: host, PluginID: info.ID}
		kind := catalog.ModuleKindForPlugin(info.ID)

		defaultEnabled := catalog.PluginDefaultEnabled(info.ID, b.moduleSeed)
		enabled := defaultEnabled
		if state, ok := states[info.ID]; ok {
			enabled = state.Enabled
		}

		moduleInfo := moduleapi.Info{
			ID:             info.ID,
			Name:           strings.TrimSpace(info.Name),
			Kind:           kind,
			Runtime:        moduleapi.RuntimeLua,
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
				pluginUserCommands[name] = discordpluginbridge.Route{Host: host, PluginID: info.ID}
				continue
			case pluginhost.CommandTypeMessage:
				if _, exists := pluginMessageCommands[name]; exists {
					b.logger.WarnContext(ctx, "duplicate plugin message command, skipping", slog.String("command", name), slog.String("module", info.ID))
					continue
				}
				pluginMessageCommands[name] = discordpluginbridge.Route{Host: host, PluginID: info.ID}
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
			pluginCommands[name] = discordpluginbridge.Route{Host: host, PluginID: info.ID}
		}
	}
}
