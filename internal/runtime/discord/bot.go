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
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
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

	I18n        i18n.Registry
	Store       commandruntime.Store
	Metrics     *ops.Metrics
	Marketplace commandruntime.MarketplaceAdmin

	SlashCooldown          time.Duration
	ComponentCooldown      time.Duration
	ModalCooldown          time.Duration
	SlashCooldownBypass    []string
	SlashCooldownOverrides map[string]time.Duration
}

type Bot struct {
	logger      *slog.Logger
	i18n        i18n.Registry
	store       commandruntime.Store
	metrics     *ops.Metrics
	marketplace commandruntime.MarketplaceAdmin

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

	client   *bot.Client
	commands map[string]slashcmd.Command
	order    []slashcmd.Command

	moduleSeed config.ModulesFile
	modules    map[string]moduleapi.Info

	pluginHost            *pluginhost.Host
	pluginCommands        map[string]discordpluginbridge.Route
	pluginUserCommands    map[string]discordpluginbridge.Route
	pluginMessageCommands map[string]discordpluginbridge.Route
	pluginRoutes          map[string]discordpluginbridge.Route
	pluginAuto            *discordpluginbridge.Automation
	scheduler             *schedulerRuntime
	ready                 atomic.Bool
	stats                 atomic.Value
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
		logger:      deps.Logger.With(slog.String("component", "discord")),
		i18n:        deps.I18n,
		store:       deps.Store,
		metrics:     deps.Metrics,
		marketplace: deps.Marketplace,
		prodMode:    deps.ProdMode,
		devGuildID:  cloneOptionalUint64(deps.DevGuildID),
		owner:       newOwnerState(deps.OwnerUserID),
		cooldowns:   newCooldownTracker(),

		commandRegistrationMode:  commandRegistrationMode,
		commandGuildIDs:          append([]uint64(nil), deps.CommandGuildIDs...),
		commandRegisterAllGuilds: deps.CommandRegisterAllGuilds,
		enableGateway:            deps.EnableGateway,
		enableScheduler:          deps.EnableScheduler,
		moduleSeed:               moduleSeed,
		modules:                  map[string]moduleapi.Info{},
		pluginCommands:           map[string]discordpluginbridge.Route{},
		pluginUserCommands:       map[string]discordpluginbridge.Route{},
		pluginMessageCommands:    map[string]discordpluginbridge.Route{},
		pluginRoutes:             map[string]discordpluginbridge.Route{},
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
