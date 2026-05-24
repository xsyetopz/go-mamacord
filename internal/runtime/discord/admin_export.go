package discordruntime

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	discordpluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	pluginhostlua "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

type GuildChannelInfo struct {
	ID       uint64
	Name     string
	Type     string
	ParentID uint64
}

type GuildRoleInfo struct {
	ID          uint64
	Name        string
	Color       int
	Position    int
	Managed     bool
	Mentionable bool
}

type GuildMemberInfo struct {
	UserID      uint64
	Username    string
	DisplayName string
	AvatarURL   string
	Bot         bool
	JoinedAt    int64
	RoleIDs     []uint64
}

type GuildEmojiInfo struct {
	ID       uint64
	Name     string
	Animated bool
}

type GuildStickerInfo struct {
	ID          uint64
	Name        string
	Description string
	Tags        string
}

func (b *Bot) ModuleInfos() []moduleapi.Info {
	if b == nil {
		return nil
	}
	return b.moduleInfos()
}

func (b *Bot) PluginInfos() []pluginhost.PluginInfo {
	if b == nil || b.pluginHost == nil {
		return nil
	}
	return b.pluginHost.Infos()
}

func (b *Bot) ReloadModules(ctx context.Context) error {
	if b == nil {
		return nil
	}
	return b.reloadModules(ctx)
}

func (b *Bot) SetModuleEnabled(ctx context.Context, moduleID string, enabled bool, actorID uint64) error {
	if b == nil {
		return nil
	}
	return b.setModuleEnabled(ctx, moduleID, enabled, actorID)
}

func (b *Bot) ResetModule(ctx context.Context, moduleID string) error {
	if b == nil {
		return nil
	}
	return b.resetModule(ctx, moduleID)
}

func (b *Bot) KnownGuildIDs() []uint64 {
	if b == nil || b.client == nil {
		return nil
	}
	guilds := make([]uint64, 0, b.client.Caches.GuildsLen())
	for guild := range b.client.Caches.Guilds() {
		guilds = append(guilds, uint64(guild.ID))
	}
	slices.Sort(guilds)
	return guilds
}

func (b *Bot) HasGuild(ctx context.Context, guildID uint64) (bool, error) {
	if b == nil || b.client == nil {
		return false, errors.New("discord client unavailable")
	}

	_, err := b.client.Rest.GetGuild(snowflake.ID(guildID), false, rest.WithCtx(ctx))
	if err == nil {
		return true, nil
	}

	var restErr *rest.Error
	if errors.As(err, &restErr) && restErr.Response != nil {
		switch restErr.Response.StatusCode {
		case 403, 404:
			// Not installed, or missing access.
			return false, nil
		}
	}
	return false, err
}

func (b *Bot) ListGuildChannels(ctx context.Context, guildID uint64) ([]GuildChannelInfo, error) {
	if b == nil || b.client == nil {
		return nil, errors.New("discord client unavailable")
	}
	channels, err := b.client.Rest.GetGuildChannels(snowflake.ID(guildID), rest.WithCtx(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]GuildChannelInfo, 0, len(channels))
	for _, channel := range channels {
		item := GuildChannelInfo{
			ID:   uint64(channel.ID()),
			Name: strings.TrimSpace(channel.Name()),
			Type: channelTypeName(channel.Type()),
		}
		if parentID := channel.ParentID(); parentID != nil {
			item.ParentID = uint64(*parentID)
		}
		out = append(out, item)
	}
	slices.SortFunc(out, func(a, b GuildChannelInfo) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}

func (b *Bot) ListGuildRoles(ctx context.Context, guildID uint64) ([]GuildRoleInfo, error) {
	if b == nil || b.client == nil {
		return nil, errors.New("discord client unavailable")
	}
	roles, err := b.client.Rest.GetRoles(snowflake.ID(guildID), rest.WithCtx(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]GuildRoleInfo, 0, len(roles))
	for _, role := range roles {
		out = append(out, GuildRoleInfo{
			ID:          uint64(role.ID),
			Name:        strings.TrimSpace(role.Name),
			Color:       role.Color,
			Position:    role.Position,
			Managed:     role.Managed,
			Mentionable: role.Mentionable,
		})
	}
	slices.SortFunc(out, func(a, b GuildRoleInfo) int {
		if a.Position == b.Position {
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		}
		if a.Position > b.Position {
			return -1
		}
		return 1
	})
	return out, nil
}

func (b *Bot) SearchGuildMembers(ctx context.Context, guildID uint64, query string, limit int) ([]GuildMemberInfo, error) {
	if b == nil || b.client == nil {
		return nil, errors.New("discord client unavailable")
	}
	if limit <= 0 || limit > 25 {
		limit = 25
	}
	var (
		members []discord.Member
		err     error
	)
	query = strings.TrimSpace(query)
	if query == "" {
		members, err = b.client.Rest.GetMembers(snowflake.ID(guildID), limit, 0, rest.WithCtx(ctx))
	} else {
		members, err = b.client.Rest.SearchMembers(snowflake.ID(guildID), query, limit, rest.WithCtx(ctx))
	}
	if err != nil {
		return nil, err
	}
	out := make([]GuildMemberInfo, 0, len(members))
	for _, member := range members {
		item := GuildMemberInfo{
			UserID:      uint64(member.User.ID),
			Username:    strings.TrimSpace(member.User.Username),
			DisplayName: strings.TrimSpace(member.User.EffectiveName()),
			AvatarURL:   strings.TrimSpace(member.User.EffectiveAvatarURL()),
			Bot:         member.User.Bot,
			RoleIDs:     make([]uint64, 0, len(member.RoleIDs)),
		}
		if member.JoinedAt != nil && !member.JoinedAt.IsZero() {
			item.JoinedAt = member.JoinedAt.UTC().Unix()
		}
		for _, roleID := range member.RoleIDs {
			item.RoleIDs = append(item.RoleIDs, uint64(roleID))
		}
		out = append(out, item)
	}
	return out, nil
}

func (b *Bot) ListGuildEmojis(ctx context.Context, guildID uint64) ([]GuildEmojiInfo, error) {
	if b == nil || b.client == nil {
		return nil, errors.New("discord client unavailable")
	}
	emojis, err := b.client.Rest.GetEmojis(snowflake.ID(guildID), rest.WithCtx(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]GuildEmojiInfo, 0, len(emojis))
	for _, emoji := range emojis {
		out = append(out, GuildEmojiInfo{
			ID:       uint64(emoji.ID),
			Name:     strings.TrimSpace(emoji.Name),
			Animated: emoji.Animated,
		})
	}
	slices.SortFunc(out, func(a, b GuildEmojiInfo) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}

func (b *Bot) ListGuildStickers(ctx context.Context, guildID uint64) ([]GuildStickerInfo, error) {
	if b == nil || b.client == nil {
		return nil, errors.New("discord client unavailable")
	}
	stickers, err := b.client.Rest.GetStickers(snowflake.ID(guildID), rest.WithCtx(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]GuildStickerInfo, 0, len(stickers))
	for _, sticker := range stickers {
		item := GuildStickerInfo{
			ID:   uint64(sticker.ID),
			Name: strings.TrimSpace(sticker.Name),
			Tags: strings.TrimSpace(sticker.Tags),
		}
		item.Description = strings.TrimSpace(sticker.Description)
		out = append(out, item)
	}
	slices.SortFunc(out, func(a, b GuildStickerInfo) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}

func (b *Bot) SetSlowmode(ctx context.Context, channelID uint64, seconds int) error {
	return b.executor().SetSlowmode(ctx, channelID, seconds)
}

func (b *Bot) SetNickname(ctx context.Context, guildID, userID uint64, nickname *string) error {
	return b.executor().SetNickname(ctx, guildID, userID, nickname)
}

func (b *Bot) TimeoutMember(ctx context.Context, guildID, userID uint64, untilUnix int64) error {
	return b.executor().TimeoutMember(ctx, guildID, userID, time.Unix(untilUnix, 0).UTC())
}

func (b *Bot) CreateRole(ctx context.Context, spec pluginhostlua.RoleCreateSpec) (pluginhostlua.RoleResult, error) {
	return b.executor().CreateRole(ctx, spec)
}

func (b *Bot) EditRole(ctx context.Context, spec pluginhostlua.RoleEditSpec) (pluginhostlua.RoleResult, error) {
	return b.executor().EditRole(ctx, spec)
}

func (b *Bot) DeleteRole(ctx context.Context, guildID, roleID uint64) error {
	return b.executor().DeleteRole(ctx, guildID, roleID)
}

func (b *Bot) AddRole(ctx context.Context, spec pluginhostlua.RoleMemberSpec) error {
	return b.executor().AddRole(ctx, spec)
}

func (b *Bot) RemoveRole(ctx context.Context, spec pluginhostlua.RoleMemberSpec) error {
	return b.executor().RemoveRole(ctx, spec)
}

func (b *Bot) PurgeMessages(ctx context.Context, spec pluginhostlua.PurgeSpec) (int, error) {
	return b.executor().PurgeMessages(ctx, spec)
}

func (b *Bot) CreateEmojiUpload(ctx context.Context, guildID uint64, name, filename string, body []byte, width, height int) (pluginhostlua.EmojiResult, error) {
	return b.executor().CreateEmojiUpload(ctx, guildID, name, filename, body, width, height)
}

func (b *Bot) EditEmoji(ctx context.Context, spec pluginhostlua.EmojiEditSpec) (pluginhostlua.EmojiResult, error) {
	return b.executor().EditEmoji(ctx, spec)
}

func (b *Bot) DeleteEmoji(ctx context.Context, spec pluginhostlua.EmojiDeleteSpec) error {
	return b.executor().DeleteEmoji(ctx, spec)
}

func (b *Bot) CreateStickerUpload(
	ctx context.Context,
	guildID uint64,
	name, description, emojiTag, filename string,
	body []byte,
	width, height int,
) (pluginhostlua.StickerResult, error) {
	return b.executor().CreateStickerUpload(ctx, guildID, name, description, emojiTag, filename, body, width, height)
}

func (b *Bot) EditSticker(ctx context.Context, spec pluginhostlua.StickerEditSpec) (pluginhostlua.StickerResult, error) {
	return b.executor().EditSticker(ctx, spec)
}

func (b *Bot) DeleteSticker(ctx context.Context, spec pluginhostlua.StickerDeleteSpec) error {
	return b.executor().DeleteSticker(ctx, spec)
}

func (b *Bot) executor() discordpluginbridge.Executor {
	return discordpluginbridge.Executor{
		ClientProvider:      func() *bot.Client { return b.client },
		EnsureDMChannelFunc: b.ensureDMChannel,
	}
}

func channelTypeName(t discord.ChannelType) string {
	switch t {
	case discord.ChannelTypeGuildText:
		return "guild_text"
	case discord.ChannelTypeGuildVoice:
		return "guild_voice"
	case discord.ChannelTypeGuildCategory:
		return "guild_category"
	case discord.ChannelTypeGuildNews:
		return "guild_news"
	case discord.ChannelTypeGuildStageVoice:
		return "guild_stage_voice"
	case discord.ChannelTypeGuildForum:
		return "guild_forum"
	default:
		return "unknown"
	}
}
