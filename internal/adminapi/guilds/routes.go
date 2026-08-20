package guilds

import (
	"context"
	"errors"
	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
	"github.com/xsyetopz/go-mamacord/internal/guildconfig"
	discordcontrol "github.com/xsyetopz/go-mamacord/internal/runtime/discord/control"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

type Application interface {
	UserGuilds(context.Context, string) ([]UserGuildSummary, error)
	GuildDashboard(context.Context, string, uint64) (GuildDashboardResponse, error)
	InstallURL(uint64, string) (string, error)
	InstallURLAnyGuild(string) (string, error)
	PutGuildConfig(context.Context, string, uint64, string, guildconfig.PluginConfig) (guildconfig.PluginConfig, error)
	GuildChannels(context.Context, string, uint64) ([]GuildChannelInfo, error)
	GuildRoles(context.Context, string, uint64) ([]GuildRoleInfo, error)
	GuildMembers(context.Context, string, uint64, string, int) ([]GuildMemberInfo, error)
	GuildEmojis(context.Context, string, uint64) ([]GuildEmojiInfo, error)
	GuildStickers(context.Context, string, uint64) ([]GuildStickerInfo, error)
	GuildWarnings(context.Context, string, uint64, uint64, int) ([]WarningInfo, error)
	CreateWarning(context.Context, string, uint64, uint64, uint64, string) (map[string]any, error)
	DeleteWarning(context.Context, string, uint64, uint64, string) error
	ManagerSlowmode(context.Context, string, uint64, uint64, int) error
	ManagerNickname(context.Context, string, uint64, uint64, *string) error
	ManagerCreateRole(context.Context, string, discordcontrol.RoleCreateSpec) (discordcontrol.RoleResult, error)
	ManagerEditRole(context.Context, string, discordcontrol.RoleEditSpec) (discordcontrol.RoleResult, error)
	ManagerDeleteRole(context.Context, string, uint64, uint64) error
	ManagerMemberRole(context.Context, string, bool, discordcontrol.RoleMemberSpec) error
	ManagerPurge(context.Context, string, uint64, discordcontrol.PurgeSpec) (int, error)
	ManagerCreateEmoji(context.Context, string, uint64, string, string, string, int, int) (discordcontrol.EmojiResult, error)
	ManagerEditEmoji(context.Context, string, discordcontrol.EmojiEditSpec) (discordcontrol.EmojiResult, error)
	ManagerDeleteEmoji(context.Context, string, discordcontrol.EmojiDeleteSpec) error
	ManagerCreateSticker(context.Context, string, uint64, string, string, string, string, string, int, int) (discordcontrol.StickerResult, error)
	ManagerEditSticker(context.Context, string, discordcontrol.StickerEditSpec) (discordcontrol.StickerResult, error)
	ManagerDeleteSticker(context.Context, string, discordcontrol.StickerDeleteSpec) error
}

type Responder interface {
	Decode(*http.Request, any) error
	JSON(http.ResponseWriter, int, any)
	Error(http.ResponseWriter, int, string)
	ServiceError(http.ResponseWriter, int, error)
}

type Options struct {
	Service          Application
	Logger           *slog.Logger
	Responder        Responder
	DashboardBaseURL func(*http.Request) string
	RequestBaseURL   func(*http.Request) string
}

type Handler struct {
	service          Application
	logger           *slog.Logger
	responder        Responder
	dashboardBaseURL func(*http.Request) string
	requestBaseURL   func(*http.Request) string
}

func New(options Options) *Handler {
	return &Handler{
		service: options.Service, logger: options.Logger, responder: options.Responder,
		dashboardBaseURL: options.DashboardBaseURL, requestBaseURL: options.RequestBaseURL,
	}
}

func (handler *Handler) HandleGuilds(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	guilds, err := handler.service.UserGuilds(r.Context(), sess.AccessToken)
	if err != nil {
		handler.responder.ServiceError(w, http.StatusBadGateway, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, struct {
		Guilds []UserGuildSummary `json:"guilds"`
	}{Guilds: guilds})
}

func (handler *Handler) HandleGuildDashboard(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	guildIDRaw := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	guildID, err := strconv.ParseUint(guildIDRaw, 10, 64)
	if err != nil || guildID == 0 {
		handler.responder.Error(w, http.StatusBadRequest, "invalid guild_id")
		return
	}
	dashboard, err := handler.service.GuildDashboard(r.Context(), sess.AccessToken, guildID)
	if err != nil {
		if errors.Is(err, ErrGuildNotAccessible) {
			handler.responder.Error(w, http.StatusForbidden, err.Error())
			return
		}
		handler.responder.ServiceError(w, http.StatusBadRequest, err)
		return
	}
	handler.responder.JSON(w, http.StatusOK, dashboard)
}

func (handler *Handler) HandleInstallStart(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	guildIDRaw := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	var (
		url string
		err error
	)
	base := handler.requestBaseURL(r)
	if guildIDRaw == "" {
		url, err = handler.service.InstallURLAnyGuild(base)
	} else {
		guildID, parseErr := strconv.ParseUint(guildIDRaw, 10, 64)
		if parseErr != nil || guildID == 0 {
			handler.responder.Error(w, http.StatusBadRequest, "invalid guild_id")
			return
		}
		url, err = handler.service.InstallURL(guildID, base)
	}
	if err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	attrs := []any{slog.Uint64("actor_id", sess.UserID)}
	if guildIDRaw != "" {
		if guildID, parseErr := strconv.ParseUint(guildIDRaw, 10, 64); parseErr == nil {
			attrs = append(attrs, slog.Uint64("guild_id", guildID))
		}
	}
	handler.logger.Info("bot install started", attrs...)
	http.Redirect(w, r, url, http.StatusFound)
}

func (handler *Handler) HandleInstallCallback(w http.ResponseWriter, r *http.Request, sess adminauth.Session) {
	guildIDRaw := strings.TrimSpace(r.URL.Query().Get("guild_id"))
	guildID, err := strconv.ParseUint(guildIDRaw, 10, 64)
	if err != nil || guildID == 0 {
		base := handler.dashboardBaseURL(r)
		http.Redirect(w, r, strings.TrimRight(base, "/")+"/#/servers", http.StatusFound)
		return
	}
	if _, err := handler.service.GuildDashboard(r.Context(), sess.AccessToken, guildID); err != nil {
		handler.responder.Error(w, http.StatusBadRequest, err.Error())
		return
	}
	base := handler.dashboardBaseURL(r)
	http.Redirect(w, r, strings.TrimRight(base, "/")+"/#/servers/"+guildIDRaw, http.StatusFound)
}
