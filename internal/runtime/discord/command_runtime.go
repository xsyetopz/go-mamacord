package discordruntime

import (
	"context"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/appcmd"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/router"
)

func (b *Bot) commandRegistrar() appcmd.Registrar {
	return appcmd.Registrar{
		Client:           b.client,
		Builtins:         b.order,
		PluginHost:       b.pluginHost,
		EnabledPluginIDs: b.enabledPluginIDsForHost(b.pluginHost),
		I18n:             b.i18n,
	}
}

func (b *Bot) commandDispatcher() appcmd.Dispatcher {
	return appcmd.Dispatcher{
		Logger:                b.logger,
		I18n:                  b.i18n,
		ProdMode:              b.prodMode,
		Commands:              b.commands,
		PluginCommands:        b.pluginCommands,
		PluginUserCommands:    b.pluginUserCommands,
		PluginMessageCommands: b.pluginMessageCommands,
		Services:              b.services,
		CheckRestrictions:     b.checkRestrictions,
		TakeSlashCooldown:     b.takeSlashCooldown,
		GuildCommandEnabled:   b.guildCommandEnabled,
		IncInteraction:        b.incInteraction,
		IncInteractionFailure: b.incInteractionFailure,
		IncPluginFailure:      b.incPluginFailure,
	}
}

func (b *Bot) onCommand(e *events.ApplicationCommandInteractionCreate) {
	b.commandDispatcher().OnCommand(e)
}

func (b *Bot) onAutocomplete(e *events.AutocompleteInteractionCreate) {
	b.commandDispatcher().OnAutocomplete(e)
}

func (b *Bot) commandCreates(_ []string) []discord.ApplicationCommandCreate {
	return b.commandRegistrar().Creates()
}

func (b *Bot) onGuildsReady(e *events.GuildsReady) {
	if b == nil || e == nil {
		return
	}
	if b.devGuildID != nil {
		return
	}
	if !b.commandRegisterAllGuilds {
		return
	}

	ctx := context.Background()
	if err := b.commandRegistrar().RegisterInCachedGuilds(ctx); err != nil {
		b.logger.Error("register commands in cached guilds failed", "err", err.Error())
	}
}

func (b *Bot) takeSlashCooldown(
	e *events.ApplicationCommandInteractionCreate,
	cmdName string,
	now time.Time,
) (int, bool) {
	key := router.SlashCooldownKey(e, cmdName)
	if d := b.commandCooldown(key); d > 0 {
		if remaining, ok := b.cooldowns.Take(uint64(e.User().ID), key, d, now); !ok {
			return cooldownSecs(remaining), false
		}
	}
	return 0, true
}
