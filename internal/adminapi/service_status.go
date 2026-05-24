package adminapi

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/guildconfig"
)

func (s *Service) Status(ctx context.Context) (StatusResponse, error) {
	var devGuildID *Snowflake
	if s.Config.DevGuildID != nil {
		v := Snowflake(*s.Config.DevGuildID)
		devGuildID = &v
	}
	resp := StatusResponse{
		Config: StatusConfig{
			StorageBackend:          string(s.Config.StorageBackend),
			StorageTarget:           storageTargetLabel(s.Config),
			MigrationsDir:           s.Config.Migrations,
			LocalesDir:              s.Config.LocalesDir,
			BundledPluginsDir:       s.Config.BundledPluginsDir,
			UserPluginsDir:          s.Config.UserPluginsDir,
			PermissionsFile:         s.Config.PermissionsFile,
			ModulesFile:             s.Config.ModulesFile,
			TrustedKeysFile:         s.Config.TrustedKeysFile,
			OpsAddr:                 s.Config.OpsAddr,
			AdminAddr:               s.Config.AdminAddr,
			RuntimeRoles:            s.Config.RuntimeRoleStrings(),
			DevGuildID:              devGuildID,
			CommandRegistrationMode: s.Config.CommandRegistrationMode,
			ProdMode:                s.Config.ProdMode,
			AllowUnsignedPlugins:    s.Config.AllowUnsignedPlugins,
		},
		Setup: s.setupResponse(false),
	}
	if s.BuildInfo != nil {
		resp.Build = buildResponse(s.BuildInfo())
	}
	if s.Snapshot != nil {
		resp.Snapshot = snapshotResponse(s.Snapshot())
	}
	keys, err := s.TrustedKeys(ctx)
	if err != nil {
		return StatusResponse{}, err
	}
	resp.Setup.TrustedKeysConfigured = len(keys.FileKeys) > 0 || len(keys.DBKeys) > 0
	return resp, nil
}

func storageTargetLabel(cfg config.Config) string {
	switch cfg.StorageBackend {
	case config.StorageBackendPostgres:
		dsn := strings.TrimSpace(cfg.PostgresDSN)
		if dsn == "" {
			return ""
		}
		parsed, err := url.Parse(dsn)
		if err != nil {
			return "<invalid postgres dsn>"
		}
		if parsed.User != nil {
			username := parsed.User.Username()
			if username != "" {
				parsed.User = url.UserPassword(username, "***")
			} else {
				parsed.User = nil
			}
		}
		return parsed.String()
	default:
		return ""
	}
}

func (s *Service) Setup(ctx context.Context) (SetupResponse, error) {
	resp := s.setupResponse(true)
	keys, err := s.TrustedKeys(ctx)
	if err != nil {
		return SetupResponse{}, err
	}
	resp.TrustedKeysConfigured = len(keys.FileKeys) > 0 || len(keys.DBKeys) > 0
	return resp, nil
}

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

	managerCfg, err := guildconfig.Load(ctx, s.Store, guildID, "manager")
	if err != nil {
		return GuildDashboardResponse{}, err
	}
	moderationCfg, err := guildconfig.Load(ctx, s.Store, guildID, "moderation")
	if err != nil {
		return GuildDashboardResponse{}, err
	}
	funCfg, err := guildconfig.Load(ctx, s.Store, guildID, "fun")
	if err != nil {
		return GuildDashboardResponse{}, err
	}
	infoCfg, err := guildconfig.Load(ctx, s.Store, guildID, "info")
	if err != nil {
		return GuildDashboardResponse{}, err
	}
	wellnessCfg, err := guildconfig.Load(ctx, s.Store, guildID, "wellness")
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
	clientID := strings.TrimSpace(s.Config.DashboardClientID)
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
	clientID := strings.TrimSpace(s.Config.DashboardClientID)
	if clientID == "" {
		return "", errors.New("dashboard client id is not configured")
	}

	values := url.Values{}
	values.Set("client_id", clientID)
	values.Set("scope", "bot applications.commands")
	values.Set("permissions", "8")
	return "https://discord.com/oauth2/authorize?" + values.Encode(), nil
}
