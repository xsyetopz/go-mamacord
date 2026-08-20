package discordruntime

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/disgoorg/disgo/bot"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	"github.com/xsyetopz/go-mamacord/internal/ops"
	discordpluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type Dependencies struct {
	Logger *slog.Logger
	Token  string

	OwnerUserID              *uint64
	DevGuildID               *uint64
	CommandRegistrationMode  string
	CommandGuildIDs          []uint64
	CommandRegisterAllGuilds bool
	EnableGateway            bool
	EnableScheduler          bool
	BundledPluginsDir        string
	UserPluginsDir           string
	Bundles                  bundles.Repository
	PermissionsFile          string
	ModulesFile              string

	ProdMode             bool
	AllowUnsignedPlugins bool
	TrustedKeysFile      string

	I18n         i18n.Registry
	Restrictions store.RestrictionStore
	PluginKV     store.PluginKVStore
	ModuleStates store.ModuleStateStore
	UserSettings store.UserSettingsStore
	Reminders    store.ReminderStore
	Guilds       store.GuildStore
	Users        store.UserStore
	GuildMembers store.GuildMemberStore
	PluginStore  pluginhost.Store
	Metrics      *ops.Metrics
	Marketplace  commandruntime.MarketplaceAdmin

	SlashCooldown          time.Duration
	ComponentCooldown      time.Duration
	ModalCooldown          time.Duration
	SlashCooldownBypass    []string
	SlashCooldownOverrides map[string]time.Duration
}

type Bot struct {
	logger        *slog.Logger
	i18n          i18n.Registry
	restrictions  store.RestrictionStore
	pluginKV      store.PluginKVStore
	moduleStates  store.ModuleStateStore
	userSettings  store.UserSettingsStore
	reminderStore store.ReminderStore
	guilds        store.GuildStore
	users         store.UserStore
	guildMembers  store.GuildMemberStore
	metrics       *ops.Metrics
	marketplace   commandruntime.MarketplaceAdmin

	prodMode bool

	cooldowns *cooldownTracker

	slashCooldown          time.Duration
	componentCooldownDur   time.Duration
	modalCooldownDur       time.Duration
	slashBypass            map[string]struct{}
	slashCooldownOverrides map[string]time.Duration

	devGuildID *uint64
	owner      ownerState

	commandRegistrationMode  string
	commandGuildIDs          []uint64
	commandRegisterAllGuilds bool
	enableGateway            bool
	enableScheduler          bool

	client *bot.Client

	moduleSeed     config.ModulesFile
	runtimeCatalog atomic.Pointer[runtimeCatalog]

	pluginHost             *pluginhost.Host
	pluginAuto             *discordpluginbridge.Automation
	scheduler              *schedulerRuntime
	ready                  atomic.Bool
	stats                  atomic.Value
	interactionSlots       chan struct{}
	interactionRejectSlots chan struct{}
	interactionRejectQueue chan interactionRejection
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
		logger:        deps.Logger.With(slog.String("component", "discord")),
		i18n:          deps.I18n,
		restrictions:  deps.Restrictions,
		pluginKV:      deps.PluginKV,
		moduleStates:  deps.ModuleStates,
		userSettings:  deps.UserSettings,
		reminderStore: deps.Reminders,
		guilds:        deps.Guilds,
		users:         deps.Users,
		guildMembers:  deps.GuildMembers,
		metrics:       deps.Metrics,
		marketplace:   deps.Marketplace,
		prodMode:      deps.ProdMode,
		devGuildID:    cloneOptionalUint64(deps.DevGuildID),
		owner:         newOwnerState(deps.OwnerUserID),
		cooldowns:     newCooldownTracker(),

		commandRegistrationMode:  commandRegistrationMode,
		commandGuildIDs:          append([]uint64(nil), deps.CommandGuildIDs...),
		commandRegisterAllGuilds: deps.CommandRegisterAllGuilds,
		enableGateway:            deps.EnableGateway,
		enableScheduler:          deps.EnableScheduler,
		moduleSeed:               moduleSeed,
		interactionSlots:         make(chan struct{}, maximumConcurrentInteractions),
		interactionRejectSlots:   make(chan struct{}, maximumConcurrentInteractionRejections),
		interactionRejectQueue:   make(chan interactionRejection, maximumPendingInteractionRejections),
	}
	b.slashCooldown = deps.SlashCooldown
	b.componentCooldownDur = deps.ComponentCooldown
	b.modalCooldownDur = deps.ModalCooldown
	b.slashBypass = buildSlashBypass(deps.SlashCooldownBypass)
	b.slashCooldownOverrides = cloneCooldownOverrides(deps.SlashCooldownOverrides)

	if initErr := b.initPlugins(deps); initErr != nil {
		return nil, initErr
	}

	if refreshErr := b.refreshRuntimeCatalog(context.Background()); refreshErr != nil {
		return nil, refreshErr
	}

	client, err := b.newClient(deps.Token)
	if err != nil {
		return nil, err
	}
	b.client = client
	if b.pluginHost != nil {
		b.pluginAuto = discordpluginbridge.NewAutomation(
			b.logger,
			b.client,
			b.enabledPluginEventSubscribers,
			b.pluginRoute,
			b.moduleEnabled,
			b.incAutomationFailure,
			b.incPluginFailure,
			b.ensureDMChannel,
		)
	}
	b.scheduler = newSchedulerRuntime(
		b.logger,
		reminderPollInterval,
		b.pollReminders,
		b.enabledPluginJobs,
		func(ctx context.Context, job pluginhost.PluginJob) {
			if b.pluginAuto != nil {
				b.pluginAuto.RunJob(ctx, job)
			}
		},
	)

	return b, nil
}

func (b *Bot) ModuleAdmin() moduleapi.Admin {
	return moduleAdmin{b: b}
}

func (b *Bot) PluginAdmin() commandruntime.PluginAdmin {
	return pluginAdmin{b: b}
}
