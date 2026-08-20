package discordruntime

import (
	"context"
	discordcatalog "github.com/xsyetopz/go-mamacord/internal/runtime/discord/catalog"
	"sort"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	"github.com/xsyetopz/go-mamacord/internal/buildinfo"
	commandtext "github.com/xsyetopz/go-mamacord/internal/commandtext"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/appcmd"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/router/cooldown"
)

func (b *Bot) commandRegistrar() appcmd.Registrar {
	snapshot := b.catalogSnapshot()
	return appcmd.Registrar{
		Client:           b.client,
		Builtins:         snapshot.Order,
		PluginHost:       b.pluginHost,
		EnabledPluginIDs: b.catalog.EnabledPluginIDs(b.pluginHost),
		I18n:             b.i18n,
	}
}

func (b *Bot) commandDispatcher() appcmd.Dispatcher {
	snapshot := b.catalogSnapshot()
	runtime := b.appCommandRuntime(snapshot)
	return appcmd.Dispatcher{
		DispatcherCore: appcmd.DispatcherCore{
			Logger: b.logger, I18n: b.i18n, ProdMode: b.prodMode, Services: runtime.Services,
		},
		DispatcherCatalog: appcmd.DispatcherCatalog{
			Commands: snapshot.Commands, PluginCommands: snapshot.PluginCommands,
			PluginUserCommands: snapshot.PluginUserCommands, PluginMessageCommands: snapshot.PluginMessageCommands,
		},
		DispatcherPolicy: appcmd.DispatcherPolicy{
			CheckRestrictions: func(ctx context.Context, e *events.ApplicationCommandInteractionCreate, translator commandtext.Translator) (bool, error) {
				return runtime.CheckRestrictions(ctx, e, translator, buildinfo.Current())
			}, TakeSlashCooldown: b.takeSlashCooldown,
			GuildCommandEnabled: b.guildCommandEnabled,
		},
		DispatcherMetrics: appcmd.DispatcherMetrics{
			IncInteraction: b.incInteraction, IncInteractionFailure: b.incInteractionFailure,
			IncPluginFailure: b.incPluginFailure,
		},
	}
}

func (b *Bot) appCommandRuntime(snapshot *discordcatalog.Snapshot) appcmd.Runtime {
	return appcmd.Runtime{
		RuntimeCore: appcmd.RuntimeCore{Logger: b.logger, Registry: b.i18n, Restrictions: b.restrictions, ProdMode: b.prodMode},
		RuntimeCommands: appcmd.RuntimeCommands{
			SlashCommands: snapshot.Commands,
			HelpNames: func(locale string) []string {
				translator := commandtext.Translator{Registry: b.i18n, Locale: locale}
				out := make([]string, 0, len(snapshot.Order)+len(snapshot.PluginCommands))
				for _, command := range snapshot.Order {
					name := strings.TrimSpace(command.Name)
					if strings.TrimSpace(command.NameID) != "" {
						name = translator.S(command.NameID, nil)
					}
					if name != "" {
						out = append(out, name)
					}
				}
				for name := range snapshot.PluginCommands {
					out = append(out, name)
				}
				sort.Strings(out)
				return out
			},
		},
		RuntimeAdmins: appcmd.RuntimeAdmins{IsOwner: b.isOwner, Plugins: b.control.PluginAdmin(), Marketplace: b.marketplace, Modules: b.control.ModuleAdmin()},
		IncFailure:    b.incInteractionFailure,
	}
}

func (b *Bot) onCommand(e *events.ApplicationCommandInteractionCreate) {
	b.interactions.Launch("command", func() { b.commandDispatcher().OnCommand(e) }, func() {
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindWarning, "", "Mamacord is busy. Try again in a moment.", true))
	})
}

func (b *Bot) onAutocomplete(e *events.AutocompleteInteractionCreate) {
	b.interactions.Launch("autocomplete", func() { b.commandDispatcher().OnAutocomplete(e) }, func() { _ = e.AutocompleteResult(nil) })
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
	key := cooldown.SlashCooldownKey(e, cmdName)
	if d := b.cooldownPolicy.CommandCooldown(key); d > 0 {
		if remaining, ok := b.cooldowns.Take(uint64(e.User().ID), key, d, now); !ok {
			return cooldown.CooldownSeconds(remaining), false
		}
	}
	return 0, true
}
