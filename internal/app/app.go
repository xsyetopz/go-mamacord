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
	"github.com/xsyetopz/go-mamacord/internal/bundles"
	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	"github.com/xsyetopz/go-mamacord/internal/ops"
	discordplatform "github.com/xsyetopz/go-mamacord/internal/runtime/discord"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	"github.com/xsyetopz/go-mamacord/internal/storagebootstrap"
)

var (
	newDiscordBotRuntime = discordplatform.New
	startDiscordBot      = func(ctx context.Context, bot *discordplatform.Bot) error {
		if bot == nil {
			return nil
		}
		return bot.Start(ctx)
	}
)

type appStore interface {
	commandruntime.Store
	Close() error
}

type Dependencies struct {
	Logger *slog.Logger
	Config config.Config
}

type App struct {
	logger *slog.Logger
	cfg    config.Config

	store       appStore
	bundleRepo  bundles.Repository
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

type startupSequence struct {
	controlEnabled bool
	discordEnabled bool

	initStorage          func(context.Context) error
	initBundleRepository func() error
	validatePluginTrust  func(context.Context) error
	initI18n             func() error
	initMarketplace      func() error
	initOpsServer        func() error
	initAdminServer      func() error
	startOps             func() error
	startAdmin           func() error
	initDiscordBot       func() error
	startDiscordBot      func(context.Context) error
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
	phase, err := runStartupSequence(ctx, startupSequence{
		controlEnabled: a.cfg.HasRuntimeRole(config.RuntimeRoleControl),
		discordEnabled: a.cfg.UsesDiscordRuntime(),
		initStorage:    a.initStorage,
		initBundleRepository: func() error {
			return a.initBundleRepository()
		},
		validatePluginTrust: a.validatePluginTrust,
		initI18n: func() error {
			return a.initI18n()
		},
		initMarketplace: func() error {
			return a.initMarketplace()
		},
		initOpsServer: func() error {
			return a.initOpsServer()
		},
		initAdminServer: func() error {
			return a.initAdminServer()
		},
		startOps: func() error {
			if a.ops != nil {
				return a.ops.Start()
			}
			return nil
		},
		startAdmin: func() error {
			if a.admin != nil {
				return a.admin.Start()
			}
			return nil
		},
		initDiscordBot: func() error {
			return a.initDiscordBot()
		},
		startDiscordBot: func(ctx context.Context) error {
			return startDiscordBot(ctx, a.bot)
		},
	})
	if err != nil {
		if phase != "" {
			return a.keepControlPlaneRunning(ctx, phase, err)
		}
		return err
	}

	<-ctx.Done()
	return ctx.Err()
}

func runStartupSequence(ctx context.Context, seq startupSequence) (string, error) {
	if seq.initStorage != nil {
		if err := seq.initStorage(ctx); err != nil {
			return "", err
		}
	}
	if seq.initBundleRepository != nil {
		if err := seq.initBundleRepository(); err != nil {
			return "", err
		}
	}
	if seq.validatePluginTrust != nil {
		if err := seq.validatePluginTrust(ctx); err != nil {
			return "", err
		}
	}
	if seq.initI18n != nil {
		if err := seq.initI18n(); err != nil {
			return "", err
		}
	}
	if seq.initMarketplace != nil {
		if err := seq.initMarketplace(); err != nil {
			return "", err
		}
	}
	if seq.initOpsServer != nil {
		if err := seq.initOpsServer(); err != nil {
			return "", err
		}
	}
	if seq.controlEnabled && seq.initAdminServer != nil {
		if err := seq.initAdminServer(); err != nil {
			return "", err
		}
	}
	if seq.startOps != nil {
		if err := seq.startOps(); err != nil {
			return "", err
		}
	}
	if seq.controlEnabled && seq.startAdmin != nil {
		if err := seq.startAdmin(); err != nil {
			return "", err
		}
	}
	if seq.discordEnabled {
		if seq.initDiscordBot != nil {
			if err := seq.initDiscordBot(); err != nil {
				return "initialize", err
			}
		}
		if seq.startDiscordBot != nil {
			if err := seq.startDiscordBot(ctx); err != nil {
				return "start", err
			}
		}
	}
	return "", nil
}

func (a *App) keepControlPlaneRunning(ctx context.Context, phase string, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	a.discordStartErr.Store(&msg)
	a.logger.ErrorContext(ctx, "discord bot failed; keeping control plane running",
		slog.String("phase", strings.TrimSpace(phase)),
		slog.String("err", err.Error()),
	)
	if a.admin == nil && a.ops == nil {
		return err
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
	store, version, err := storagebootstrap.OpenRuntimeStore(ctx, a.cfg)
	if err != nil {
		return err
	}
	a.store = store
	a.migrationVersion = version
	return nil
}

func (a *App) initBundleRepository() error {
	if a.bundleRepo != nil {
		return nil
	}
	repo, err := bundles.Open(a.cfg)
	if err != nil {
		return err
	}
	a.bundleRepo = repo
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
			"prod mode requires at least one trusted signer in %s or the configured %s store; bundled plugins expect a trusted public key file there, and custom plugins should be signed with mamacord gen-signing-key + sign-plugin",
			pathLabel,
			a.cfg.StorageBackend,
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

func (a *App) initMarketplace() error {
	if a.marketplace != nil {
		return nil
	}
	if err := a.initBundleRepository(); err != nil {
		return err
	}
	if a.store == nil {
		return errors.New("store must be initialized before marketplace")
	}
	manager, err := marketplace.New(marketplace.Options{
		Logger:            a.logger,
		Store:             a.store,
		Bundles:           a.bundleRepo,
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
	return nil
}

func (a *App) initDiscordBot() error {
	if !a.cfg.UsesDiscordRuntime() {
		return nil
	}
	if a.bot != nil {
		return nil
	}
	if err := a.initBundleRepository(); err != nil {
		return err
	}
	if a.store == nil {
		return errors.New("store must be initialized before discord bot")
	}
	if err := a.initMarketplace(); err != nil {
		return err
	}

	bot, err := newDiscordBotRuntime(a.discordBotDependencies())
	if err != nil {
		return err
	}

	a.bot = bot
	return nil
}

func (a *App) discordBotDependencies() discordplatform.Dependencies {
	return discordplatform.Dependencies{
		Logger: a.logger,
		Token:  a.cfg.DiscordToken,

		OwnerUserID:              a.cfg.OwnerUserID,
		DevGuildID:               a.cfg.DevGuildID,
		CommandRegistrationMode:  a.cfg.CommandRegistrationMode,
		CommandGuildIDs:          a.cfg.CommandGuildIDs,
		CommandRegisterAllGuilds: a.cfg.CommandRegisterAllGuilds,
		EnableGateway:            a.cfg.HasRuntimeRole(config.RuntimeRoleGateway),
		EnableScheduler:          a.cfg.HasRuntimeRole(config.RuntimeRoleScheduler),
		BundledPluginsDir:        a.cfg.BundledPluginsDir,
		UserPluginsDir:           a.cfg.UserPluginsDir,
		Bundles:                  a.bundleRepo,
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
	}
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
	if a.admin != nil || !a.cfg.ControlAPIEnabled() {
		return nil
	}
	if err := a.initBundleRepository(); err != nil {
		return err
	}
	oauthClient := adminapi.NewDiscordOAuthClient(
		a.cfg.DashboardClientID,
		a.cfg.DashboardClientSecret,
	)

	server, err := adminapi.New(adminapi.Options{
		Addr:          a.cfg.AdminAddr,
		Logger:        a.logger,
		SessionSecret: a.cfg.DashboardSessionSecret,
		ClientID:      a.cfg.DashboardClientID,
		ClientSecret:  a.cfg.DashboardClientSecret,
		OAuthClient:   oauthClient,
		SessionStore:  a.store.AdminSessions(),
		Service: adminapi.Service{
			Logger:              a.logger,
			Config:              a.cfg,
			Bundles:             a.bundleRepo,
			Snapshot:            a.opsSnapshot,
			ModuleAdmin:         adminModuleAdmin{app: a},
			PluginAdmin:         adminPluginAdmin{app: a},
			Marketplace:         a.marketplace,
			Store:               a.store,
			BuildInfo:           buildinfo.Current,
			OAuth:               oauthClient,
			OwnerStatus:         a.ownerStatus,
			KnownGuildIDs:       a.knownGuildIDs,
			BotHasGuild:         a.botHasGuild,
			ListGuildChannels:   a.listGuildChannels,
			ListGuildRoles:      a.listGuildRoles,
			SearchGuildMembers:  a.searchGuildMembers,
			ListGuildEmojis:     a.listGuildEmojis,
			ListGuildStickers:   a.listGuildStickers,
			SetSlowmode:         a.setSlowmode,
			SetNickname:         a.setNickname,
			TimeoutMember:       a.timeoutMember,
			CreateRole:          a.createRole,
			EditRole:            a.editRole,
			DeleteRole:          a.deleteRole,
			AddRole:             a.addRole,
			RemoveRole:          a.removeRole,
			PurgeMessages:       a.purgeMessages,
			CreateEmojiUpload:   a.createEmojiUpload,
			EditEmoji:           a.editEmoji,
			DeleteEmoji:         a.deleteEmoji,
			CreateStickerUpload: a.createStickerUpload,
			EditSticker:         a.editSticker,
			DeleteSticker:       a.deleteSticker,
		},
		OwnerStatus: a.ownerStatus,
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
		Ready:            !a.cfg.UsesDiscordRuntime(),
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
