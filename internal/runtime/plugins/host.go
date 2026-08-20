package pluginhost

import (
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/permissions"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/httpjson"
	starlarkruntime "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type Host struct {
	mu                   sync.RWMutex
	logger               *slog.Logger
	dirs                 []string
	prodMode             bool
	allowUnsignedPlugins bool
	trustedKeysFile      string
	permissionsFile      string
	store                Store
	bundles              bundles.Repository
	bridge               Bridge
	policy               permissions.Policy
	i18n                 *i18n.Registry
	http                 starlarkruntime.HTTPClient
	generationCounter    atomic.Uint64
	plugins              map[string]*Plugin
	commands             map[string]PluginCommand
	eventSubs            map[string][]string
	jobs                 []PluginJob
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
	HTTP                starlarkruntime.HTTPClient
	Logger              *slog.Logger
	I18n                *i18n.Registry
}
type Bridge struct{ Discord DiscordBridge }
type Plugin struct {
	ID           string
	Dir          string
	BundleDir    string
	Bundled      bool
	Manifest     StarlarkManifest
	Signature    *Signature
	Effective    permissions.Permissions
	Capabilities []contract.Capability
	Resources    map[string][]byte
	Commands     []Command
	Events       []string
	Jobs         []Job
	Definition   contract.Definition
	I18n         i18n.Registry
	Runtime      *starlarkruntime.GenerationManager
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

func NewHost(opts Options) (*Host, error) {
	dirs := make([]string, 0, len(opts.Dirs))
	for _, dir := range opts.Dirs {
		if dir = strings.TrimSpace(dir); dir != "" {
			dirs = append(dirs, dir)
		}
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
	httpClient := opts.HTTP
	if httpClient == nil {
		client, clientErr := httpjson.New(httpjson.Options{Timeout: 2 * time.Second})
		if clientErr != nil {
			return nil, clientErr
		}
		httpClient = client
	}
	return &Host{logger: opts.Logger.With(slog.String("component", "plugins")), dirs: dirs, prodMode: opts.ProdMode, allowUnsignedPlugins: opts.AllowUnsignedPlugin, trustedKeysFile: opts.TrustedKeysFile, permissionsFile: opts.PermissionsFile, store: opts.Store, bundles: bundleRepo, bridge: opts.Bridge, policy: policy, i18n: opts.I18n, http: httpClient, plugins: map[string]*Plugin{}, commands: map[string]PluginCommand{}, eventSubs: map[string][]string{}}, nil
}
