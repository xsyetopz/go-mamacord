package adminapi

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/guildconfig"
	pluginhostlua "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

func (s *Server) handleGuildConfig(w http.ResponseWriter, r *http.Request, sess session) {
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
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	cfg, err := s.svc.PutGuildConfig(r.Context(), sess.AccessToken, uint64(req.GuildID), strings.TrimSpace(req.PluginID), guildconfig.PluginConfig{
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
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg})
}

func (s *Server) handleGuildChannels(w http.ResponseWriter, r *http.Request, sess session) {
	guildID, ok := guildIDQuery(w, r)
	if !ok {
		return
	}
	items, err := s.svc.GuildChannels(r.Context(), sess.AccessToken, guildID)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"channels": items})
}

func (s *Server) handleGuildRoles(w http.ResponseWriter, r *http.Request, sess session) {
	guildID, ok := guildIDQuery(w, r)
	if !ok {
		return
	}
	items, err := s.svc.GuildRoles(r.Context(), sess.AccessToken, guildID)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"roles": items})
}

func (s *Server) handleGuildMembers(w http.ResponseWriter, r *http.Request, sess session) {
	guildID, ok := guildIDQuery(w, r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	items, err := s.svc.GuildMembers(r.Context(), sess.AccessToken, guildID, strings.TrimSpace(r.URL.Query().Get("query")), limit)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"members": items})
}

func (s *Server) handleGuildEmojis(w http.ResponseWriter, r *http.Request, sess session) {
	guildID, ok := guildIDQuery(w, r)
	if !ok {
		return
	}
	items, err := s.svc.GuildEmojis(r.Context(), sess.AccessToken, guildID)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"emojis": items})
}

func (s *Server) handleGuildStickers(w http.ResponseWriter, r *http.Request, sess session) {
	guildID, ok := guildIDQuery(w, r)
	if !ok {
		return
	}
	items, err := s.svc.GuildStickers(r.Context(), sess.AccessToken, guildID)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"stickers": items})
}

func (s *Server) handleGuildWarnings(w http.ResponseWriter, r *http.Request, sess session) {
	guildID, ok := guildIDQuery(w, r)
	if !ok {
		return
	}
	userID, err := strconv.ParseUint(strings.TrimSpace(r.URL.Query().Get("user_id")), 10, 64)
	if err != nil || userID == 0 {
		writeError(w, http.StatusBadRequest, "invalid user_id")
		return
	}
	limit, _ := strconv.Atoi(strings.TrimSpace(r.URL.Query().Get("limit")))
	if limit <= 0 {
		limit = 25
	}
	items, err := s.svc.GuildWarnings(r.Context(), sess.AccessToken, guildID, userID, limit)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"warnings": items})
}

func (s *Server) handleGuildWarn(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID Snowflake `json:"guild_id"`
		UserID  Snowflake `json:"user_id"`
		Reason  string    `json:"reason"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	result, err := s.svc.CreateWarning(r.Context(), sess.AccessToken, uint64(req.GuildID), sess.UserID, uint64(req.UserID), req.Reason)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleGuildUnwarn(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID   Snowflake `json:"guild_id"`
		WarningID string    `json:"warning_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.DeleteWarning(r.Context(), sess.AccessToken, uint64(req.GuildID), sess.UserID, req.WarningID); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGuildSlowmode(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID   Snowflake `json:"guild_id"`
		ChannelID Snowflake `json:"channel_id"`
		Seconds   int       `json:"seconds"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.ManagerSlowmode(r.Context(), sess.AccessToken, uint64(req.GuildID), uint64(req.ChannelID), req.Seconds); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGuildNickname(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID  Snowflake `json:"guild_id"`
		UserID   Snowflake `json:"user_id"`
		Nickname string    `json:"nickname"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	var nickname *string
	if strings.TrimSpace(req.Nickname) != "" {
		value := strings.TrimSpace(req.Nickname)
		nickname = &value
	}
	if err := s.svc.ManagerNickname(r.Context(), sess.AccessToken, uint64(req.GuildID), uint64(req.UserID), nickname); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGuildRoleCreate(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID     Snowflake `json:"guild_id"`
		Name        string    `json:"name"`
		Color       *int      `json:"color"`
		Hoist       *bool     `json:"hoist"`
		Mentionable *bool     `json:"mentionable"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	role, err := s.svc.ManagerCreateRole(r.Context(), sess.AccessToken, pluginhostlua.RoleCreateSpec{
		GuildID:     uint64(req.GuildID),
		Name:        req.Name,
		Color:       req.Color,
		Hoist:       req.Hoist,
		Mentionable: req.Mentionable,
	})
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"role": role})
}

func (s *Server) handleGuildRoleEdit(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID     Snowflake `json:"guild_id"`
		RoleID      Snowflake `json:"role_id"`
		Name        *string   `json:"name"`
		Color       *int      `json:"color"`
		Hoist       *bool     `json:"hoist"`
		Mentionable *bool     `json:"mentionable"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	role, err := s.svc.ManagerEditRole(r.Context(), sess.AccessToken, pluginhostlua.RoleEditSpec{
		GuildID:     uint64(req.GuildID),
		RoleID:      uint64(req.RoleID),
		Name:        req.Name,
		Color:       req.Color,
		Hoist:       req.Hoist,
		Mentionable: req.Mentionable,
	})
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"role": role})
}

func (s *Server) handleGuildRoleDelete(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID Snowflake `json:"guild_id"`
		RoleID  Snowflake `json:"role_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.ManagerDeleteRole(r.Context(), sess.AccessToken, uint64(req.GuildID), uint64(req.RoleID)); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGuildRoleMember(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		Add     bool      `json:"add"`
		GuildID Snowflake `json:"guild_id"`
		UserID  Snowflake `json:"user_id"`
		RoleID  Snowflake `json:"role_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.ManagerMemberRole(r.Context(), sess.AccessToken, req.Add, pluginhostlua.RoleMemberSpec{
		GuildID: uint64(req.GuildID),
		UserID:  uint64(req.UserID),
		RoleID:  uint64(req.RoleID),
	}); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGuildPurge(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID   Snowflake `json:"guild_id"`
		ChannelID Snowflake `json:"channel_id"`
		Mode      string    `json:"mode"`
		AnchorRaw string    `json:"anchor_raw"`
		Count     int       `json:"count"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	deleted, err := s.svc.ManagerPurge(r.Context(), sess.AccessToken, uint64(req.GuildID), pluginhostlua.PurgeSpec{
		ChannelID: uint64(req.ChannelID),
		Mode:      req.Mode,
		AnchorRaw: req.AnchorRaw,
		Count:     req.Count,
	})
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"deleted_count": deleted})
}

func (s *Server) handleGuildEmojiCreate(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID    Snowflake `json:"guild_id"`
		Name       string    `json:"name"`
		Filename   string    `json:"filename"`
		ContentB64 string    `json:"content_b64"`
		Width      int       `json:"width"`
		Height     int       `json:"height"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	emoji, err := s.svc.ManagerCreateEmoji(r.Context(), sess.AccessToken, uint64(req.GuildID), req.Name, req.Filename, req.ContentB64, req.Width, req.Height)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"emoji": emoji})
}

func (s *Server) handleGuildEmojiEdit(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID  Snowflake `json:"guild_id"`
		RawEmoji string    `json:"raw_emoji"`
		Name     string    `json:"name"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	emoji, err := s.svc.ManagerEditEmoji(r.Context(), sess.AccessToken, pluginhostlua.EmojiEditSpec{
		GuildID:  uint64(req.GuildID),
		RawEmoji: req.RawEmoji,
		Name:     req.Name,
	})
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"emoji": emoji})
}

func (s *Server) handleGuildEmojiDelete(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID  Snowflake `json:"guild_id"`
		RawEmoji string    `json:"raw_emoji"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.ManagerDeleteEmoji(r.Context(), sess.AccessToken, pluginhostlua.EmojiDeleteSpec{
		GuildID:  uint64(req.GuildID),
		RawEmoji: req.RawEmoji,
	}); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) handleGuildStickerCreate(w http.ResponseWriter, r *http.Request, sess session) {
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
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	sticker, err := s.svc.ManagerCreateSticker(r.Context(), sess.AccessToken, uint64(req.GuildID), req.Name, req.Description, req.EmojiTag, req.Filename, req.ContentB64, req.Width, req.Height)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sticker": sticker})
}

func (s *Server) handleGuildStickerEdit(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID     Snowflake `json:"guild_id"`
		RawID       string    `json:"raw_id"`
		Name        string    `json:"name"`
		Description *string   `json:"description"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	sticker, err := s.svc.ManagerEditSticker(r.Context(), sess.AccessToken, pluginhostlua.StickerEditSpec{
		GuildID:     uint64(req.GuildID),
		RawID:       req.RawID,
		Name:        req.Name,
		Description: req.Description,
	})
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sticker": sticker})
}

func (s *Server) handleGuildStickerDelete(w http.ResponseWriter, r *http.Request, sess session) {
	var req struct {
		GuildID Snowflake `json:"guild_id"`
		RawID   string    `json:"raw_id"`
	}
	if err := decodeJSON(r, &req); err != nil {
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	if err := s.svc.ManagerDeleteSticker(r.Context(), sess.AccessToken, pluginhostlua.StickerDeleteSpec{
		GuildID: uint64(req.GuildID),
		RawID:   req.RawID,
	}); err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeServiceError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func guildIDQuery(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	guildIDRaw := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	guildID, err := strconv.ParseUint(guildIDRaw, 10, 64)
	if err != nil || guildID == 0 {
		writeError(w, http.StatusBadRequest, "invalid guild_id")
		return 0, false
	}
	return guildID, true
}
