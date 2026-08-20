package guilds

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/url"
	"slices"
	"strconv"
	"strings"

	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
	"github.com/xsyetopz/go-mamacord/internal/guildconfig"
)

func (s *Service) UserGuilds(ctx context.Context, accessToken string) ([]UserGuildSummary, error) {
	if s.OAuth == nil {
		return nil, errors.New("oauth client is not configured")
	}
	guilds, err := s.fetchGuildsCached(ctx, accessToken)
	if err != nil {
		return nil, err
	}

	// Prefer an explicit "bot has guild" check (REST) so install-state updates
	// even when the gateway cache isn't available yet.
	knownInstalled := toUint64Set(s.KnownGuildIDs)
	installedCache := map[uint64]bool{}

	out := make([]UserGuildSummary, 0, len(guilds))
	for _, guild := range guilds {
		id, err := parseDiscordID(guild.ID)
		if err != nil {
			continue
		}
		canManage := guild.Owner || hasManageGuildPermissions(string(guild.Permissions))
		if !canManage {
			continue
		}

		botInstalled := knownInstalled[id]
		if s.BotHasGuild != nil {
			if cached, ok := installedCache[id]; ok {
				botInstalled = cached
			} else {
				installed, installErr := s.BotHasGuild(ctx, id)
				if installErr == nil {
					botInstalled = installed
				}
				installedCache[id] = botInstalled
			}
		}

		out = append(out, UserGuildSummary{
			ID:           Snowflake(id),
			Name:         strings.TrimSpace(guild.Name),
			IconURL:      guildIconURL(guild),
			Owner:        guild.Owner,
			CanManage:    canManage,
			BotInstalled: botInstalled,
		})
	}
	slices.SortFunc(out, func(a, b UserGuildSummary) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out, nil
}

func (s *Service) GuildDashboard(ctx context.Context, accessToken string, guildID uint64) (GuildDashboardResponse, error) {
	guilds, err := s.UserGuilds(ctx, accessToken)
	if err != nil {
		return GuildDashboardResponse{}, err
	}
	target := Snowflake(guildID)
	var guild UserGuildSummary
	found := false
	for _, item := range guilds {
		if item.ID == target {
			guild = item
			found = true
			break
		}
	}
	if !found {
		return GuildDashboardResponse{}, ErrGuildNotAccessible
	}
	installURL := fmt.Sprintf("/api/install/start?guild_id=%d", guildID)

	managerCfg, err := guildconfig.Load(ctx, s.PluginKV, guildID, "manager")
	if err != nil {
		return GuildDashboardResponse{}, err
	}
	moderationCfg, err := guildconfig.Load(ctx, s.PluginKV, guildID, "moderation")
	if err != nil {
		return GuildDashboardResponse{}, err
	}
	funCfg, err := guildconfig.Load(ctx, s.PluginKV, guildID, "fun")
	if err != nil {
		return GuildDashboardResponse{}, err
	}
	infoCfg, err := guildconfig.Load(ctx, s.PluginKV, guildID, "info")
	if err != nil {
		return GuildDashboardResponse{}, err
	}
	wellnessCfg, err := guildconfig.Load(ctx, s.PluginKV, guildID, "wellness")
	if err != nil {
		return GuildDashboardResponse{}, err
	}

	channels, _ := s.guildChannels(ctx, guildID)
	roles, _ := s.guildRoles(ctx, guildID)
	emojis, _ := s.guildEmojis(ctx, guildID)
	stickers, _ := s.guildStickers(ctx, guildID)
	return GuildDashboardResponse{
		Guild:      guild,
		InstallURL: installURL,
		SetupChecks: []SetupCheck{
			{
				ID:      "user_access",
				Label:   "You can manage this server",
				OK:      guild.CanManage,
				Message: boolMessage(guild.CanManage, "You have permission to manage this server.", "You do not have permission to manage this server."),
			},
			{
				ID:      "bot_installed",
				Label:   "Bot installed",
				OK:      guild.BotInstalled,
				Message: boolMessage(guild.BotInstalled, "The bot is already in this server.", "Add the bot to this server to continue."),
			},
		},
		Manager: ManagerSection{
			PluginSection: s.pluginSection("manager", "Manager", managerCfg),
			ChannelCount:  len(channels),
			RoleCount:     len(roles),
			EmojiCount:    len(emojis),
			StickerCount:  len(stickers),
		},
		Moderation: ModerationSection{
			PluginSection:    s.pluginSection("moderation", "Moderation", moderationCfg),
			WarningLimit:     moderationCfg.WarningLimit,
			TimeoutThreshold: moderationCfg.TimeoutThreshold,
			TimeoutMinutes:   moderationCfg.TimeoutMinutes,
		},
		Fun:  s.pluginSection("fun", "Fun", funCfg),
		Info: s.pluginSection("info", "Info", infoCfg),
		Wellness: WellnessSection{
			PluginSection:            s.pluginSection("wellness", "Wellness", wellnessCfg),
			AllowChannelReminders:    wellnessCfg.AllowChannelReminders,
			DefaultReminderChannelID: Snowflake(wellnessCfg.DefaultReminderChannelID),
		},
	}, nil
}

func (s *Service) InstallURL(guildID uint64, baseURL string) (string, error) {
	_ = baseURL
	clientID := strings.TrimSpace(s.ClientID)
	if clientID == "" {
		return "", errors.New("dashboard client id is not configured")
	}

	values := url.Values{}
	values.Set("client_id", clientID)
	values.Set("scope", "bot applications.commands")
	values.Set("permissions", "8")
	values.Set("guild_id", fmt.Sprintf("%d", guildID))
	values.Set("disable_guild_select", "true")
	return "https://discord.com/oauth2/authorize?" + values.Encode(), nil
}

func (s *Service) InstallURLAnyGuild(baseURL string) (string, error) {
	_ = baseURL
	clientID := strings.TrimSpace(s.ClientID)
	if clientID == "" {
		return "", errors.New("dashboard client id is not configured")
	}

	values := url.Values{}
	values.Set("client_id", clientID)
	values.Set("scope", "bot applications.commands")
	values.Set("permissions", "8")
	return "https://discord.com/oauth2/authorize?" + values.Encode(), nil
}

func toUint64Set(fn func() []uint64) map[uint64]bool {
	out := map[uint64]bool{}
	if fn == nil {
		return out
	}
	for _, id := range fn() {
		out[id] = true
	}
	return out
}

func parseDiscordID(raw string) (uint64, error) {
	return strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
}

func hasManageGuildPermissions(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	permission, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return false
	}
	administrator := big.NewInt(0x8)
	manageGuild := big.NewInt(0x20)
	return new(big.Int).And(permission, administrator).Cmp(big.NewInt(0)) != 0 ||
		new(big.Int).And(permission, manageGuild).Cmp(big.NewInt(0)) != 0
}

func guildIconURL(guild adminauth.OAuthGuild) string {
	id := strings.TrimSpace(guild.ID)
	icon := strings.TrimSpace(guild.Icon)
	if id == "" || icon == "" {
		return ""
	}
	return "https://cdn.discordapp.com/icons/" + id + "/" + icon + ".png"
}

func boolMessage(value bool, okMessage, noMessage string) string {
	if value {
		return okMessage
	}
	return noMessage
}
