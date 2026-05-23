package discordruntime

import (
	"github.com/disgoorg/disgo/events"

	discordgateway "github.com/xsyetopz/go-mamacord/internal/runtime/discord/gateway"
)

func (b *Bot) onGuildJoin(e *events.GuildJoin) {
	b.gatewayHandlers().OnGuildJoin(e)
}

func (b *Bot) onGuildLeave(e *events.GuildLeave) {
	b.gatewayHandlers().OnGuildLeave(e)
}

func (b *Bot) onGuildUpdate(e *events.GuildUpdate) {
	b.gatewayHandlers().OnGuildUpdate(e)
}

func (b *Bot) onGuildMemberJoin(e *events.GuildMemberJoin) {
	b.gatewayHandlers().OnGuildMemberJoin(e)
}

func (b *Bot) onGuildMemberLeave(e *events.GuildMemberLeave) {
	b.gatewayHandlers().OnGuildMemberLeave(e)
}

func (b *Bot) onGuildBan(e *events.GuildBan) {
	b.gatewayHandlers().OnGuildBan(e)
}

func (b *Bot) onGuildUnban(e *events.GuildUnban) {
	b.gatewayHandlers().OnGuildUnban(e)
}

func (b *Bot) onGuildChannelCreate(e *events.GuildChannelCreate) {
	b.gatewayHandlers().OnGuildChannelCreate(e)
}

func (b *Bot) onGuildChannelDelete(e *events.GuildChannelDelete) {
	b.gatewayHandlers().OnGuildChannelDelete(e)
}

func (b *Bot) onRoleCreate(e *events.RoleCreate) {
	b.gatewayHandlers().OnRoleCreate(e)
}

func (b *Bot) onRoleDelete(e *events.RoleDelete) {
	b.gatewayHandlers().OnRoleDelete(e)
}

func (b *Bot) onInviteCreate(e *events.InviteCreate) {
	b.gatewayHandlers().OnInviteCreate(e)
}

func (b *Bot) onInviteDelete(e *events.InviteDelete) {
	b.gatewayHandlers().OnInviteDelete(e)
}

func (b *Bot) gatewayHandlers() discordgateway.Handlers {
	return discordgateway.Handlers{
		Logger:                   b.logger,
		Store:                    b.store,
		I18n:                     b.i18n,
		Client:                   b.client,
		CommandRegisterAllGuilds: b.commandRegisterAllGuilds,
		DevGuildID:               b.devGuildID,
		CommandCreates:           b.commandCreates,
		PluginEvents:             b.pluginAuto,
	}
}
