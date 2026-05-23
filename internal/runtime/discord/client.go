package discordruntime

import (
	"strings"

	"github.com/disgoorg/disgo"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/gateway"

	discordplugin "github.com/xsyetopz/go-mamacord/internal/runtime/discord/plugin"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

const (
	commandRegistrationModeGlobal = "global"
	commandRegistrationModeGuilds = "guilds"
	commandRegistrationModeHybrid = "hybrid"
)

func requestedGatewayIntents() []gateway.Intents {
	// Keep this list in one place so we can:
	// 1) configure disgo gateway intents, and
	// 2) print accurate, actionable diagnostics when Discord rejects intents (4014).
	return []gateway.Intents{
		gateway.IntentGuilds,
		gateway.IntentGuildMembers, // privileged
		gateway.IntentGuildModeration,
		gateway.IntentGuildInvites,
		gateway.IntentDirectMessages,
	}
}

func requestedGatewayIntentsMask() gateway.Intents {
	mask := gateway.IntentsNone
	for _, intent := range requestedGatewayIntents() {
		mask |= intent
	}
	return mask
}

func (b *Bot) initPlugins(deps Dependencies) error {
	dirs := make([]string, 0, 2)
	if dir := strings.TrimSpace(deps.BundledPluginsDir); dir != "" {
		dirs = append(dirs, dir)
	}
	if dir := strings.TrimSpace(deps.UserPluginsDir); dir != "" {
		dirs = append(dirs, dir)
	}
	if len(dirs) > 0 {
		host, err := pluginhost.NewHost(pluginhost.Options{
			Dirs:                dirs,
			ProdMode:            deps.ProdMode,
			AllowUnsignedPlugin: deps.AllowUnsignedPlugins,
			TrustedKeysFile:     deps.TrustedKeysFile,
			PermissionsFile:     deps.PermissionsFile,
			Store:               deps.Store,
			Discord: discordplugin.Executor{
				ClientProvider:      func() *bot.Client { return b.client },
				EnsureDMChannelFunc: b.ensureDMChannel,
			},
			Logger: b.logger,
			I18n:   &b.i18n,
		})
		if err != nil {
			return err
		}
		b.pluginHost = host
	}

	return nil
}

func (b *Bot) newClient(token string) (*bot.Client, error) {
	return disgo.New(token,
		bot.WithLogger(b.logger),
		bot.WithGatewayConfigOpts(gateway.WithIntents(
			requestedGatewayIntents()...,
		)),
		bot.WithEventListenerFunc(b.onCommand),
		bot.WithEventListenerFunc(b.onAutocomplete),
		bot.WithEventListenerFunc(b.onComponent),
		bot.WithEventListenerFunc(b.onModal),
		bot.WithEventListenerFunc(b.onGuildJoin),
		bot.WithEventListenerFunc(b.onGuildLeave),
		bot.WithEventListenerFunc(b.onGuildUpdate),
		bot.WithEventListenerFunc(b.onGuildMemberJoin),
		bot.WithEventListenerFunc(b.onGuildMemberLeave),
		bot.WithEventListenerFunc(b.onGuildBan),
		bot.WithEventListenerFunc(b.onGuildUnban),
		bot.WithEventListenerFunc(b.onGuildChannelCreate),
		bot.WithEventListenerFunc(b.onGuildChannelDelete),
		bot.WithEventListenerFunc(b.onRoleCreate),
		bot.WithEventListenerFunc(b.onRoleDelete),
		bot.WithEventListenerFunc(b.onInviteCreate),
		bot.WithEventListenerFunc(b.onInviteDelete),
		bot.WithEventListenerFunc(b.onGuildsReady),
	)
}
