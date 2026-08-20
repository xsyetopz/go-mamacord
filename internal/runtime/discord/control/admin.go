package control

import (
	"context"
	"errors"
	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	"slices"
	"strings"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/host"
)

type AdminExecutor interface {
	SetSlowmode(context.Context, uint64, int) error
	SetNickname(context.Context, uint64, uint64, *string) error
	TimeoutMember(context.Context, uint64, uint64, time.Time) error
	CreateRole(context.Context, RoleCreateSpec) (RoleResult, error)
	EditRole(context.Context, RoleEditSpec) (RoleResult, error)
	DeleteRole(context.Context, uint64, uint64) error
	AddRole(context.Context, RoleMemberSpec) error
	RemoveRole(context.Context, RoleMemberSpec) error
	PurgeMessages(context.Context, PurgeSpec) (int, error)
	CreateEmojiUpload(context.Context, uint64, string, string, []byte, int, int) (EmojiResult, error)
	EditEmoji(context.Context, EmojiEditSpec) (EmojiResult, error)
	DeleteEmoji(context.Context, EmojiDeleteSpec) error
	CreateStickerUpload(context.Context, uint64, string, string, string, string, []byte, int, int) (StickerResult, error)
	EditSticker(context.Context, StickerEditSpec) (StickerResult, error)
	DeleteSticker(context.Context, StickerDeleteSpec) error
}

type AdminOptions struct {
	Client           *bot.Client
	PluginHost       *pluginhost.Host
	ModuleInfos      func() []moduleapi.Info
	ReloadModules    func(context.Context) error
	SetModuleEnabled func(context.Context, string, bool, uint64) error
	ResetModule      func(context.Context, string) error
	Executor         AdminExecutor
}

type Admin struct {
	client           *bot.Client
	pluginHost       *pluginhost.Host
	moduleInfos      func() []moduleapi.Info
	reloadModules    func(context.Context) error
	setModuleEnabled func(context.Context, string, bool, uint64) error
	resetModule      func(context.Context, string) error
	executor         AdminExecutor
}

func NewAdmin(options AdminOptions) *Admin {
	return &Admin{
		client: options.Client, pluginHost: options.PluginHost, moduleInfos: options.ModuleInfos,
		reloadModules: options.ReloadModules, setModuleEnabled: options.SetModuleEnabled,
		resetModule: options.ResetModule, executor: options.Executor,
	}
}

func (admin *Admin) PluginAdmin() commandruntime.PluginAdmin { return pluginAdmin{admin: admin} }
func (admin *Admin) ModuleAdmin() moduleapi.Admin            { return moduleAdmin{admin: admin} }

type pluginAdmin struct{ admin *Admin }

func (plugin pluginAdmin) Configured() bool {
	return plugin.admin != nil && plugin.admin.pluginHost != nil
}
func (plugin pluginAdmin) Infos() []pluginhost.PluginInfo {
	if !plugin.Configured() {
		return nil
	}
	return append([]pluginhost.PluginInfo(nil), plugin.admin.pluginHost.Infos()...)
}
func (plugin pluginAdmin) Reload(ctx context.Context) error {
	if !plugin.Configured() || plugin.admin.reloadModules == nil {
		return errors.New("plugins not configured")
	}
	return plugin.admin.reloadModules(ctx)
}

type moduleAdmin struct{ admin *Admin }

func (module moduleAdmin) Configured() bool { return module.admin != nil }
func (module moduleAdmin) Infos() []moduleapi.Info {
	if module.admin == nil || module.admin.moduleInfos == nil {
		return nil
	}
	return module.admin.moduleInfos()
}
func (module moduleAdmin) Reload(ctx context.Context) error {
	if module.admin == nil || module.admin.reloadModules == nil {
		return errors.New("modules not configured")
	}
	return module.admin.reloadModules(ctx)
}
func (module moduleAdmin) SetEnabled(ctx context.Context, moduleID string, enabled bool, actorID uint64) error {
	if module.admin == nil || module.admin.setModuleEnabled == nil {
		return errors.New("modules not configured")
	}
	return module.admin.setModuleEnabled(ctx, moduleID, enabled, actorID)
}
func (module moduleAdmin) Reset(ctx context.Context, moduleID string) error {
	if module.admin == nil || module.admin.resetModule == nil {
		return errors.New("modules not configured")
	}
	return module.admin.resetModule(ctx, moduleID)
}

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

func (admin *Admin) ModuleInfos() []moduleapi.Info {
	if admin == nil {
		return nil
	}
	return admin.moduleInfos()
}

func (admin *Admin) PluginInfos() []pluginhost.PluginInfo {
	if admin == nil || admin.pluginHost == nil {
		return nil
	}
	return admin.pluginHost.Infos()
}

func (admin *Admin) ReloadModules(ctx context.Context) error {
	if admin == nil {
		return nil
	}
	return admin.reloadModules(ctx)
}

func (admin *Admin) SetModuleEnabled(ctx context.Context, moduleID string, enabled bool, actorID uint64) error {
	if admin == nil {
		return nil
	}
	return admin.setModuleEnabled(ctx, moduleID, enabled, actorID)
}

func (admin *Admin) ResetModule(ctx context.Context, moduleID string) error {
	if admin == nil {
		return nil
	}
	return admin.resetModule(ctx, moduleID)
}

func (admin *Admin) KnownGuildIDs() []uint64 {
	if admin == nil || admin.client == nil {
		return nil
	}
	guilds := make([]uint64, 0, admin.client.Caches.GuildsLen())
	for guild := range admin.client.Caches.Guilds() {
		guilds = append(guilds, uint64(guild.ID))
	}
	slices.Sort(guilds)
	return guilds
}

func (admin *Admin) HasGuild(ctx context.Context, guildID uint64) (bool, error) {
	if admin == nil || admin.client == nil {
		return false, errors.New("discord client unavailable")
	}

	_, err := admin.client.Rest.GetGuild(snowflake.ID(guildID), false, rest.WithCtx(ctx))
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

func (admin *Admin) ListGuildChannels(ctx context.Context, guildID uint64) ([]GuildChannelInfo, error) {
	if admin == nil || admin.client == nil {
		return nil, errors.New("discord client unavailable")
	}
	channels, err := admin.client.Rest.GetGuildChannels(snowflake.ID(guildID), rest.WithCtx(ctx))
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

func (admin *Admin) ListGuildRoles(ctx context.Context, guildID uint64) ([]GuildRoleInfo, error) {
	if admin == nil || admin.client == nil {
		return nil, errors.New("discord client unavailable")
	}
	roles, err := admin.client.Rest.GetRoles(snowflake.ID(guildID), rest.WithCtx(ctx))
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

func (admin *Admin) SearchGuildMembers(ctx context.Context, guildID uint64, query string, limit int) ([]GuildMemberInfo, error) {
	if admin == nil || admin.client == nil {
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
		members, err = admin.client.Rest.GetMembers(snowflake.ID(guildID), limit, 0, rest.WithCtx(ctx))
	} else {
		members, err = admin.client.Rest.SearchMembers(snowflake.ID(guildID), query, limit, rest.WithCtx(ctx))
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

func (admin *Admin) ListGuildEmojis(ctx context.Context, guildID uint64) ([]GuildEmojiInfo, error) {
	if admin == nil || admin.client == nil {
		return nil, errors.New("discord client unavailable")
	}
	emojis, err := admin.client.Rest.GetEmojis(snowflake.ID(guildID), rest.WithCtx(ctx))
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

func (admin *Admin) ListGuildStickers(ctx context.Context, guildID uint64) ([]GuildStickerInfo, error) {
	if admin == nil || admin.client == nil {
		return nil, errors.New("discord client unavailable")
	}
	stickers, err := admin.client.Rest.GetStickers(snowflake.ID(guildID), rest.WithCtx(ctx))
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

func (admin *Admin) SetSlowmode(ctx context.Context, channelID uint64, seconds int) error {
	return admin.executorService().SetSlowmode(ctx, channelID, seconds)
}

func (admin *Admin) SetNickname(ctx context.Context, guildID, userID uint64, nickname *string) error {
	return admin.executorService().SetNickname(ctx, guildID, userID, nickname)
}

func (admin *Admin) TimeoutMember(ctx context.Context, guildID, userID uint64, untilUnix int64) error {
	return admin.executorService().TimeoutMember(ctx, guildID, userID, time.Unix(untilUnix, 0).UTC())
}

func (admin *Admin) CreateRole(ctx context.Context, spec RoleCreateSpec) (RoleResult, error) {
	return admin.executorService().CreateRole(ctx, spec)
}

func (admin *Admin) EditRole(ctx context.Context, spec RoleEditSpec) (RoleResult, error) {
	return admin.executorService().EditRole(ctx, spec)
}

func (admin *Admin) DeleteRole(ctx context.Context, guildID, roleID uint64) error {
	return admin.executorService().DeleteRole(ctx, guildID, roleID)
}

func (admin *Admin) AddRole(ctx context.Context, spec RoleMemberSpec) error {
	return admin.executorService().AddRole(ctx, spec)
}

func (admin *Admin) RemoveRole(ctx context.Context, spec RoleMemberSpec) error {
	return admin.executorService().RemoveRole(ctx, spec)
}

func (admin *Admin) PurgeMessages(ctx context.Context, spec PurgeSpec) (int, error) {
	return admin.executorService().PurgeMessages(ctx, spec)
}

func (admin *Admin) CreateEmojiUpload(ctx context.Context, guildID uint64, name, filename string, body []byte, width, height int) (EmojiResult, error) {
	return admin.executorService().CreateEmojiUpload(ctx, guildID, name, filename, body, width, height)
}

func (admin *Admin) EditEmoji(ctx context.Context, spec EmojiEditSpec) (EmojiResult, error) {
	return admin.executorService().EditEmoji(ctx, spec)
}

func (admin *Admin) DeleteEmoji(ctx context.Context, spec EmojiDeleteSpec) error {
	return admin.executorService().DeleteEmoji(ctx, spec)
}

func (admin *Admin) CreateStickerUpload(
	ctx context.Context,
	guildID uint64,
	name, description, emojiTag, filename string,
	body []byte,
	width, height int,
) (StickerResult, error) {
	return admin.executorService().CreateStickerUpload(ctx, guildID, name, description, emojiTag, filename, body, width, height)
}

func (admin *Admin) EditSticker(ctx context.Context, spec StickerEditSpec) (StickerResult, error) {
	return admin.executorService().EditSticker(ctx, spec)
}

func (admin *Admin) DeleteSticker(ctx context.Context, spec StickerDeleteSpec) error {
	return admin.executorService().DeleteSticker(ctx, spec)
}

func (admin *Admin) executorService() AdminExecutor { return admin.executor }

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

var _ commandruntime.PluginAdmin = pluginAdmin{}
var _ moduleapi.Admin = moduleAdmin{}
