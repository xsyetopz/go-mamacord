package adminapi

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/guildconfig"
	pluginhostlua "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

var ErrGuildNotAccessible = errors.New("guild is not accessible to this user")

func (s Service) pluginSection(pluginID, name string, cfg guildconfig.PluginConfig) PluginSection {
	return PluginSection{
		ID:            pluginID,
		Name:          name,
		Enabled:       cfg.Enabled,
		GlobalEnabled: s.globalModuleEnabled(pluginID),
		Commands:      commandStates(pluginID, cfg.Commands),
	}
}

func (s Service) globalModuleEnabled(moduleID string) bool {
	if s.ModuleAdmin == nil {
		return false
	}
	for _, info := range s.ModuleAdmin.Infos() {
		if info.ID == moduleID {
			return info.Enabled
		}
	}
	return false
}

func commandStates(pluginID string, commandMap map[string]bool) []PluginCommandState {
	commands := guildconfig.Commands(pluginID)
	out := make([]PluginCommandState, 0, len(commands))
	for _, command := range commands {
		out = append(out, PluginCommandState{
			ID:      command,
			Label:   command,
			Enabled: commandMap[command],
		})
	}
	return out
}

func (s Service) accessibleGuild(ctx context.Context, accessToken string, guildID uint64) (UserGuildSummary, error) {
	guilds, err := s.UserGuilds(ctx, accessToken)
	if err != nil {
		return UserGuildSummary{}, err
	}
	for _, guild := range guilds {
		if uint64(guild.ID) == guildID {
			return guild, nil
		}
	}
	return UserGuildSummary{}, ErrGuildNotAccessible
}

func (s Service) GuildConfig(ctx context.Context, accessToken string, guildID uint64, pluginID string) (guildconfig.PluginConfig, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return guildconfig.PluginConfig{}, err
	}
	return guildconfig.Load(ctx, s.Store, guildID, pluginID)
}

func (s Service) PutGuildConfig(ctx context.Context, accessToken string, guildID uint64, pluginID string, cfg guildconfig.PluginConfig) (guildconfig.PluginConfig, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return guildconfig.PluginConfig{}, err
	}
	return guildconfig.Save(ctx, s.Store, guildID, pluginID, cfg)
}

func (s Service) GuildWarnings(ctx context.Context, accessToken string, guildID, userID uint64, limit int) ([]WarningInfo, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return nil, err
	}
	if s.Store == nil || s.Store.Warnings() == nil {
		return nil, errors.New("warnings store unavailable")
	}
	items, err := s.Store.Warnings().ListWarnings(ctx, guildID, userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]WarningInfo, 0, len(items))
	for _, item := range items {
		out = append(out, WarningInfo{
			ID:          item.ID,
			UserID:      Snowflake(item.UserID),
			ModeratorID: Snowflake(item.ModeratorID),
			Reason:      item.Reason,
			CreatedAt:   item.CreatedAt.UTC().Format(time.RFC3339),
		})
	}
	return out, nil
}

func (s Service) CreateWarning(ctx context.Context, accessToken string, guildID, actorID, targetID uint64, reason string) (map[string]any, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return nil, err
	}
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return nil, errors.New("reason is required")
	}

	cfg, err := guildconfig.Load(ctx, s.Store, guildID, "moderation")
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled || !cfg.Commands["warn"] {
		return nil, errors.New("moderation warn is disabled in this server")
	}
	if s.Store == nil || s.Store.Warnings() == nil || s.Store.Audit() == nil {
		return nil, errors.New("moderation storage unavailable")
	}

	count, err := s.Store.Warnings().CountWarnings(ctx, guildID, targetID)
	if err != nil {
		return nil, err
	}
	if count >= cfg.WarningLimit {
		return nil, errors.New("warning limit reached for this member")
	}

	now := time.Now().UTC()
	if err := s.Store.Warnings().CreateWarning(ctx, store.Warning{
		ID:          warningID(guildID, targetID, now),
		GuildID:     guildID,
		UserID:      targetID,
		ModeratorID: actorID,
		Reason:      reason,
		CreatedAt:   now,
	}); err != nil {
		return nil, err
	}
	if err := s.Store.Audit().Append(ctx, store.AuditEntry{
		GuildID:    &guildID,
		ActorID:    &actorID,
		Action:     "warn.create",
		TargetType: ptrTargetType(store.TargetTypeUser),
		TargetID:   &targetID,
		CreatedAt:  now,
		MetaJSON:   "{}",
	}); err != nil {
		return nil, err
	}

	timeoutMinutes := 0
	timeoutFailed := false
	if count+1 >= cfg.TimeoutThreshold {
		untilUnix := now.Add(time.Duration(cfg.TimeoutMinutes) * time.Minute).Unix()
		if s.TimeoutMember == nil {
			timeoutFailed = true
		} else if err := s.TimeoutMember(ctx, guildID, targetID, untilUnix); err != nil {
			timeoutFailed = true
		} else {
			timeoutMinutes = cfg.TimeoutMinutes
			_ = s.Store.Audit().Append(ctx, store.AuditEntry{
				GuildID:    &guildID,
				ActorID:    &actorID,
				Action:     "warn.timeout",
				TargetType: ptrTargetType(store.TargetTypeUser),
				TargetID:   &targetID,
				CreatedAt:  now,
				MetaJSON:   `{"until":` + strconv.FormatInt(untilUnix, 10) + `}`,
			})
		}
	}

	return map[string]any{
		"warning_count":     count + 1,
		"timeout_minutes":   timeoutMinutes,
		"timeout_failed":    timeoutFailed,
		"warning_limit":     cfg.WarningLimit,
		"timeout_threshold": cfg.TimeoutThreshold,
	}, nil
}

func (s Service) DeleteWarning(ctx context.Context, accessToken string, guildID, actorID uint64, warningID string) error {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return err
	}
	cfg, err := guildconfig.Load(ctx, s.Store, guildID, "moderation")
	if err != nil {
		return err
	}
	if !cfg.Enabled || !cfg.Commands["unwarn"] {
		return errors.New("moderation unwarn is disabled in this server")
	}
	if s.Store == nil || s.Store.Warnings() == nil || s.Store.Audit() == nil {
		return errors.New("moderation storage unavailable")
	}
	if err := s.Store.Warnings().DeleteWarning(ctx, strings.TrimSpace(warningID)); err != nil {
		return err
	}
	return s.Store.Audit().Append(ctx, store.AuditEntry{
		GuildID:   &guildID,
		ActorID:   &actorID,
		Action:    "warn.delete",
		CreatedAt: time.Now().UTC(),
		MetaJSON:  "{}",
	})
}

func (s Service) GuildChannels(ctx context.Context, accessToken string, guildID uint64) ([]GuildChannelInfo, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return nil, err
	}
	return s.guildChannels(ctx, guildID)
}

func (s Service) GuildRoles(ctx context.Context, accessToken string, guildID uint64) ([]GuildRoleInfo, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return nil, err
	}
	return s.guildRoles(ctx, guildID)
}

func (s Service) GuildMembers(ctx context.Context, accessToken string, guildID uint64, query string, limit int) ([]GuildMemberInfo, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return nil, err
	}
	if s.SearchGuildMembers == nil {
		return nil, errors.New("member search unavailable")
	}
	return s.SearchGuildMembers(ctx, guildID, query, limit)
}

func (s Service) GuildEmojis(ctx context.Context, accessToken string, guildID uint64) ([]GuildEmojiInfo, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return nil, err
	}
	return s.guildEmojis(ctx, guildID)
}

func (s Service) GuildStickers(ctx context.Context, accessToken string, guildID uint64) ([]GuildStickerInfo, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return nil, err
	}
	return s.guildStickers(ctx, guildID)
}

func (s Service) ManagerSlowmode(ctx context.Context, accessToken string, guildID, channelID uint64, seconds int) error {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return err
	}
	if s.SetSlowmode == nil {
		return errors.New("slowmode control unavailable")
	}
	return s.SetSlowmode(ctx, channelID, seconds)
}

func (s Service) ManagerNickname(ctx context.Context, accessToken string, guildID, userID uint64, nickname *string) error {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return err
	}
	if s.SetNickname == nil {
		return errors.New("nickname control unavailable")
	}
	return s.SetNickname(ctx, guildID, userID, nickname)
}

func (s Service) ManagerCreateRole(ctx context.Context, accessToken string, spec pluginhostlua.RoleCreateSpec) (pluginhostlua.RoleResult, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, spec.GuildID); err != nil {
		return pluginhostlua.RoleResult{}, err
	}
	if s.CreateRole == nil {
		return pluginhostlua.RoleResult{}, errors.New("role control unavailable")
	}
	return s.CreateRole(ctx, spec)
}

func (s Service) ManagerEditRole(ctx context.Context, accessToken string, spec pluginhostlua.RoleEditSpec) (pluginhostlua.RoleResult, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, spec.GuildID); err != nil {
		return pluginhostlua.RoleResult{}, err
	}
	if s.EditRole == nil {
		return pluginhostlua.RoleResult{}, errors.New("role control unavailable")
	}
	return s.EditRole(ctx, spec)
}

func (s Service) ManagerDeleteRole(ctx context.Context, accessToken string, guildID, roleID uint64) error {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return err
	}
	if s.DeleteRole == nil {
		return errors.New("role control unavailable")
	}
	return s.DeleteRole(ctx, guildID, roleID)
}

func (s Service) ManagerMemberRole(ctx context.Context, accessToken string, add bool, spec pluginhostlua.RoleMemberSpec) error {
	if _, err := s.accessibleGuild(ctx, accessToken, spec.GuildID); err != nil {
		return err
	}
	if add {
		if s.AddRole == nil {
			return errors.New("role control unavailable")
		}
		return s.AddRole(ctx, spec)
	}
	if s.RemoveRole == nil {
		return errors.New("role control unavailable")
	}
	return s.RemoveRole(ctx, spec)
}

func (s Service) ManagerPurge(ctx context.Context, accessToken string, guildID uint64, spec pluginhostlua.PurgeSpec) (int, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return 0, err
	}
	if s.PurgeMessages == nil {
		return 0, errors.New("purge unavailable")
	}
	return s.PurgeMessages(ctx, spec)
}

func (s Service) ManagerCreateEmoji(ctx context.Context, accessToken string, guildID uint64, name, filename, contentB64 string, width, height int) (pluginhostlua.EmojiResult, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return pluginhostlua.EmojiResult{}, err
	}
	if s.CreateEmojiUpload == nil {
		return pluginhostlua.EmojiResult{}, errors.New("emoji control unavailable")
	}
	body, err := decodeBase64File(contentB64)
	if err != nil {
		return pluginhostlua.EmojiResult{}, err
	}
	return s.CreateEmojiUpload(ctx, guildID, name, filename, body, width, height)
}

func (s Service) ManagerEditEmoji(ctx context.Context, accessToken string, spec pluginhostlua.EmojiEditSpec) (pluginhostlua.EmojiResult, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, spec.GuildID); err != nil {
		return pluginhostlua.EmojiResult{}, err
	}
	if s.EditEmoji == nil {
		return pluginhostlua.EmojiResult{}, errors.New("emoji control unavailable")
	}
	return s.EditEmoji(ctx, spec)
}

func (s Service) ManagerDeleteEmoji(ctx context.Context, accessToken string, spec pluginhostlua.EmojiDeleteSpec) error {
	if _, err := s.accessibleGuild(ctx, accessToken, spec.GuildID); err != nil {
		return err
	}
	if s.DeleteEmoji == nil {
		return errors.New("emoji control unavailable")
	}
	return s.DeleteEmoji(ctx, spec)
}

func (s Service) ManagerCreateSticker(ctx context.Context, accessToken string, guildID uint64, name, description, emojiTag, filename, contentB64 string, width, height int) (pluginhostlua.StickerResult, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, guildID); err != nil {
		return pluginhostlua.StickerResult{}, err
	}
	if s.CreateStickerUpload == nil {
		return pluginhostlua.StickerResult{}, errors.New("sticker control unavailable")
	}
	body, err := decodeBase64File(contentB64)
	if err != nil {
		return pluginhostlua.StickerResult{}, err
	}
	return s.CreateStickerUpload(ctx, guildID, name, description, emojiTag, filename, body, width, height)
}

func (s Service) ManagerEditSticker(ctx context.Context, accessToken string, spec pluginhostlua.StickerEditSpec) (pluginhostlua.StickerResult, error) {
	if _, err := s.accessibleGuild(ctx, accessToken, spec.GuildID); err != nil {
		return pluginhostlua.StickerResult{}, err
	}
	if s.EditSticker == nil {
		return pluginhostlua.StickerResult{}, errors.New("sticker control unavailable")
	}
	return s.EditSticker(ctx, spec)
}

func (s Service) ManagerDeleteSticker(ctx context.Context, accessToken string, spec pluginhostlua.StickerDeleteSpec) error {
	if _, err := s.accessibleGuild(ctx, accessToken, spec.GuildID); err != nil {
		return err
	}
	if s.DeleteSticker == nil {
		return errors.New("sticker control unavailable")
	}
	return s.DeleteSticker(ctx, spec)
}

func (s Service) guildChannels(ctx context.Context, guildID uint64) ([]GuildChannelInfo, error) {
	if s.ListGuildChannels == nil {
		return nil, errors.New("channel listing unavailable")
	}
	return s.ListGuildChannels(ctx, guildID)
}

func (s Service) guildRoles(ctx context.Context, guildID uint64) ([]GuildRoleInfo, error) {
	if s.ListGuildRoles == nil {
		return nil, errors.New("role listing unavailable")
	}
	return s.ListGuildRoles(ctx, guildID)
}

func (s Service) guildEmojis(ctx context.Context, guildID uint64) ([]GuildEmojiInfo, error) {
	if s.ListGuildEmojis == nil {
		return nil, errors.New("emoji listing unavailable")
	}
	return s.ListGuildEmojis(ctx, guildID)
}

func (s Service) guildStickers(ctx context.Context, guildID uint64) ([]GuildStickerInfo, error) {
	if s.ListGuildStickers == nil {
		return nil, errors.New("sticker listing unavailable")
	}
	return s.ListGuildStickers(ctx, guildID)
}

func decodeBase64File(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, errors.New("file content is required")
	}
	if idx := strings.Index(raw, ","); idx >= 0 {
		raw = raw[idx+1:]
	}
	body, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, errors.New("invalid file encoding")
	}
	return body, nil
}

func ptrTargetType(value store.TargetType) *store.TargetType {
	return &value
}

func warningID(guildID, userID uint64, now time.Time) string {
	return fmt.Sprintf("warn_%d_%d_%d", guildID, userID, now.UnixNano())
}
