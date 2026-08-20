package app

import (
	"context"
	"errors"
	"fmt"
	adminauth "github.com/xsyetopz/go-mamacord/internal/adminapi/auth"
	adminguilds "github.com/xsyetopz/go-mamacord/internal/adminapi/guilds"
	adminservice "github.com/xsyetopz/go-mamacord/internal/adminapi/service"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/signing"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/adminapi"
	"github.com/xsyetopz/go-mamacord/internal/buildinfo"
	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	"github.com/xsyetopz/go-mamacord/internal/ops"
	discordplatform "github.com/xsyetopz/go-mamacord/internal/runtime/discord"
	postgresstore "github.com/xsyetopz/go-mamacord/internal/storage/postgres"
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

type Dependencies struct {
	Logger *slog.Logger
	Config config.Config
}

type appFoundation struct {
	logger           *slog.Logger
	cfg              config.Config
	startedAt        time.Time
	migrationVersion int
}
type appStorage struct {
	store       *postgresstore.Store
	storeCloser interface{ Close() error }
	bundleRepo  bundles.Repository
	marketplace *marketplace.Manager
	i18n        i18n.Registry
}
type appRuntime struct {
	bot             *discordplatform.Bot
	ops             *ops.Server
	admin           *adminapi.Server
	metrics         *ops.Metrics
	discordStartErr atomic.Pointer[string]
}
type appStartup struct{ startupComplete chan struct{} }
type App struct {
	appFoundation
	appStorage
	appRuntime
	appStartup
}
type startupModes struct {
	controlEnabled bool
	discordEnabled bool
}
type startupInitialization struct {
	initStorage          func(context.Context) error
	initBundleRepository func() error
	validatePluginTrust  func(context.Context) error
	initI18n             func() error
	initMarketplace      func() error
	initOpsServer        func() error
	initAdminServer      func() error
	initDiscordBot       func() error
}
type startupStarts struct {
	startOps        func() error
	startAdmin      func() error
	startDiscordBot func(context.Context) error
}
type startupSequence struct {
	startupModes
	startupInitialization
	startupStarts
}

func New(deps Dependencies) (*App, error) {
	if deps.Logger == nil {
		return nil, errors.New("logger is required")
	}
	if deps.Config.Runtime.ProdMode && deps.Config.Plugins.AllowUnsigned {
		return nil, errors.New("prod mode requires signed plugins; set MAMACORD_ALLOW_UNSIGNED_PLUGINS=0")
	}

	return &App{appFoundation: appFoundation{logger: deps.Logger, cfg: deps.Config}, appRuntime: appRuntime{metrics: ops.NewMetrics()}, appStartup: appStartup{startupComplete: make(chan struct{})}}, nil
}

func (a *App) Start(ctx context.Context) error {
	a.startedAt = time.Now()
	phase, err := runStartupSequence(ctx, startupSequence{
		startupModes: startupModes{controlEnabled: a.cfg.HasRuntimeRole(config.RuntimeRoleControl),
			discordEnabled: a.cfg.UsesDiscordRuntime()},
		startupInitialization: startupInitialization{initStorage: a.initStorage,
			initBundleRepository: func() error { return a.initBundleRepository() },
			validatePluginTrust:  a.validatePluginTrust,
			initI18n:             func() error { return a.initI18n() },
			initMarketplace:      func() error { return a.initMarketplace() },
			initOpsServer:        func() error { return a.initOpsServer() },
			initAdminServer:      func() error { return a.initAdminServer() },
			initDiscordBot:       func() error { return a.initDiscordBot() }},
		startupStarts: startupStarts{startOps: func() error {
			if a.ops != nil {
				return a.ops.Start()
			}
			return nil
		}, startAdmin: func() error {
			if a.admin != nil {
				return a.admin.Start()
			}
			return nil
		}, startDiscordBot: func(ctx context.Context) error { return startDiscordBot(ctx, a.bot) }},
	})
	if err != nil {
		if phase != "" {
			return a.keepControlPlaneRunning(ctx, phase, err)
		}
		return err
	}
	if a.startupComplete != nil {
		close(a.startupComplete)
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
	if a.storeCloser != nil {
		return a.storeCloser.Close()
	}
	return nil
}

func (a *App) initStorage(ctx context.Context) error {
	if a.store != nil {
		return nil
	}
	store, version, err := postgresstore.OpenRuntimeStore(ctx, a.cfg)
	if err != nil {
		return err
	}
	a.store = store
	a.storeCloser = store
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
	if !a.cfg.Runtime.ProdMode || a.cfg.Plugins.AllowUnsigned || a.store == nil {
		return nil
	}

	fileKeys := 0
	path := strings.TrimSpace(a.cfg.Plugins.TrustedKeysFile)
	if path != "" {
		keys, err := signing.ReadTrustedKeysFile(path)
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
			a.cfg.Storage.Backend,
		)
	}
	return nil
}

func (a *App) initI18n() error {
	reg, err := i18n.LoadCore(a.cfg.Files.LocalesDir)
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
		OptionDependencies: marketplace.OptionDependencies{Logger: a.logger, Store: a.store, Bundles: a.bundleRepo},
		OptionPaths: marketplace.OptionPaths{
			BundledPluginsDir: a.cfg.Bundles.BundledPluginsDir,
			UserPluginsDir:    a.cfg.Bundles.UserPluginsDir,
			TrustedKeysFile:   a.cfg.Plugins.TrustedKeysFile,
			CacheDir:          a.cfg.Bundles.MarketplaceCacheDir,
		},
		OptionTrust: marketplace.OptionTrust{ProdMode: a.cfg.Runtime.ProdMode, AllowUnsigned: a.cfg.Plugins.AllowUnsigned},
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
	deps := discordplatform.Dependencies{
		ConnectionDependencies: discordplatform.ConnectionDependencies{
			Logger: a.logger, Token: a.cfg.Discord.Token, OwnerUserID: a.cfg.Discord.OwnerUserID,
		},
		RuntimeDependencies: discordplatform.RuntimeDependencies{
			DevGuildID: a.cfg.Discord.DevGuildID, CommandRegistrationMode: a.cfg.Commands.RegistrationMode,
			CommandGuildIDs: a.cfg.Commands.GuildIDs, CommandRegisterAllGuilds: a.cfg.Commands.RegisterAllGuilds,
			EnableGateway: a.cfg.HasRuntimeRole(config.RuntimeRoleGateway), EnableScheduler: a.cfg.HasRuntimeRole(config.RuntimeRoleScheduler),
		},
		PluginDependencies: discordplatform.PluginDependencies{
			BundledPluginsDir: a.cfg.Bundles.BundledPluginsDir, UserPluginsDir: a.cfg.Bundles.UserPluginsDir,
			Bundles: a.bundleRepo, PermissionsFile: a.cfg.Files.PermissionsFile, ModulesFile: a.cfg.Files.ModulesFile,
			AllowUnsignedPlugins: a.cfg.Plugins.AllowUnsigned, ProdMode: a.cfg.Runtime.ProdMode,
			TrustedKeysFile: a.cfg.Plugins.TrustedKeysFile,
		},
		CooldownDependencies: discordplatform.CooldownDependencies{
			SlashCooldown: a.cfg.Cooldowns.Slash, ComponentCooldown: a.cfg.Cooldowns.Component,
			ModalCooldown: a.cfg.Cooldowns.Modal, SlashCooldownBypass: a.cfg.Cooldowns.SlashBypass,
			SlashCooldownOverrides: a.cfg.Cooldowns.SlashOverrides,
		},
		StorageDependencies: discordplatform.StorageDependencies{I18n: a.i18n},
		ServiceDependencies: discordplatform.ServiceDependencies{Metrics: a.metrics, Marketplace: a.marketplace},
	}
	if a.store != nil {
		deps.Restrictions = a.store.Restrictions()
		deps.PluginKV = a.store.PluginKV()
		deps.ModuleStates = a.store.ModuleStates()
		deps.UserSettings = a.store.UserSettings()
		deps.Reminders = a.store.Reminders()
		deps.Guilds = a.store.Guilds()
		deps.Users = a.store.Users()
		deps.GuildMembers = a.store.GuildMembers()
		deps.PluginStore = a.store
	}
	return deps
}

func (a *App) initOpsServer() error {
	if a.ops != nil || a.cfg.Runtime.OpsAddr == "" {
		return nil
	}

	server, err := ops.New(a.cfg.Runtime.OpsAddr, a.logger, a.opsSnapshot)
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
	oauthClient := adminauth.NewDiscordOAuthClient(
		a.cfg.Dashboard.ClientID,
		a.cfg.Dashboard.ClientSecret,
	)

	server, err := adminapi.New(adminapi.Options{
		Addr:          a.cfg.Runtime.AdminAddr,
		Logger:        a.logger,
		SessionSecret: a.cfg.Dashboard.SessionSecret,
		ClientID:      a.cfg.Dashboard.ClientID,
		ClientSecret:  a.cfg.Dashboard.ClientSecret,
		OAuthClient:   oauthClient,
		SessionStore:  a.store.AdminSessions(),
		Service: &adminservice.Service{
			ServiceCore: adminservice.ServiceCore{
				Logger:      a.logger,
				Config:      a.cfg,
				Bundles:     a.bundleRepo,
				Snapshot:    a.opsSnapshot,
				BuildInfo:   buildinfo.Current,
				OwnerStatus: a.ownerStatus,
			},
			ServiceAdmins: adminservice.ServiceAdmins{
				ModuleAdmin: adminModuleAdmin{app: a},
				PluginAdmin: adminPluginAdmin{app: a},
				Marketplace: a.marketplace,
			},
			ServiceStores: adminservice.ServiceStores{
				TrustedSigners: a.store.TrustedSigners(),
				PluginInstalls: a.store.PluginInstalls(),
			},
		},
		GuildService: &adminguilds.Service{
			ServiceCore: adminguilds.ServiceCore{
				ClientID:    a.cfg.Dashboard.ClientID,
				OAuth:       oauthClient,
				ModuleAdmin: adminModuleAdmin{app: a},
			},
			ServiceStores: adminguilds.ServiceStores{
				PluginKV: a.store.PluginKV(),
				Warnings: a.store.Warnings(),
				Audit:    a.store.Audit(),
			},
			DiscordAccess: adminguilds.DiscordAccess{
				KnownGuildIDs: a.knownGuildIDs,
				BotHasGuild:   a.botHasGuild,
			},
			DiscordCatalog: adminguilds.DiscordCatalog{
				ListGuildChannels:  a.listGuildChannels,
				ListGuildRoles:     a.listGuildRoles,
				SearchGuildMembers: a.searchGuildMembers,
				ListGuildEmojis:    a.listGuildEmojis,
				ListGuildStickers:  a.listGuildStickers,
			},
			DiscordModeration: adminguilds.DiscordModeration{
				SetSlowmode:   a.setSlowmode,
				SetNickname:   a.setNickname,
				TimeoutMember: a.timeoutMember,
				PurgeMessages: a.purgeMessages,
			},
			DiscordRoles: adminguilds.DiscordRoles{
				CreateRole: a.createRole,
				EditRole:   a.editRole,
				DeleteRole: a.deleteRole,
				AddRole:    a.addRole,
				RemoveRole: a.removeRole,
			},
			DiscordMedia: adminguilds.DiscordMedia{
				CreateEmojiUpload:   a.createEmojiUpload,
				EditEmoji:           a.editEmoji,
				DeleteEmoji:         a.deleteEmoji,
				CreateStickerUpload: a.createStickerUpload,
				EditSticker:         a.editSticker,
				DeleteSticker:       a.deleteSticker,
			},
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
		Runtime: ops.RuntimeSnapshot{
			StartedAt:        a.startedAt,
			MigrationVersion: a.migrationVersion,
			ProdMode:         a.cfg.Runtime.ProdMode,
			Ready:            !a.cfg.UsesDiscordRuntime(),
		},
	}
	if msg := a.discordStartErr.Load(); msg != nil {
		snap.Runtime.DiscordStartError = strings.TrimSpace(*msg)
	}
	if a.bot == nil {
		if a.metrics != nil {
			a.metrics.FillSnapshot(&snap)
		}
		return snap
	}

	stats := a.bot.Stats()
	snap.Runtime.Ready = stats.Ready
	snap.Modules.Total = stats.ModuleCount
	snap.Modules.Enabled = stats.EnabledModuleCount
	snap.Plugins.Total = stats.PluginCount
	snap.Plugins.Enabled = stats.EnabledPluginCount
	snap.Commands.Builtin = stats.BuiltinCommandCount
	snap.Commands.Slash = stats.SlashCommandCount
	snap.Commands.User = stats.UserCommandCount
	snap.Commands.Message = stats.MessageCommandCount
	if a.metrics != nil {
		a.metrics.FillSnapshot(&snap)
	}
	return snap
}
