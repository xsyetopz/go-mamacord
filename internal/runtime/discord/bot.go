package discordruntime

import (
	"context"
	discordautomation "github.com/xsyetopz/go-mamacord/internal/runtime/discord/automation"
	discordcatalog "github.com/xsyetopz/go-mamacord/internal/runtime/discord/catalog"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/control"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	discordexecutor "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge/executor"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/router/cooldown"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/disgoorg/disgo/bot"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/ops"
	discordpluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/host"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
)

type Dependencies struct {
	ConnectionDependencies
	RuntimeDependencies
	PluginDependencies
	StorageDependencies
	ServiceDependencies
	CooldownDependencies
}

type ConnectionDependencies struct {
	Logger      *slog.Logger
	Token       string
	OwnerUserID *uint64
}

type RuntimeDependencies struct {
	DevGuildID               *uint64
	CommandRegistrationMode  string
	CommandGuildIDs          []uint64
	CommandRegisterAllGuilds bool
	EnableGateway            bool
	EnableScheduler          bool
}

type PluginDependencies struct {
	BundledPluginsDir    string
	UserPluginsDir       string
	Bundles              bundles.Repository
	PermissionsFile      string
	ModulesFile          string
	ProdMode             bool
	AllowUnsignedPlugins bool
	TrustedKeysFile      string
}

type StorageDependencies struct {
	I18n         i18n.Registry
	Restrictions storage.RestrictionStore
	PluginKV     storage.PluginKVStore
	ModuleStates storage.ModuleStateStore
	UserSettings storage.UserSettingsStore
	Reminders    storage.ReminderStore
	Guilds       storage.GuildStore
	Users        storage.UserStore
	GuildMembers storage.GuildMemberStore
	PluginStore  pluginhost.Store
}

type ServiceDependencies struct {
	Metrics     *ops.Metrics
	Marketplace commandruntime.MarketplaceAdmin
}

type CooldownDependencies struct {
	SlashCooldown          time.Duration
	ComponentCooldown      time.Duration
	ModalCooldown          time.Duration
	SlashCooldownBypass    []string
	SlashCooldownOverrides map[string]time.Duration
}

type Bot struct {
	botServices
	botStores
	botRuntimeConfig
	botCooldowns
	botRuntime
	interactions *interactions.Dispatcher
}

type botServices struct {
	logger      *slog.Logger
	i18n        i18n.Registry
	metrics     *ops.Metrics
	marketplace commandruntime.MarketplaceAdmin
}

type botStores struct {
	restrictions  storage.RestrictionStore
	pluginKV      storage.PluginKVStore
	moduleStates  storage.ModuleStateStore
	userSettings  storage.UserSettingsStore
	reminderStore storage.ReminderStore
	guilds        storage.GuildStore
	users         storage.UserStore
	guildMembers  storage.GuildMemberStore
}

type botRuntimeConfig struct {
	prodMode                 bool
	devGuildID               *uint64
	owner                    control.Owner
	commandRegistrationMode  string
	commandGuildIDs          []uint64
	commandRegisterAllGuilds bool
	enableGateway            bool
	enableScheduler          bool
	moduleSeed               config.ModulesFile
}

type botCooldowns struct {
	cooldowns      *cooldown.Tracker
	cooldownPolicy cooldown.Policy
}

type botRuntime struct {
	client             *bot.Client
	catalog            *discordcatalog.Runtime
	pluginHost         *pluginhost.Host
	pluginAuto         *discordpluginbridge.Automation
	scheduler          *discordautomation.Scheduler
	ready              atomic.Bool
	stats              atomic.Value
	control            *control.Admin
	pluginInteractions *discordpluginbridge.InteractionDispatcher
	automation         *discordautomation.Service
}

func New(deps Dependencies) (*Bot, error) {
	deps.EnableGateway, deps.EnableScheduler = normalizeRuntimeRoleDeps(deps.EnableGateway, deps.EnableScheduler)
	if err := validateNewDeps(deps); err != nil {
		return nil, err
	}

	commandRegistrationMode, err := normalizeCommandRegistrationMode(deps.CommandRegistrationMode)
	if err != nil {
		return nil, err
	}

	moduleSeed, err := config.LoadModulesFile(deps.ModulesFile)
	if err != nil {
		return nil, err
	}

	b := &Bot{
		botServices: botServices{
			logger: deps.Logger.With(slog.String("component", "discord")), i18n: deps.I18n,
			metrics: deps.Metrics, marketplace: deps.Marketplace,
		},
		botStores: botStores{
			restrictions: deps.Restrictions, pluginKV: deps.PluginKV, moduleStates: deps.ModuleStates,
			userSettings: deps.UserSettings, reminderStore: deps.Reminders, guilds: deps.Guilds,
			users: deps.Users, guildMembers: deps.GuildMembers,
		},
		botRuntimeConfig: botRuntimeConfig{
			prodMode: deps.ProdMode, devGuildID: cloneUint64Pointer(deps.DevGuildID), owner: control.NewOwner(deps.OwnerUserID),
			commandRegistrationMode: commandRegistrationMode, commandGuildIDs: append([]uint64(nil), deps.CommandGuildIDs...),
			commandRegisterAllGuilds: deps.CommandRegisterAllGuilds, enableGateway: deps.EnableGateway,
			enableScheduler: deps.EnableScheduler, moduleSeed: moduleSeed,
		},
		botCooldowns: botCooldowns{cooldowns: cooldown.NewTracker()},
	}
	b.interactions = interactions.NewDispatcher(b.logger, b.incInteractionFailure)
	b.cooldownPolicy = cooldown.NewPolicy(
		deps.SlashCooldown, deps.ComponentCooldown, deps.ModalCooldown,
		deps.SlashCooldownBypass, deps.SlashCooldownOverrides,
	)

	if initErr := b.initPlugins(deps); initErr != nil {
		return nil, initErr
	}

	b.catalog = discordcatalog.NewRuntime(b.logger, b.moduleSeed, b.moduleStates, b.pluginKV, b.pluginHost)

	if refreshErr := b.refreshRuntimeCatalog(context.Background()); refreshErr != nil {
		return nil, refreshErr
	}

	b.pluginInteractions = discordpluginbridge.NewInteractionDispatcher(discordpluginbridge.InteractionOptions{
		InteractionAccess: discordpluginbridge.InteractionAccess{
			Logger: b.logger, I18n: b.i18n, Route: b.catalog.PluginRoute,
			ModuleEnabled: b.moduleEnabled, GuildPluginEnabled: b.guildPluginEnabled, IsOwner: b.isOwner,
		},
		InteractionLimits: discordpluginbridge.InteractionLimits{
			Cooldowns: b.cooldowns, Policy: b.cooldownPolicy, Interactions: b.interactions,
		},
		InteractionMetrics: discordpluginbridge.InteractionMetrics{
			IncInteraction: b.incInteraction, IncInteractionFailure: b.incInteractionFailure, IncPluginFailure: b.incPluginFailure,
		},
	})

	client, err := b.newClient(deps.Token)
	if err != nil {
		return nil, err
	}
	b.client = client
	b.automation = discordautomation.NewService(b.logger, b.i18n, b.reminderStore, b.userSettings, b.client, b.incReminderFailure)
	b.control = control.NewAdmin(control.AdminOptions{
		Client: b.client, PluginHost: b.pluginHost, ModuleInfos: b.moduleInfos,
		ReloadModules: b.reloadModules, SetModuleEnabled: b.setModuleEnabled,
		ResetModule: b.resetModule, Executor: discordexecutor.Discord{ClientProvider: func() *bot.Client { return b.client }, EnsureDMChannelFunc: b.automation.EnsureDMChannel},
	})
	if b.pluginHost != nil {
		b.pluginAuto = discordpluginbridge.NewAutomation(
			b.logger,
			b.client,
			b.enabledPluginEventSubscribers,
			b.pluginRoute,
			b.moduleEnabled,
			b.incAutomationFailure,
			b.incPluginFailure,
		)
	}
	b.scheduler = discordautomation.NewScheduler(
		b.logger,
		discordautomation.DefaultReminderPollInterval,
		b.automation.PollDue,
		b.enabledPluginJobs,
		func(ctx context.Context, job pluginhost.PluginJob) {
			if b.pluginAuto != nil {
				b.pluginAuto.RunJob(ctx, job)
			}
		},
	)

	return b, nil
}

func (b *Bot) isOwner(userID uint64) bool {
	return b != nil && b.owner.Is(userID)
}

func (b *Bot) OwnerStatus() control.OwnerStatus {
	if b == nil {
		return control.NewOwner(nil).Status()
	}
	return b.owner.Status()
}

func (b *Bot) Control() *control.Admin {
	if b == nil {
		return nil
	}
	return b.control
}
