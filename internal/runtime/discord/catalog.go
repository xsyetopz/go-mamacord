package discordruntime

import (
	"context"

	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	discordcatalog "github.com/xsyetopz/go-mamacord/internal/runtime/discord/catalog"
	discordpluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/host"
)

func (b *Bot) catalogSnapshot() *discordcatalog.Snapshot { return b.catalog.Snapshot() }

func (b *Bot) refreshRuntimeCatalog(ctx context.Context) error {
	stats, err := b.catalog.Refresh(ctx)
	if err != nil {
		return err
	}
	b.stats.Store(stats)
	return nil
}

func (b *Bot) moduleInfos() []moduleapi.Info      { return b.catalog.ModuleInfos() }
func (b *Bot) moduleEnabled(moduleID string) bool { return b.catalog.ModuleEnabled(moduleID) }

func (b *Bot) setModuleEnabled(ctx context.Context, moduleID string, enabled bool, actorID uint64) error {
	if err := b.catalog.SetModuleEnabled(ctx, moduleID, enabled, actorID); err != nil {
		return err
	}
	return b.reloadModules(ctx)
}

func (b *Bot) resetModule(ctx context.Context, moduleID string) error {
	if err := b.catalog.ResetModule(ctx, moduleID); err != nil {
		return err
	}
	return b.reloadModules(ctx)
}

func (b *Bot) reloadModules(ctx context.Context) error {
	if b.pluginHost != nil {
		if err := b.pluginHost.LoadAll(ctx); err != nil {
			return err
		}
	}
	if err := b.refreshRuntimeCatalog(ctx); err != nil {
		return err
	}
	if err := syncGatewayCommands(ctx, b.enableGateway, b.commandRegisterAllGuilds, b.devGuildID, b.registerCommands, func(ctx context.Context) error {
		return b.commandRegistrar().RegisterInCachedGuilds(ctx)
	}); err != nil {
		return err
	}
	if b.enableScheduler && b.scheduler != nil && b.ready.Load() {
		b.scheduler.Restart(ctx)
	}
	return nil
}

func syncGatewayCommands(ctx context.Context, enableGateway, registerAllGuilds bool, devGuildID *uint64, registerCommands, registerCachedGuilds func(context.Context) error) error {
	if !enableGateway {
		return nil
	}
	if registerCommands != nil {
		if err := registerCommands(ctx); err != nil {
			return err
		}
	}
	if registerAllGuilds && devGuildID == nil && registerCachedGuilds != nil {
		if err := registerCachedGuilds(ctx); err != nil {
			return err
		}
	}
	return nil
}

func (b *Bot) guildCommandEnabled(ctx context.Context, guildID uint64, pluginID, commandName string) (bool, error) {
	return b.catalog.GuildCommandEnabled(ctx, guildID, pluginID, commandName)
}
func (b *Bot) guildPluginEnabled(ctx context.Context, guildID uint64, pluginID string) (bool, error) {
	return b.catalog.GuildPluginEnabled(ctx, guildID, pluginID)
}
func (b *Bot) pluginRoute(pluginID string) (discordpluginbridge.Route, bool) {
	return b.catalog.PluginRoute(pluginID)
}
func (b *Bot) enabledPluginJobs() []pluginhost.PluginJob { return b.catalog.EnabledPluginJobs() }
func (b *Bot) enabledPluginEventSubscribers(eventName string) []discordpluginbridge.Route {
	return b.catalog.EnabledPluginEventSubscribers(eventName)
}
