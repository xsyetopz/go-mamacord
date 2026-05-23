package discordruntime

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/disgoorg/disgo/bot"

	commandapi "github.com/xsyetopz/go-mamacord/internal/commands/api"
	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/ops"
	discordplugin "github.com/xsyetopz/go-mamacord/internal/runtime/discord/plugin"
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
	BundledPluginsDir        string
	UserPluginsDir           string
	PermissionsFile          string
	ModulesFile              string

	ProdMode             bool
	AllowUnsignedPlugins bool
	TrustedKeysFile      string

	I18n        i18n.Registry
	Store       commandapi.Store
	Metrics     *ops.Metrics
	Marketplace commandapi.MarketplaceAdmin

	SlashCooldown          time.Duration
	ComponentCooldown      time.Duration
	ModalCooldown          time.Duration
	SlashCooldownBypass    []string
	SlashCooldownOverrides map[string]time.Duration
}

type Bot struct {
	logger      *slog.Logger
	i18n        i18n.Registry
	store       commandapi.Store
	metrics     *ops.Metrics
	marketplace commandapi.MarketplaceAdmin

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

	client   *bot.Client
	commands map[string]commandapi.SlashCommand
	order    []commandapi.SlashCommand

	moduleSeed config.ModulesFile
	modules    map[string]commandapi.ModuleInfo

	pluginHost            *pluginhost.Host
	pluginCommands        map[string]discordplugin.Route
	pluginUserCommands    map[string]discordplugin.Route
	pluginMessageCommands map[string]discordplugin.Route
	pluginRoutes          map[string]discordplugin.Route
	pluginAuto            *discordplugin.Automation
	ready                 atomic.Bool
	stats                 atomic.Value
}

func New(deps Dependencies) (*Bot, error) {
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
		moduleSeed:               moduleSeed,
		modules:                  map[string]commandapi.ModuleInfo{},
		pluginCommands:           map[string]discordplugin.Route{},
		pluginUserCommands:       map[string]discordplugin.Route{},
		pluginMessageCommands:    map[string]discordplugin.Route{},
		pluginRoutes:             map[string]discordplugin.Route{},
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
	b.resolveOwner(context.Background())
	if b.pluginHost != nil {
		b.pluginAuto = discordplugin.NewAutomation(
			b.logger,
			b.client,
			b.enabledPluginJobs,
			b.enabledPluginEventSubscribers,
			b.pluginRoute,
			b.moduleEnabled,
			b.incAutomationFailure,
			b.incPluginFailure,
			b.ensureDMChannel,
		)
	}

	return b, nil
}

func (b *Bot) ModuleAdmin() commandapi.ModuleAdmin {
	return moduleAdmin{b: b}
}

func (b *Bot) PluginAdmin() commandapi.PluginAdmin {
	return pluginAdmin{b: b}
}
