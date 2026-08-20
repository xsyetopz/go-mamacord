package app

import (
	"context"
	adminguilds "github.com/xsyetopz/go-mamacord/internal/adminapi/guilds"
	adminservice "github.com/xsyetopz/go-mamacord/internal/adminapi/service"
	"net/http"
	"strings"

	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	discordcontrol "github.com/xsyetopz/go-mamacord/internal/runtime/discord/control"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/host"
)

type adminModuleAdmin struct{ app *App }

func (m adminModuleAdmin) Configured() bool {
	return m.app != nil && m.app.bot != nil
}

func (m adminModuleAdmin) Infos() []moduleapi.Info {
	if m.app == nil || m.app.bot == nil {
		return nil
	}
	return m.app.bot.Control().ModuleAdmin().Infos()
}

func (m adminModuleAdmin) Reload(ctx context.Context) error {
	if m.app == nil || m.app.bot == nil {
		return liveDiscordUnavailable("module reload")
	}
	return m.app.bot.Control().ModuleAdmin().Reload(ctx)
}

func (m adminModuleAdmin) SetEnabled(ctx context.Context, moduleID string, enabled bool, actorID uint64) error {
	if m.app == nil || m.app.bot == nil {
		return liveDiscordUnavailable("module control")
	}
	return m.app.bot.Control().ModuleAdmin().SetEnabled(ctx, moduleID, enabled, actorID)
}

func (m adminModuleAdmin) Reset(ctx context.Context, moduleID string) error {
	if m.app == nil || m.app.bot == nil {
		return liveDiscordUnavailable("module control")
	}
	return m.app.bot.Control().ModuleAdmin().Reset(ctx, moduleID)
}

type adminPluginAdmin struct{ app *App }

func (p adminPluginAdmin) Configured() bool {
	return p.app != nil && p.app.bot != nil
}

func (p adminPluginAdmin) Infos() []pluginhost.PluginInfo {
	if p.app == nil || p.app.bot == nil {
		return nil
	}
	return p.app.bot.Control().PluginAdmin().Infos()
}

func (p adminPluginAdmin) Reload(ctx context.Context) error {
	if p.app == nil || p.app.bot == nil {
		return liveDiscordUnavailable("plugin reload")
	}
	return p.app.bot.Control().PluginAdmin().Reload(ctx)
}

func cloneOptionalUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func liveDiscordUnavailable(feature string) error {
	message := "discord runtime is unavailable"
	if feature = strings.TrimSpace(feature); feature != "" {
		message += " for " + feature
	}
	return &adminservice.PublicError{
		Status:  http.StatusServiceUnavailable,
		Message: message,
	}
}

func (a *App) ownerStatus() adminservice.OwnerStatus {
	if a != nil && a.bot != nil {
		status := a.bot.OwnerStatus()
		return adminservice.OwnerStatus{
			Configured:      status.Configured,
			Resolved:        status.Resolved,
			Source:          status.Source,
			EffectiveUserID: status.EffectiveUserID,
		}
	}

	status := adminservice.OwnerStatus{Source: "unresolved"}
	if a != nil && a.cfg.Discord.OwnerUserID != nil {
		status.Configured = true
		status.Resolved = true
		status.Source = "config_fallback"
		status.EffectiveUserID = cloneOptionalUint64(a.cfg.Discord.OwnerUserID)
	}
	return status
}

func (a *App) knownGuildIDs() []uint64 {
	if a == nil || a.bot == nil {
		return nil
	}
	return a.bot.Control().KnownGuildIDs()
}

func (a *App) botHasGuild(ctx context.Context, guildID uint64) (bool, error) {
	if a == nil || a.bot == nil {
		return false, liveDiscordUnavailable("guild install checks")
	}
	return a.bot.Control().HasGuild(ctx, guildID)
}

func (a *App) listGuildChannels(ctx context.Context, guildID uint64) ([]adminguilds.GuildChannelInfo, error) {
	if a == nil || a.bot == nil {
		return nil, liveDiscordUnavailable("channel listing")
	}
	items, err := a.bot.Control().ListGuildChannels(ctx, guildID)
	if err != nil {
		return nil, err
	}
	out := make([]adminguilds.GuildChannelInfo, 0, len(items))
	for _, item := range items {
		out = append(out, adminguilds.GuildChannelInfo{
			ID:       adminguilds.Snowflake(item.ID),
			Name:     item.Name,
			Type:     item.Type,
			ParentID: adminguilds.Snowflake(item.ParentID),
		})
	}
	return out, nil
}

func (a *App) listGuildRoles(ctx context.Context, guildID uint64) ([]adminguilds.GuildRoleInfo, error) {
	if a == nil || a.bot == nil {
		return nil, liveDiscordUnavailable("role listing")
	}
	items, err := a.bot.Control().ListGuildRoles(ctx, guildID)
	if err != nil {
		return nil, err
	}
	out := make([]adminguilds.GuildRoleInfo, 0, len(items))
	for _, item := range items {
		out = append(out, adminguilds.GuildRoleInfo{
			ID:          adminguilds.Snowflake(item.ID),
			Name:        item.Name,
			Color:       item.Color,
			Position:    item.Position,
			Managed:     item.Managed,
			Mentionable: item.Mentionable,
		})
	}
	return out, nil
}

func (a *App) searchGuildMembers(ctx context.Context, guildID uint64, query string, limit int) ([]adminguilds.GuildMemberInfo, error) {
	if a == nil || a.bot == nil {
		return nil, liveDiscordUnavailable("member search")
	}
	items, err := a.bot.Control().SearchGuildMembers(ctx, guildID, query, limit)
	if err != nil {
		return nil, err
	}
	out := make([]adminguilds.GuildMemberInfo, 0, len(items))
	for _, item := range items {
		roleIDs := make([]adminguilds.Snowflake, 0, len(item.RoleIDs))
		for _, roleID := range item.RoleIDs {
			roleIDs = append(roleIDs, adminguilds.Snowflake(roleID))
		}
		out = append(out, adminguilds.GuildMemberInfo{
			UserID:      adminguilds.Snowflake(item.UserID),
			Username:    item.Username,
			DisplayName: item.DisplayName,
			AvatarURL:   item.AvatarURL,
			Bot:         item.Bot,
			JoinedAt:    item.JoinedAt,
			RoleIDs:     roleIDs,
		})
	}
	return out, nil
}

func (a *App) listGuildEmojis(ctx context.Context, guildID uint64) ([]adminguilds.GuildEmojiInfo, error) {
	if a == nil || a.bot == nil {
		return nil, liveDiscordUnavailable("emoji listing")
	}
	items, err := a.bot.Control().ListGuildEmojis(ctx, guildID)
	if err != nil {
		return nil, err
	}
	out := make([]adminguilds.GuildEmojiInfo, 0, len(items))
	for _, item := range items {
		out = append(out, adminguilds.GuildEmojiInfo{
			ID:       adminguilds.Snowflake(item.ID),
			Name:     item.Name,
			Animated: item.Animated,
		})
	}
	return out, nil
}

func (a *App) listGuildStickers(ctx context.Context, guildID uint64) ([]adminguilds.GuildStickerInfo, error) {
	if a == nil || a.bot == nil {
		return nil, liveDiscordUnavailable("sticker listing")
	}
	items, err := a.bot.Control().ListGuildStickers(ctx, guildID)
	if err != nil {
		return nil, err
	}
	out := make([]adminguilds.GuildStickerInfo, 0, len(items))
	for _, item := range items {
		out = append(out, adminguilds.GuildStickerInfo{
			ID:          adminguilds.Snowflake(item.ID),
			Name:        item.Name,
			Description: item.Description,
			Tags:        item.Tags,
		})
	}
	return out, nil
}

func (a *App) setSlowmode(ctx context.Context, channelID uint64, seconds int) error {
	if a == nil || a.bot == nil {
		return liveDiscordUnavailable("slowmode control")
	}
	return a.bot.Control().SetSlowmode(ctx, channelID, seconds)
}

func (a *App) setNickname(ctx context.Context, guildID, userID uint64, nickname *string) error {
	if a == nil || a.bot == nil {
		return liveDiscordUnavailable("nickname control")
	}
	return a.bot.Control().SetNickname(ctx, guildID, userID, nickname)
}

func (a *App) timeoutMember(ctx context.Context, guildID, userID uint64, untilUnix int64) error {
	if a == nil || a.bot == nil {
		return liveDiscordUnavailable("member timeout")
	}
	return a.bot.Control().TimeoutMember(ctx, guildID, userID, untilUnix)
}

func (a *App) createRole(ctx context.Context, spec discordcontrol.RoleCreateSpec) (discordcontrol.RoleResult, error) {
	if a == nil || a.bot == nil {
		return discordcontrol.RoleResult{}, liveDiscordUnavailable("role control")
	}
	return a.bot.Control().CreateRole(ctx, spec)
}

func (a *App) editRole(ctx context.Context, spec discordcontrol.RoleEditSpec) (discordcontrol.RoleResult, error) {
	if a == nil || a.bot == nil {
		return discordcontrol.RoleResult{}, liveDiscordUnavailable("role control")
	}
	return a.bot.Control().EditRole(ctx, spec)
}

func (a *App) deleteRole(ctx context.Context, guildID, roleID uint64) error {
	if a == nil || a.bot == nil {
		return liveDiscordUnavailable("role control")
	}
	return a.bot.Control().DeleteRole(ctx, guildID, roleID)
}

func (a *App) addRole(ctx context.Context, spec discordcontrol.RoleMemberSpec) error {
	if a == nil || a.bot == nil {
		return liveDiscordUnavailable("role control")
	}
	return a.bot.Control().AddRole(ctx, spec)
}

func (a *App) removeRole(ctx context.Context, spec discordcontrol.RoleMemberSpec) error {
	if a == nil || a.bot == nil {
		return liveDiscordUnavailable("role control")
	}
	return a.bot.Control().RemoveRole(ctx, spec)
}

func (a *App) purgeMessages(ctx context.Context, spec discordcontrol.PurgeSpec) (int, error) {
	if a == nil || a.bot == nil {
		return 0, liveDiscordUnavailable("message purge")
	}
	return a.bot.Control().PurgeMessages(ctx, spec)
}

func (a *App) createEmojiUpload(ctx context.Context, guildID uint64, name, filename string, body []byte, width, height int) (discordcontrol.EmojiResult, error) {
	if a == nil || a.bot == nil {
		return discordcontrol.EmojiResult{}, liveDiscordUnavailable("emoji control")
	}
	return a.bot.Control().CreateEmojiUpload(ctx, guildID, name, filename, body, width, height)
}

func (a *App) editEmoji(ctx context.Context, spec discordcontrol.EmojiEditSpec) (discordcontrol.EmojiResult, error) {
	if a == nil || a.bot == nil {
		return discordcontrol.EmojiResult{}, liveDiscordUnavailable("emoji control")
	}
	return a.bot.Control().EditEmoji(ctx, spec)
}

func (a *App) deleteEmoji(ctx context.Context, spec discordcontrol.EmojiDeleteSpec) error {
	if a == nil || a.bot == nil {
		return liveDiscordUnavailable("emoji control")
	}
	return a.bot.Control().DeleteEmoji(ctx, spec)
}

func (a *App) createStickerUpload(ctx context.Context, guildID uint64, name, description, emojiTag, filename string, body []byte, width, height int) (discordcontrol.StickerResult, error) {
	if a == nil || a.bot == nil {
		return discordcontrol.StickerResult{}, liveDiscordUnavailable("sticker control")
	}
	return a.bot.Control().CreateStickerUpload(ctx, guildID, name, description, emojiTag, filename, body, width, height)
}

func (a *App) editSticker(ctx context.Context, spec discordcontrol.StickerEditSpec) (discordcontrol.StickerResult, error) {
	if a == nil || a.bot == nil {
		return discordcontrol.StickerResult{}, liveDiscordUnavailable("sticker control")
	}
	return a.bot.Control().EditSticker(ctx, spec)
}

func (a *App) deleteSticker(ctx context.Context, spec discordcontrol.StickerDeleteSpec) error {
	if a == nil || a.bot == nil {
		return liveDiscordUnavailable("sticker control")
	}
	return a.bot.Control().DeleteSticker(ctx, spec)
}

var _ moduleapi.Admin = adminModuleAdmin{}
var _ commandruntime.PluginAdmin = adminPluginAdmin{}
