package gateway

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"

	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

const (
	EventMemberJoin  = "guild_member_join"
	EventMemberLeave = "guild_member_leave"
	EventGuildBan    = "guild_ban"
	EventGuildUnban  = "guild_unban"
)

type PluginEmitter interface {
	FireEvent(eventName string, payload pluginhost.Payload)
}

type Handlers struct {
	Logger                   *slog.Logger
	Store                    commandruntime.Store
	I18n                     i18n.Registry
	Client                   *bot.Client
	CommandRegisterAllGuilds bool
	DevGuildID               *uint64
	CommandCreates           func(locales []string) []discord.ApplicationCommandCreate
	PluginEvents             PluginEmitter
}

func (h Handlers) OnGuildJoin(e *events.GuildJoin) {
	if e == nil || h.Store == nil {
		return
	}

	ctx := context.Background()
	guildID := uint64(e.GuildID)

	restrictions := h.Store.Restrictions()
	if _, ok, err := restrictions.GetRestriction(ctx, store.TargetTypeGuild, guildID); err != nil {
		h.logger().Error(
			"guild restriction check failed",
			slog.String("err", err.Error()),
			slog.Uint64("guild_id", guildID),
		)
	} else if ok {
		h.logger().Warn("leaving restricted guild", slog.Uint64("guild_id", guildID))
		_ = e.Client().Rest.LeaveGuild(snowflake.ID(guildID))
		return
	}

	now := time.Now().UTC()
	guildName := strings.TrimSpace(e.Guild.Name)
	ownerID := uint64(e.Guild.OwnerID)

	_ = h.Store.Guilds().UpsertGuildSeen(ctx, store.GuildSeen{
		GuildID:   guildID,
		OwnerID:   ownerID,
		CreatedAt: e.Guild.ID.Time().UTC(),
		JoinedAt:  now,
		LeftAt:    nil,
		Name:      guildName,
		UpdatedAt: now,
	})

	if ownerID != 0 {
		owner, _ := e.Client().Rest.GetUser(snowflake.ID(ownerID))
		isBot := false
		isSystem := false
		if owner != nil {
			isBot = owner.Bot
			isSystem = owner.System
		}

		_ = h.Store.Users().UpsertUserSeen(ctx, store.UserSeen{
			UserID:      ownerID,
			CreatedAt:   snowflake.ID(ownerID).Time().UTC(),
			IsBot:       isBot,
			IsSystem:    isSystem,
			FirstSeenAt: now,
			LastSeenAt:  now,
		})
	}

	h.logger().Info(
		"joined guild",
		slog.Uint64("guild_id", guildID),
		slog.String("guild_name", guildName),
		slog.Uint64("owner_id", ownerID),
	)

	if h.DevGuildID == nil && h.CommandRegisterAllGuilds && h.Client != nil && h.CommandCreates != nil {
		locales := h.I18n.SupportedLocales()
		creates := h.CommandCreates(locales)
		if _, err := h.Client.Rest.SetGuildCommands(h.Client.ApplicationID, snowflake.ID(guildID), creates); err != nil {
			h.logger().Error(
				"register commands in joined guild failed",
				slog.String("err", err.Error()),
				slog.Uint64("guild_id", guildID),
			)
		}
	}
}

func (h Handlers) OnGuildLeave(e *events.GuildLeave) {
	if e == nil || h.Store == nil {
		return
	}
	ctx := context.Background()
	now := time.Now().UTC()
	guildID := uint64(e.GuildID)

	_ = h.Store.Guilds().MarkGuildLeft(ctx, guildID, now)

	guildName := strings.TrimSpace(e.Guild.Name)
	h.logger().Info("left guild", slog.Uint64("guild_id", guildID), slog.String("guild_name", guildName))
}

func (h Handlers) OnGuildUpdate(e *events.GuildUpdate) {
	if e == nil || h.Store == nil {
		return
	}

	ctx := context.Background()
	guildID := uint64(e.GuildID)
	newOwner := uint64(e.Guild.OwnerID)
	now := time.Now().UTC()

	guildName := strings.TrimSpace(e.Guild.Name)
	_ = h.Store.Guilds().UpsertGuildSeen(ctx, store.GuildSeen{
		GuildID:   guildID,
		OwnerID:   newOwner,
		CreatedAt: e.Guild.ID.Time().UTC(),
		JoinedAt:  now,
		LeftAt:    nil,
		Name:      guildName,
		UpdatedAt: now,
	})

	h.logger().Info(
		"guild updated",
		slog.Uint64("guild_id", guildID),
		slog.String("guild_name", guildName),
		slog.Uint64("owner_id", newOwner),
	)
}

func (h Handlers) OnGuildMemberJoin(e *events.GuildMemberJoin) {
	if e == nil || h.Store == nil {
		return
	}

	user := e.Member.User
	if user.ID == 0 || user.Bot || user.System {
		return
	}

	ctx := context.Background()
	now := time.Now().UTC()
	guildID := uint64(e.GuildID)
	userID := uint64(user.ID)

	_ = h.Store.Users().UpsertUserSeen(ctx, store.UserSeen{
		UserID:      userID,
		CreatedAt:   user.ID.Time().UTC(),
		IsBot:       user.Bot,
		IsSystem:    user.System,
		FirstSeenAt: now,
		LastSeenAt:  now,
	})
	_ = h.Store.GuildMembers().MarkMemberJoined(ctx, guildID, userID, now)

	h.logger().Info(
		"member joined",
		slog.Uint64("guild_id", guildID),
		slog.Uint64("user_id", userID),
		slog.String("username", strings.TrimSpace(user.Username)),
	)

	if h.PluginEvents != nil {
		h.PluginEvents.FireEvent(EventMemberJoin, pluginhost.Payload{
			GuildID:   snowflake.ID(guildID).String(),
			ChannelID: "",
			UserID:    user.ID.String(),
			Locale:    "",
			Options:   map[string]any{},
		})
	}
}

func (h Handlers) OnGuildMemberLeave(e *events.GuildMemberLeave) {
	if e == nil || h.Store == nil {
		return
	}

	user := e.User
	if user.ID == 0 || user.Bot || user.System {
		return
	}

	ctx := context.Background()
	now := time.Now().UTC()
	guildID := uint64(e.GuildID)
	userID := uint64(user.ID)

	_ = h.Store.GuildMembers().MarkMemberLeft(ctx, guildID, userID, now)
	_ = h.Store.Users().TouchUserSeen(ctx, userID, now)

	h.logger().Info(
		"member left",
		slog.Uint64("guild_id", guildID),
		slog.Uint64("user_id", userID),
		slog.String("username", strings.TrimSpace(user.Username)),
	)

	if h.PluginEvents != nil {
		h.PluginEvents.FireEvent(EventMemberLeave, pluginhost.Payload{
			GuildID:   snowflake.ID(guildID).String(),
			ChannelID: "",
			UserID:    user.ID.String(),
			Locale:    "",
			Options:   map[string]any{},
		})
	}
}

func (h Handlers) OnGuildBan(e *events.GuildBan) {
	if e == nil {
		return
	}
	h.logger().Info(
		"user banned",
		slog.Uint64("guild_id", uint64(e.GuildID)),
		slog.Uint64("user_id", uint64(e.User.ID)),
		slog.String("username", strings.TrimSpace(e.User.Username)),
	)

	if h.PluginEvents != nil {
		h.PluginEvents.FireEvent(EventGuildBan, pluginhost.Payload{
			GuildID:   e.GuildID.String(),
			ChannelID: "",
			UserID:    e.User.ID.String(),
			Locale:    "",
			Options:   map[string]any{},
		})
	}
}

func (h Handlers) OnGuildUnban(e *events.GuildUnban) {
	if e == nil {
		return
	}
	h.logger().Info(
		"user unbanned",
		slog.Uint64("guild_id", uint64(e.GuildID)),
		slog.Uint64("user_id", uint64(e.User.ID)),
		slog.String("username", strings.TrimSpace(e.User.Username)),
	)

	if h.PluginEvents != nil {
		h.PluginEvents.FireEvent(EventGuildUnban, pluginhost.Payload{
			GuildID:   e.GuildID.String(),
			ChannelID: "",
			UserID:    e.User.ID.String(),
			Locale:    "",
			Options:   map[string]any{},
		})
	}
}

func (h Handlers) OnGuildChannelCreate(e *events.GuildChannelCreate) {
	if e == nil {
		return
	}
	h.logger().Info(
		"channel created",
		slog.Uint64("guild_id", uint64(e.GuildID)),
		slog.Uint64("channel_id", uint64(e.ChannelID)),
		slog.String("channel_name", strings.TrimSpace(e.Channel.Name())),
	)
}

func (h Handlers) OnGuildChannelDelete(e *events.GuildChannelDelete) {
	if e == nil {
		return
	}
	h.logger().Info(
		"channel deleted",
		slog.Uint64("guild_id", uint64(e.GuildID)),
		slog.Uint64("channel_id", uint64(e.ChannelID)),
		slog.String("channel_name", strings.TrimSpace(e.Channel.Name())),
	)
}

func (h Handlers) OnRoleCreate(e *events.RoleCreate) {
	if e == nil {
		return
	}
	h.logger().Info(
		"role created",
		slog.Uint64("guild_id", uint64(e.GuildID)),
		slog.Uint64("role_id", uint64(e.RoleID)),
		slog.String("role_name", strings.TrimSpace(e.Role.Name)),
	)
}

func (h Handlers) OnRoleDelete(e *events.RoleDelete) {
	if e == nil {
		return
	}
	h.logger().Info(
		"role deleted",
		slog.Uint64("guild_id", uint64(e.GuildID)),
		slog.Uint64("role_id", uint64(e.RoleID)),
		slog.String("role_name", strings.TrimSpace(e.Role.Name)),
	)
}

func (h Handlers) OnInviteCreate(e *events.InviteCreate) {
	if e == nil {
		return
	}

	guildID := uint64(0)
	if e.GuildID != nil {
		guildID = uint64(*e.GuildID)
	}
	inviterID := uint64(0)
	inviterName := ""
	if e.Inviter != nil {
		inviterID = uint64(e.Inviter.ID)
		inviterName = strings.TrimSpace(e.Inviter.Username)
	}

	h.logger().Info(
		"invite created",
		slog.Uint64("guild_id", guildID),
		slog.Uint64("channel_id", uint64(e.ChannelID)),
		slog.String("code", strings.TrimSpace(e.Code)),
		slog.Uint64("inviter_id", inviterID),
		slog.String("inviter_name", inviterName),
	)
}

func (h Handlers) OnInviteDelete(e *events.InviteDelete) {
	if e == nil {
		return
	}

	guildID := uint64(0)
	if e.GuildID != nil {
		guildID = uint64(*e.GuildID)
	}

	h.logger().Info(
		"invite deleted",
		slog.Uint64("guild_id", guildID),
		slog.Uint64("channel_id", uint64(e.ChannelID)),
		slog.String("code", strings.TrimSpace(e.Code)),
	)
}

func (h Handlers) logger() *slog.Logger {
	if h.Logger != nil {
		return h.Logger
	}
	return slog.Default()
}
