package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/adminapi"
	"github.com/xsyetopz/go-mamacord/internal/buildinfo"
	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	migrate "github.com/xsyetopz/go-mamacord/internal/migration"
	"github.com/xsyetopz/go-mamacord/internal/ops"
	discordplatform "github.com/xsyetopz/go-mamacord/internal/runtime/discord"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	"github.com/xsyetopz/go-mamacord/internal/sqlite"
	sqlitestore "github.com/xsyetopz/go-mamacord/internal/storage/sqlite"
)

type Dependencies struct {
	Logger *slog.Logger
	Config config.Config
}

type App struct {
	logger *slog.Logger
	cfg    config.Config

	store       *sqlitestore.Store
	marketplace *marketplace.Manager
	i18n        i18n.Registry
	bot         *discordplatform.Bot
	ops         *ops.Server
	admin       *adminapi.Server
	metrics     *ops.Metrics

	startedAt        time.Time
	migrationVersion int

	discordStartErr atomic.Pointer[string]
}

func New(deps Dependencies) (*App, error) {
	if deps.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if deps.Config.ProdMode && deps.Config.AllowUnsignedPlugins {
		return nil, errors.New("prod mode requires signed plugins; set MAMACORD_ALLOW_UNSIGNED_PLUGINS=0")
	}

	return &App{
		logger:  deps.Logger,
		cfg:     deps.Config,
		metrics: ops.NewMetrics(),
	}, nil
}

func (a *App) Start(ctx context.Context) error {
	a.startedAt = time.Now()
	if err := a.initStorage(ctx); err != nil {
		return err
	}
	if err := a.validatePluginTrust(ctx); err != nil {
		return err
	}
	if err := a.initI18n(); err != nil {
		return err
	}
	if err := a.initDiscordBot(); err != nil {
		return err
	}
	if err := a.initOpsServer(); err != nil {
		return err
	}
	if err := a.initAdminServer(); err != nil {
		return err
	}
	if a.ops != nil {
		if err := a.ops.Start(); err != nil {
			return err
		}
	}
	if a.admin != nil {
		if err := a.admin.Start(); err != nil {
			return err
		}
	}

	if err := a.bot.Start(ctx); err != nil {
		// Dev ergonomics: keep the admin API up even if Discord rejects our gateway
		// connection (missing intents, bad token, etc). Production should still
		// fail fast so the process restarts and the error is visible.
		if a.cfg.ProdMode {
			return err
		}
		msg := err.Error()
		a.discordStartErr.Store(&msg)
		a.logger.ErrorContext(ctx, "discord bot failed to start; keeping admin API running", slog.String("err", err.Error()))
		<-ctx.Done()
		return ctx.Err()
	}

	<-ctx.Done()
	return ctx.Err()
}

func (a *App) Close() error {
	if a.admin != nil {
		_ = a.admin.Close(context.Background())
	}
	if a.ops != nil {
		_ = a.ops.Close(context.Background())
	}
	if a.bot != nil {
		a.bot.Close(context.Background())
	}
	if a.store != nil {
		return a.store.Close()
	}
	return nil
}

func (a *App) initStorage(ctx context.Context) error {
	if a.store != nil {
		return nil
	}

	runner, err := migrate.New(migrate.Options{
		Dir:       a.cfg.Migrations,
		BackupDir: a.cfg.MigrationBackups,
	})
	if err != nil {
		return err
	}
	status, runErr := runner.UpPath(ctx, a.cfg.SQLitePath)
	if runErr != nil {
		return runErr
	}
	a.migrationVersion = status.CurrentVersion

	db, err := sqlite.Open(ctx, sqlite.Options{
		Path: a.cfg.SQLitePath,
	})
	if err != nil {
		return err
	}

	store, err := sqlitestore.New(db)
	if err != nil {
		_ = db.Close()
		return err
	}

	a.store = store
	return nil
}

func (a *App) validatePluginTrust(ctx context.Context) error {
	if !a.cfg.ProdMode || a.cfg.AllowUnsignedPlugins || a.store == nil {
		return nil
	}

	fileKeys := 0
	path := strings.TrimSpace(a.cfg.TrustedKeysFile)
	if path != "" {
		keys, err := pluginhost.ReadTrustedKeysFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		fileKeys = len(keys)
	}

	signers, err := a.store.TrustedSigners().ListTrustedSigners(ctx)
	if err != nil {
		return err
	}
	if fileKeys == 0 && len(signers) == 0 {
		pathLabel := strings.TrimSpace(path)
		if pathLabel == "" {
			pathLabel = "./config/trusted_keys.json"
		}
		return fmt.Errorf(
			"prod mode requires at least one trusted signer in %s or SQLite; bundled plugins expect a trusted public key file there, and custom plugins should be signed with mamacord gen-signing-key + sign-plugin",
			pathLabel,
		)
	}
	return nil
}

func (a *App) initI18n() error {
	reg, err := i18n.LoadCore(a.cfg.LocalesDir)
	if err != nil {
		return err
	}

	a.i18n = reg
	return nil
}

func (a *App) initDiscordBot() error {
	if a.bot != nil {
		return nil
	}
	if a.store == nil {
		return errors.New("store must be initialized before discord bot")
	}
	if a.marketplace == nil {
		manager, err := marketplace.New(marketplace.Options{
			Logger:            a.logger,
			Store:             a.store,
			BundledPluginsDir: a.cfg.BundledPluginsDir,
			UserPluginsDir:    a.cfg.UserPluginsDir,
			TrustedKeysFile:   a.cfg.TrustedKeysFile,
			CacheDir:          a.cfg.MarketplaceCacheDir,
			ProdMode:          a.cfg.ProdMode,
			AllowUnsigned:     a.cfg.AllowUnsignedPlugins,
		})
		if err != nil {
			return err
		}
		a.marketplace = manager
	}

	bot, err := discordplatform.New(discordplatform.Dependencies{
		Logger: a.logger,
		Token:  a.cfg.DiscordToken,

		OwnerUserID:              a.cfg.OwnerUserID,
		DevGuildID:               a.cfg.DevGuildID,
		CommandRegistrationMode:  a.cfg.CommandRegistrationMode,
		CommandGuildIDs:          a.cfg.CommandGuildIDs,
		CommandRegisterAllGuilds: a.cfg.CommandRegisterAllGuilds,
		BundledPluginsDir:        a.cfg.BundledPluginsDir,
		UserPluginsDir:           a.cfg.UserPluginsDir,
		PermissionsFile:          a.cfg.PermissionsFile,
		ModulesFile:              a.cfg.ModulesFile,
		AllowUnsignedPlugins:     a.cfg.AllowUnsignedPlugins,
		ProdMode:                 a.cfg.ProdMode,
		TrustedKeysFile:          a.cfg.TrustedKeysFile,

		SlashCooldown:          a.cfg.SlashCooldown,
		ComponentCooldown:      a.cfg.ComponentCooldown,
		ModalCooldown:          a.cfg.ModalCooldown,
		SlashCooldownBypass:    a.cfg.SlashCooldownBypass,
		SlashCooldownOverrides: a.cfg.SlashCooldownOverrides,

		I18n:        a.i18n,
		Store:       a.store,
		Metrics:     a.metrics,
		Marketplace: a.marketplace,
	})
	if err != nil {
		return err
	}

	a.bot = bot
	return nil
}

func (a *App) initOpsServer() error {
	if a.ops != nil || a.cfg.OpsAddr == "" {
		return nil
	}

	server, err := ops.New(a.cfg.OpsAddr, a.logger, a.opsSnapshot)
	if err != nil {
		return err
	}
	a.ops = server
	return nil
}

func (a *App) initAdminServer() error {
	if a.admin != nil || a.cfg.AdminAddr == "" {
		return nil
	}
	if a.bot == nil {
		return errors.New("discord bot must be initialized before admin server")
	}
	oauthClient := adminapi.NewDiscordOAuthClient(
		a.cfg.DashboardClientID,
		a.cfg.DashboardClientSecret,
	)
	ownerStatus := func() adminapi.OwnerStatus {
		status := a.bot.OwnerStatus()
		return adminapi.OwnerStatus{
			Configured:      status.Configured,
			Resolved:        status.Resolved,
			Source:          status.Source,
			EffectiveUserID: status.EffectiveUserID,
		}
	}

	server, err := adminapi.New(adminapi.Options{
		Addr:          a.cfg.AdminAddr,
		Logger:        a.logger,
		SessionSecret: a.cfg.DashboardSessionSecret,
		ClientID:      a.cfg.DashboardClientID,
		ClientSecret:  a.cfg.DashboardClientSecret,
		OAuthClient:   oauthClient,
		SessionStore:  a.store.AdminSessions(),
		Service: adminapi.Service{
			Logger:        a.logger,
			Config:        a.cfg,
			Snapshot:      a.opsSnapshot,
			ModuleAdmin:   a.bot.ModuleAdmin(),
			PluginAdmin:   a.bot.PluginAdmin(),
			Marketplace:   a.marketplace,
			Store:         a.store,
			BuildInfo:     buildinfo.Current,
			OAuth:         oauthClient,
			OwnerStatus:   ownerStatus,
			KnownGuildIDs: a.bot.KnownGuildIDs,
			BotHasGuild:   a.bot.HasGuild,
			ListGuildChannels: func(ctx context.Context, guildID uint64) ([]adminapi.GuildChannelInfo, error) {
				items, err := a.bot.ListGuildChannels(ctx, guildID)
				if err != nil {
					return nil, err
				}
				out := make([]adminapi.GuildChannelInfo, 0, len(items))
				for _, item := range items {
					out = append(out, adminapi.GuildChannelInfo{
						ID:       adminapi.Snowflake(item.ID),
						Name:     item.Name,
						Type:     item.Type,
						ParentID: adminapi.Snowflake(item.ParentID),
					})
				}
				return out, nil
			},
			ListGuildRoles: func(ctx context.Context, guildID uint64) ([]adminapi.GuildRoleInfo, error) {
				items, err := a.bot.ListGuildRoles(ctx, guildID)
				if err != nil {
					return nil, err
				}
				out := make([]adminapi.GuildRoleInfo, 0, len(items))
				for _, item := range items {
					out = append(out, adminapi.GuildRoleInfo{
						ID:          adminapi.Snowflake(item.ID),
						Name:        item.Name,
						Color:       item.Color,
						Position:    item.Position,
						Managed:     item.Managed,
						Mentionable: item.Mentionable,
					})
				}
				return out, nil
			},
			SearchGuildMembers: func(ctx context.Context, guildID uint64, query string, limit int) ([]adminapi.GuildMemberInfo, error) {
				items, err := a.bot.SearchGuildMembers(ctx, guildID, query, limit)
				if err != nil {
					return nil, err
				}
				out := make([]adminapi.GuildMemberInfo, 0, len(items))
				for _, item := range items {
					roleIDs := make([]adminapi.Snowflake, 0, len(item.RoleIDs))
					for _, roleID := range item.RoleIDs {
						roleIDs = append(roleIDs, adminapi.Snowflake(roleID))
					}
					out = append(out, adminapi.GuildMemberInfo{
						UserID:      adminapi.Snowflake(item.UserID),
						Username:    item.Username,
						DisplayName: item.DisplayName,
						AvatarURL:   item.AvatarURL,
						Bot:         item.Bot,
						JoinedAt:    item.JoinedAt,
						RoleIDs:     roleIDs,
					})
				}
				return out, nil
			},
			ListGuildEmojis: func(ctx context.Context, guildID uint64) ([]adminapi.GuildEmojiInfo, error) {
				items, err := a.bot.ListGuildEmojis(ctx, guildID)
				if err != nil {
					return nil, err
				}
				out := make([]adminapi.GuildEmojiInfo, 0, len(items))
				for _, item := range items {
					out = append(out, adminapi.GuildEmojiInfo{
						ID:       adminapi.Snowflake(item.ID),
						Name:     item.Name,
						Animated: item.Animated,
					})
				}
				return out, nil
			},
			ListGuildStickers: func(ctx context.Context, guildID uint64) ([]adminapi.GuildStickerInfo, error) {
				items, err := a.bot.ListGuildStickers(ctx, guildID)
				if err != nil {
					return nil, err
				}
				out := make([]adminapi.GuildStickerInfo, 0, len(items))
				for _, item := range items {
					out = append(out, adminapi.GuildStickerInfo{
						ID:          adminapi.Snowflake(item.ID),
						Name:        item.Name,
						Description: item.Description,
						Tags:        item.Tags,
					})
				}
				return out, nil
			},
			SetSlowmode:         a.bot.SetSlowmode,
			SetNickname:         a.bot.SetNickname,
			TimeoutMember:       a.bot.TimeoutMember,
			CreateRole:          a.bot.CreateRole,
			EditRole:            a.bot.EditRole,
			DeleteRole:          a.bot.DeleteRole,
			AddRole:             a.bot.AddRole,
			RemoveRole:          a.bot.RemoveRole,
			PurgeMessages:       a.bot.PurgeMessages,
			CreateEmojiUpload:   a.bot.CreateEmojiUpload,
			EditEmoji:           a.bot.EditEmoji,
			DeleteEmoji:         a.bot.DeleteEmoji,
			CreateStickerUpload: a.bot.CreateStickerUpload,
			EditSticker:         a.bot.EditSticker,
			DeleteSticker:       a.bot.DeleteSticker,
		},
		OwnerStatus: ownerStatus,
	})
	if err != nil {
		return err
	}
	a.admin = server
	return nil
}

func (a *App) opsSnapshot() ops.Snapshot {
	snap := ops.Snapshot{
		StartedAt:        a.startedAt,
		MigrationVersion: a.migrationVersion,
		ProdMode:         a.cfg.ProdMode,
	}
	if msg := a.discordStartErr.Load(); msg != nil {
		snap.DiscordStartError = strings.TrimSpace(*msg)
	}
	if a.bot == nil {
		if a.metrics != nil {
			a.metrics.FillSnapshot(&snap)
		}
		return snap
	}

	stats := a.bot.Stats()
	snap.Ready = stats.Ready
	snap.ModuleCount = stats.ModuleCount
	snap.EnabledModuleCount = stats.EnabledModuleCount
	snap.PluginCount = stats.PluginCount
	snap.EnabledPluginCount = stats.EnabledPluginCount
	snap.BuiltinCommandCount = stats.BuiltinCommandCount
	snap.SlashCommandCount = stats.SlashCommandCount
	snap.UserCommandCount = stats.UserCommandCount
	snap.MessageCommandCount = stats.MessageCommandCount
	if a.metrics != nil {
		a.metrics.FillSnapshot(&snap)
	}
	return snap
}
