package pluginhost

import (
	"errors"
	pluginmanifest "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/manifest"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/projection"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/signing"
	contextapi "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/execution/context"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/permissions"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/generation"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
)

type Host struct {
	mu                sync.RWMutex
	generationCounter atomic.Uint64
	hostAuthority
	hostBundles
	hostServices
	hostRegistry
}

type hostAuthority struct {
	prodMode             bool
	allowUnsignedPlugins bool
	trustedKeysFile      string
	permissionsFile      string
	policy               permissions.Policy
}

type hostBundles struct {
	dirs    []string
	bundles bundles.Repository
}

type hostServices struct {
	logger *slog.Logger
	store  Store
	bridge Bridge
	i18n   *i18n.Registry
	http   contextapi.HTTPClient
}

type hostRegistry struct {
	plugins   map[string]*Plugin
	commands  map[string]PluginCommand
	eventSubs map[string][]string
	jobs      []PluginJob
}

type Store interface {
	TrustedSigners() storage.TrustedSignerStore
	PluginInstalls() storage.PluginInstallStore
	PluginKV() storage.PluginKVStore
	UserSettings() storage.UserSettingsStore
	Reminders() storage.ReminderStore
	CheckIns() storage.CheckInStore
	Warnings() storage.WarningStore
	Audit() storage.AuditStore
}
type Options struct {
	BundleOptions
	AuthorityOptions
	RuntimeOptions
}

type BundleOptions struct {
	Dirs       []string
	Repository bundles.Repository
}

type AuthorityOptions struct {
	ProdMode            bool
	AllowUnsignedPlugin bool
	TrustedKeysFile     string
	PermissionsFile     string
}

type RuntimeOptions struct {
	Store  Store
	Bridge Bridge
	HTTP   contextapi.HTTPClient
	Logger *slog.Logger
	I18n   *i18n.Registry
}

type Bridge struct{ Discord DiscordBridge }
type Plugin struct {
	PluginSource
	PluginAuthority
	PluginCatalog
	PluginExecution
}

type PluginSource struct {
	ID        string
	Dir       string
	BundleDir string
	Bundled   bool
}

type PluginAuthority struct {
	Manifest     pluginmanifest.StarlarkManifest
	Signature    *signing.Signature
	Effective    permissions.Permissions
	Capabilities []contract.Capability
}

type PluginCatalog struct {
	Resources  map[string][]byte
	Commands   []projection.Command
	Events     []string
	Jobs       []projection.Job
	Definition contract.Definition
}

type PluginExecution struct {
	I18n    i18n.Registry
	Runtime *generation.GenerationManager
}

type PluginCommand struct {
	PluginID string
	Command  projection.Command
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
	bundleRepo := opts.Repository
	if bundleRepo == nil {
		bundleRepo = bundles.NewLocalRepository()
	}
	policy, err := permissions.LoadPolicyFile(opts.PermissionsFile)
	if err != nil {
		return nil, err
	}
	httpClient := opts.HTTP
	if httpClient == nil {
		client, clientErr := newHTTPJSONClient(httpJSONOptions{Timeout: 2 * time.Second})
		if clientErr != nil {
			return nil, clientErr
		}
		httpClient = client
	}
	return &Host{
		hostAuthority: hostAuthority{
			prodMode: opts.ProdMode, allowUnsignedPlugins: opts.AllowUnsignedPlugin,
			trustedKeysFile: opts.TrustedKeysFile, permissionsFile: opts.PermissionsFile, policy: policy,
		},
		hostBundles: hostBundles{dirs: dirs, bundles: bundleRepo},
		hostServices: hostServices{
			logger: opts.Logger.With(slog.String("component", "plugins")), store: opts.Store,
			bridge: opts.Bridge, i18n: opts.I18n, http: httpClient,
		},
		hostRegistry: hostRegistry{
			plugins: map[string]*Plugin{}, commands: map[string]PluginCommand{}, eventSubs: map[string][]string{},
		},
	}, nil
}
