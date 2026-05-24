package pluginhost

import (
	"errors"
	"log/slog"
	"strings"
	"sync"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/permissions"
	pluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/bridge"
	luaplugin "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type Host struct {
	mu sync.RWMutex

	logger *slog.Logger
	dirs   []string

	prodMode             bool
	allowUnsignedPlugins bool
	trustedKeysFile      string
	permissionsFile      string

	store   Store
	bundles bundles.Repository
	bridge  Bridge
	policy  permissions.Policy
	i18n    *i18n.Registry

	plugins  map[string]*Plugin
	commands map[string]PluginCommand

	eventSubs map[string][]string
	jobs      []PluginJob
}

type Store interface {
	TrustedSigners() store.TrustedSignerStore
	PluginInstalls() store.PluginInstallStore
	PluginKV() store.PluginKVStore
	UserSettings() store.UserSettingsStore
	Reminders() store.ReminderStore
	CheckIns() store.CheckInStore
	Warnings() store.WarningStore
	Audit() store.AuditStore
}

type Options struct {
	Dirs                []string
	ProdMode            bool
	AllowUnsignedPlugin bool
	TrustedKeysFile     string
	PermissionsFile     string
	Store               Store
	Bundles             bundles.Repository
	Bridge              Bridge
	Logger              *slog.Logger
	I18n                *i18n.Registry
}

type Bridge struct {
	Discord luaplugin.Discord
}

type Plugin struct {
	ID        string
	Dir       string
	BundleDir string
	Bundled   bool

	Manifest  Manifest
	Signature *Signature
	Effective permissions.Permissions
	Commands  []Command
	Events    []string
	Jobs      []Job

	VM *luaplugin.VM
}

type PluginCommand struct {
	PluginID string
	Command  Command
}

type PluginJob struct {
	PluginID string
	JobID    string
	Schedule string
}

type Payload struct {
	GuildID     string
	ChannelID   string
	UserID      string
	Locale      string
	IsOwner     bool
	Options     luaplugin.PayloadOptions
	Interaction pluginbridge.Interaction
}

func PayloadOptionsFromMap(options map[string]any) luaplugin.PayloadOptions {
	return luaplugin.NewPayloadOptions(options)
}

func NewHost(opts Options) (*Host, error) {
	dirs := make([]string, 0, len(opts.Dirs))
	for _, dir := range opts.Dirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		dirs = append(dirs, dir)
	}
	if len(dirs) == 0 {
		return nil, errors.New("at least one plugins dir is required")
	}
	if opts.Logger == nil {
		return nil, errors.New("logger is required")
	}
	bundleRepo := opts.Bundles
	if bundleRepo == nil {
		bundleRepo = bundles.NewLocalRepository()
	}

	policy, err := permissions.LoadPolicyFile(opts.PermissionsFile)
	if err != nil {
		return nil, err
	}

	return &Host{
		logger:               opts.Logger.With(slog.String("component", "plugins")),
		dirs:                 dirs,
		prodMode:             opts.ProdMode,
		allowUnsignedPlugins: opts.AllowUnsignedPlugin,
		trustedKeysFile:      opts.TrustedKeysFile,
		permissionsFile:      opts.PermissionsFile,
		store:                opts.Store,
		bundles:              bundleRepo,
		bridge:               opts.Bridge,
		policy:               policy,
		i18n:                 opts.I18n,
		plugins:              map[string]*Plugin{},
		commands:             map[string]PluginCommand{},
		eventSubs:            map[string][]string{},
	}, nil
}
