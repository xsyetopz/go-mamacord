package plugin

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	pluginhostlua "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

func (e Executor) SelfUser(ctx context.Context) (pluginhostlua.UserResult, error) {
	if e.client() == nil {
		return pluginhostlua.UserResult{}, errors.New("discord client unavailable")
	}
	self, ok := e.client().Caches.SelfUser()
	if !ok {
		return pluginhostlua.UserResult{}, errors.New("self user unavailable")
	}

	result := pluginhostlua.UserResult{
		ID:          uint64(self.ID),
		Username:    strings.TrimSpace(self.Username),
		DisplayName: strings.TrimSpace(self.Username),
		Mention:     "<@" + self.ID.String() + ">",
		Bot:         true,
		CreatedAt:   self.ID.Time().UTC().Unix(),
	}
	if avatar := self.AvatarURL(); avatar != nil {
		result.AvatarURL = strings.TrimSpace(*avatar)
	}
	return result, nil
}

func (e Executor) GetUser(ctx context.Context, userID uint64) (pluginhostlua.UserResult, error) {
	if e.client() == nil {
		return pluginhostlua.UserResult{}, errors.New("discord client unavailable")
	}
	if userID == 0 {
		return pluginhostlua.UserResult{}, errors.New("invalid user")
	}

	user, err := e.client().Rest.GetUser(snowflake.ID(userID), rest.WithCtx(ctx))
	if err != nil || user == nil {
		return pluginhostlua.UserResult{}, errors.New("get_user_error")
	}
	return userResult(*user), nil
}

func (e Executor) GetMember(
	ctx context.Context,
	guildID, userID uint64,
) (pluginhostlua.MemberResult, error) {
	if e.client() == nil {
		return pluginhostlua.MemberResult{}, errors.New("discord client unavailable")
	}
	if guildID == 0 || userID == 0 {
		return pluginhostlua.MemberResult{}, errors.New("invalid member")
	}

	member, err := e.client().Rest.GetMember(snowflake.ID(guildID), snowflake.ID(userID), rest.WithCtx(ctx))
	if err != nil || member == nil {
		return pluginhostlua.MemberResult{}, errors.New("get_member_error")
	}
	return memberResult(uint64(guildID), *member), nil
}

func (e Executor) GetGuild(ctx context.Context, guildID uint64) (pluginhostlua.GuildResult, error) {
	if e.client() == nil {
		return pluginhostlua.GuildResult{}, errors.New("discord client unavailable")
	}
	if guildID == 0 {
		return pluginhostlua.GuildResult{}, errors.New("invalid guild")
	}

	guild, err := e.client().Rest.GetGuild(snowflake.ID(guildID), true, rest.WithCtx(ctx))
	if err != nil || guild == nil {
		return pluginhostlua.GuildResult{}, errors.New("get_guild_error")
	}

	channels, err := e.client().Rest.GetGuildChannels(snowflake.ID(guildID), rest.WithCtx(ctx))
	if err != nil {
		return pluginhostlua.GuildResult{}, errors.New("get_guild_error")
	}

	memberCount := guild.ApproximateMemberCount
	if memberCount <= 0 && guild.MemberCount > 0 {
		memberCount = guild.MemberCount
	}

	result := pluginhostlua.GuildResult{
		ID:            uint64(guild.ID),
		Name:          strings.TrimSpace(guild.Name),
		OwnerID:       uint64(guild.OwnerID),
		RolesCount:    len(guild.Roles),
		EmojisCount:   len(guild.Emojis),
		StickersCount: len(guild.Stickers),
		MemberCount:   memberCount,
		ChannelsCount: len(channels),
		CreatedAt:     guild.CreatedAt().UTC().Unix(),
	}
	if guild.Description != nil {
		result.Description = strings.TrimSpace(*guild.Description)
	}
	if icon := guild.IconURL(); icon != nil {
		result.IconURL = strings.TrimSpace(*icon)
	}
	if banner := guild.BannerURL(); banner != nil {
		result.BannerURL = strings.TrimSpace(*banner)
	}
	return result, nil
}

func (e Executor) GetRole(
	ctx context.Context,
	guildID, roleID uint64,
) (pluginhostlua.RoleResult, error) {
	if e.client() == nil {
		return pluginhostlua.RoleResult{}, errors.New("discord client unavailable")
	}
	if guildID == 0 || roleID == 0 {
		return pluginhostlua.RoleResult{}, errors.New("invalid role")
	}

	guild, err := e.client().Rest.GetGuild(snowflake.ID(guildID), false, rest.WithCtx(ctx))
	if err != nil || guild == nil {
		return pluginhostlua.RoleResult{}, errors.New("get_role_error")
	}
	for _, role := range guild.Roles {
		if uint64(role.ID) == roleID {
			return roleResult(role), nil
		}
	}
	return pluginhostlua.RoleResult{}, errors.New("get_role_error")
}

func (e Executor) GetChannel(ctx context.Context, channelID uint64) (pluginhostlua.ChannelResult, error) {
	if e.client() == nil {
		return pluginhostlua.ChannelResult{}, errors.New("discord client unavailable")
	}
	if channelID == 0 {
		return pluginhostlua.ChannelResult{}, errors.New("invalid channel")
	}

	channel, err := e.client().Rest.GetChannel(snowflake.ID(channelID), rest.WithCtx(ctx))
	if err != nil || channel == nil {
		return pluginhostlua.ChannelResult{}, errors.New("get_channel_error")
	}

	result := pluginhostlua.ChannelResult{
		ID:        uint64(channel.ID()),
		Name:      strings.TrimSpace(channel.Name()),
		Mention:   discord.ChannelMention(channel.ID()),
		Type:      pluginChannelTypeName(channel.Type()),
		CreatedAt: channel.CreatedAt().UTC().Unix(),
	}
	if guildChannel, ok := channel.(discord.GuildChannel); ok {
		if parentID := guildChannel.ParentID(); parentID != nil {
			result.ParentID = uint64(*parentID)
		}
	}
	return result, nil
}

func (e Executor) GetMessage(
	ctx context.Context,
	spec pluginhostlua.MessageGetSpec,
) (pluginhostlua.MessageInfo, error) {
	if e.client() == nil {
		return pluginhostlua.MessageInfo{}, errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.MessageID == 0 {
		return pluginhostlua.MessageInfo{}, errors.New("invalid message")
	}

	message, err := e.client().Rest.GetMessage(
		snowflake.ID(spec.ChannelID),
		snowflake.ID(spec.MessageID),
		rest.WithCtx(ctx),
	)
	if err != nil || message == nil {
		return pluginhostlua.MessageInfo{}, errors.New("get_message_error")
	}
	return messageInfo(*message), nil
}

func userResult(user discord.User) pluginhostlua.UserResult {
	result := pluginhostlua.UserResult{
		ID:          uint64(user.ID),
		Username:    strings.TrimSpace(user.Username),
		DisplayName: strings.TrimSpace(user.EffectiveName()),
		Mention:     user.Mention(),
		Bot:         user.Bot,
		System:      user.System,
		CreatedAt:   user.CreatedAt().UTC().Unix(),
	}
	if user.AccentColor != nil {
		result.AccentColor = *user.AccentColor
	}
	if avatar := user.AvatarURL(); avatar != nil {
		result.AvatarURL = strings.TrimSpace(*avatar)
	} else {
		result.AvatarURL = strings.TrimSpace(user.EffectiveAvatarURL())
	}
	if banner := user.BannerURL(); banner != nil {
		result.BannerURL = strings.TrimSpace(*banner)
	}
	return result
}

func memberResult(guildID uint64, member discord.Member) pluginhostlua.MemberResult {
	result := pluginhostlua.MemberResult{
		UserID:  uint64(member.User.ID),
		GuildID: guildID,
	}
	if member.JoinedAt != nil && !member.JoinedAt.IsZero() {
		result.JoinedAt = member.JoinedAt.UTC().Unix()
	}
	result.RoleIDs = make([]uint64, 0, len(member.RoleIDs))
	for _, roleID := range member.RoleIDs {
		result.RoleIDs = append(result.RoleIDs, uint64(roleID))
	}
	if avatar := strings.TrimSpace(member.EffectiveAvatarURL()); avatar != "" {
		result.AvatarURL = avatar
	}
	if banner := member.EffectiveBannerURL(); banner != "" {
		result.BannerURL = strings.TrimSpace(banner)
	}
	return result
}

func messageInfo(message discord.Message) pluginhostlua.MessageInfo {
	info := pluginhostlua.MessageInfo{
		ID:        uint64(message.ID),
		ChannelID: uint64(message.ChannelID),
		AuthorID:  uint64(message.Author.ID),
		Content:   message.Content,
		CreatedAt: message.CreatedAt.UTC().Unix(),
		Pinned:    message.Pinned,
	}
	if message.EditedTimestamp != nil && !message.EditedTimestamp.IsZero() {
		info.EditedAt = message.EditedTimestamp.UTC().Unix()
	}
	return info
}

func pluginChannelTypeName(t discord.ChannelType) string {
	switch t {
	case discord.ChannelTypeGuildText:
		return "guild_text"
	case discord.ChannelTypeGuildVoice:
		return "guild_voice"
	case discord.ChannelTypeGuildCategory:
		return "guild_category"
	case discord.ChannelTypeGuildNews:
		return "guild_news"
	case discord.ChannelTypeGuildNewsThread:
		return "guild_news_thread"
	case discord.ChannelTypeGuildPublicThread:
		return "guild_public_thread"
	case discord.ChannelTypeGuildPrivateThread:
		return "guild_private_thread"
	case discord.ChannelTypeGuildStageVoice:
		return "guild_stage_voice"
	case discord.ChannelTypeGuildDirectory:
		return "guild_directory"
	case discord.ChannelTypeGuildForum:
		return "guild_forum"
	case discord.ChannelTypeGuildMedia:
		return "guild_media"
	case discord.ChannelTypeDM:
		return "dm"
	case discord.ChannelTypeGroupDM:
		return "group_dm"
	default:
		return strconv.Itoa(int(t))
	}
}
