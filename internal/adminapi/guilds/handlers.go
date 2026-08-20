package guilds

import (
	"errors"
	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
	"net/http"
	"strconv"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/guildconfig"
	discordcontrol "github.com/xsyetopz/go-mamacord/internal/runtime/discord/control"
)

func (handler *Handler) HandleGuildConfig(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID  Snowflake `json:"guild_id"`
		PluginID string    `json:"plugin_id"`
		Config   struct {
			Enabled                  bool            `json:"enabled"`
			Commands                 map[string]bool `json:"commands"`
			WarningLimit             int             `json:"warning_limit,omitempty"`
			TimeoutThreshold         int             `json:"timeout_threshold,omitempty"`
			TimeoutMinutes           int             `json:"timeout_minutes,omitempty"`
			AllowChannelReminders    bool            `json:"allow_channel_reminders,omitempty"`
			DefaultReminderChannelID Snowflake       `json:"default_reminder_channel_id,omitempty"`
		} `json:"config"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := handler.service.PutGuildConfig(r.Context(), sess.AccessToken, uint64(req.GuildID), strings.TrimSpace(req.PluginID), guildconfig.PluginConfig{
		Enabled:                  req.Config.Enabled,
		Commands:                 req.Config.Commands,
		WarningLimit:             req.Config.WarningLimit,
		TimeoutThreshold:         req.Config.TimeoutThreshold,
		TimeoutMinutes:           req.Config.TimeoutMinutes,
		AllowChannelReminders:    req.Config.AllowChannelReminders,
		DefaultReminderChannelID: uint64(req.Config.DefaultReminderChannelID),
	})
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"config": cfg})
}

func (handler *Handler) HandleGuildChannels(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	guildID, ok := handler.guildIDQuery(w, r)
	if !ok {
		return
	}
	items, err := handler.service.GuildChannels(r.Context(), sess.AccessToken, guildID)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"channels": items})
}

func (handler *Handler) HandleGuildRoles(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	guildID, ok := handler.guildIDQuery(w, r)
	if !ok {
		return
	}
	items, err := handler.service.GuildRoles(r.Context(), sess.AccessToken, guildID)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"roles": items})
}

func (handler *Handler) HandleGuildMembers(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	guildID, ok := handler.guildIDQuery(w, r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := handler.service.GuildMembers(r.Context(), sess.AccessToken, guildID, strings.TrimSpace(r.URL.Query().Get("query")), limit)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"members": items})
}

func (handler *Handler) HandleGuildEmojis(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	guildID, ok := handler.guildIDQuery(w, r)
	if !ok {
		return
	}
	items, err := handler.service.GuildEmojis(r.Context(), sess.AccessToken, guildID)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"emojis": items})
}

func (handler *Handler) HandleGuildStickers(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	guildID, ok := handler.guildIDQuery(w, r)
	if !ok {
		return
	}
	items, err := handler.service.GuildStickers(r.Context(), sess.AccessToken, guildID)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"stickers": items})
}

func (handler *Handler) HandleGuildWarnings(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	guildID, ok := handler.guildIDQuery(w, r)
	if !ok {
		return
	}
	userID, err := strconv.ParseUint(strings.TrimSpace(r.URL.Query().Get("user_id")), 10, 64)
	if err != nil || userID == 0 {
		handler.responder.Error(w, http.StatusBadRequest, "invalid user_id")
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	if limit <= 0 {
		limit = 25
	}
	items, err := handler.service.GuildWarnings(r.Context(), sess.AccessToken, guildID, userID, limit)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"warnings": items})
}

func (handler *Handler) HandleGuildWarn(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID Snowflake `json:"guild_id"`
		UserID  Snowflake `json:"user_id"`
		Reason  string    `json:"reason"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	result, err := handler.service.CreateWarning(r.Context(), sess.AccessToken, uint64(req.GuildID), sess.UserID, uint64(req.UserID), req.Reason)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, result)
}

func (handler *Handler) HandleGuildUnwarn(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID   Snowflake `json:"guild_id"`
		WarningID string    `json:"warning_id"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := handler.service.DeleteWarning(r.Context(), sess.AccessToken, uint64(req.GuildID), sess.UserID, req.WarningID); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *Handler) HandleGuildSlowmode(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID   Snowflake `json:"guild_id"`
		ChannelID Snowflake `json:"channel_id"`
		Seconds   int       `json:"seconds"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := handler.service.ManagerSlowmode(r.Context(), sess.AccessToken, uint64(req.GuildID), uint64(req.ChannelID), req.Seconds); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *Handler) HandleGuildNickname(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID  Snowflake `json:"guild_id"`
		UserID   Snowflake `json:"user_id"`
		Nickname string    `json:"nickname"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	var nickname *string
	if strings.TrimSpace(req.Nickname) != "" {
		value := strings.TrimSpace(req.Nickname)
		nickname = &value
	}
	if err := handler.service.ManagerNickname(r.Context(), sess.AccessToken, uint64(req.GuildID), uint64(req.UserID), nickname); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *Handler) HandleGuildRoleCreate(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID     Snowflake `json:"guild_id"`
		Name        string    `json:"name"`
		Color       *int      `json:"color"`
		Hoist       *bool     `json:"hoist"`
		Mentionable *bool     `json:"mentionable"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	role, err := handler.service.ManagerCreateRole(r.Context(), sess.AccessToken, discordcontrol.RoleCreateSpec{
		GuildID:     uint64(req.GuildID),
		Name:        req.Name,
		Color:       req.Color,
		Hoist:       req.Hoist,
		Mentionable: req.Mentionable,
	})
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"role": role})
}

func (handler *Handler) HandleGuildRoleEdit(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID     Snowflake `json:"guild_id"`
		RoleID      Snowflake `json:"role_id"`
		Name        *string   `json:"name"`
		Color       *int      `json:"color"`
		Hoist       *bool     `json:"hoist"`
		Mentionable *bool     `json:"mentionable"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	role, err := handler.service.ManagerEditRole(r.Context(), sess.AccessToken, discordcontrol.RoleEditSpec{
		GuildID:     uint64(req.GuildID),
		RoleID:      uint64(req.RoleID),
		Name:        req.Name,
		Color:       req.Color,
		Hoist:       req.Hoist,
		Mentionable: req.Mentionable,
	})
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"role": role})
}

func (handler *Handler) HandleGuildRoleDelete(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID Snowflake `json:"guild_id"`
		RoleID  Snowflake `json:"role_id"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := handler.service.ManagerDeleteRole(r.Context(), sess.AccessToken, uint64(req.GuildID), uint64(req.RoleID)); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *Handler) HandleGuildRoleMember(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		Add     bool      `json:"add"`
		GuildID Snowflake `json:"guild_id"`
		UserID  Snowflake `json:"user_id"`
		RoleID  Snowflake `json:"role_id"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := handler.service.ManagerMemberRole(r.Context(), sess.AccessToken, req.Add, discordcontrol.RoleMemberSpec{
		GuildID: uint64(req.GuildID),
		UserID:  uint64(req.UserID),
		RoleID:  uint64(req.RoleID),
	}); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *Handler) HandleGuildPurge(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID   Snowflake `json:"guild_id"`
		ChannelID Snowflake `json:"channel_id"`
		Mode      string    `json:"mode"`
		AnchorRaw string    `json:"anchor_raw"`
		Count     int       `json:"count"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	deleted, err := handler.service.ManagerPurge(r.Context(), sess.AccessToken, uint64(req.GuildID), discordcontrol.PurgeSpec{
		ChannelID: uint64(req.ChannelID),
		Mode:      req.Mode,
		AnchorRaw: req.AnchorRaw,
		Count:     req.Count,
	})
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"deleted_count": deleted})
}

func (handler *Handler) HandleGuildEmojiCreate(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID    Snowflake `json:"guild_id"`
		Name       string    `json:"name"`
		Filename   string    `json:"filename"`
		ContentB64 string    `json:"content_b64"`
		Width      int       `json:"width"`
		Height     int       `json:"height"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	emoji, err := handler.service.ManagerCreateEmoji(r.Context(), sess.AccessToken, uint64(req.GuildID), req.Name, req.Filename, req.ContentB64, req.Width, req.Height)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"emoji": emoji})
}

func (handler *Handler) HandleGuildEmojiEdit(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID  Snowflake `json:"guild_id"`
		RawEmoji string    `json:"raw_emoji"`
		Name     string    `json:"name"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	emoji, err := handler.service.ManagerEditEmoji(r.Context(), sess.AccessToken, discordcontrol.EmojiEditSpec{
		GuildID:  uint64(req.GuildID),
		RawEmoji: req.RawEmoji,
		Name:     req.Name,
	})
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"emoji": emoji})
}

func (handler *Handler) HandleGuildEmojiDelete(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID  Snowflake `json:"guild_id"`
		RawEmoji string    `json:"raw_emoji"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := handler.service.ManagerDeleteEmoji(r.Context(), sess.AccessToken, discordcontrol.EmojiDeleteSpec{
		GuildID:  uint64(req.GuildID),
		RawEmoji: req.RawEmoji,
	}); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *Handler) HandleGuildStickerCreate(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID     Snowflake `json:"guild_id"`
		Name        string    `json:"name"`
		Description string    `json:"description"`
		EmojiTag    string    `json:"emoji_tag"`
		Filename    string    `json:"filename"`
		ContentB64  string    `json:"content_b64"`
		Width       int       `json:"width"`
		Height      int       `json:"height"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	sticker, err := handler.service.ManagerCreateSticker(r.Context(), sess.AccessToken, uint64(req.GuildID), req.Name, req.Description, req.EmojiTag, req.Filename, req.ContentB64, req.Width, req.Height)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"sticker": sticker})
}

func (handler *Handler) HandleGuildStickerEdit(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID     Snowflake `json:"guild_id"`
		RawID       string    `json:"raw_id"`
		Name        string    `json:"name"`
		Description *string   `json:"description"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	sticker, err := handler.service.ManagerEditSticker(r.Context(), sess.AccessToken, discordcontrol.StickerEditSpec{
		GuildID:     uint64(req.GuildID),
		RawID:       req.RawID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"sticker": sticker})
}

func (handler *Handler) HandleGuildStickerDelete(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	var req struct {
		GuildID Snowflake `json:"guild_id"`
		RawID   string    `json:"raw_id"`
	}
	if err := handler.responder.Decode(r, &req); err != nil {
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := handler.service.ManagerDeleteSticker(r.Context(), sess.AccessToken, discordcontrol.StickerDeleteSpec{
		GuildID: uint64(req.GuildID),
		RawID:   req.RawID,
	}); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (handler *Handler) guildIDQuery(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	guildIDRaw := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	guildID, err := strconv.ParseUint(guildIDRaw, 10, 64)
	if err != nil || guildID == 0 {
		handler.responder.Error(w, http.StatusBadRequest, "invalid guild_id")
		return 0, false
	}
	return guildID, true
}
