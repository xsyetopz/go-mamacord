package app

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/ops"
	"github.com/xsyetopz/go-mamacord/internal/postgrestest"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
	postgresstore "github.com/xsyetopz/go-mamacord/internal/storage/postgres"
)

func TestNewRejectsProdModeWithUnsignedPlugins(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := New(Dependencies{
		Logger: logger,
		Config: config.Config{
			ProdMode:             true,
			AllowUnsignedPlugins: true,
		},
	})
	if err == nil {
		t.Fatalf("expected prod-mode plugin trust validation error")
	}
}

func TestInitAdminServerAllowsNilBot(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := postgrestest.OpenMigratedDB(t)
	store, err := postgresstore.New(db)
	if err != nil {
		_ = db.Close()
		t.Fatalf("postgresstore.New: %v", err)
	}

	app := &App{
		logger: logger,
		cfg: config.Config{
			AdminAddr:              "127.0.0.1:8081",
			DashboardSessionSecret: strings.Repeat("x", 32),
			UserPluginsDir:         filepath.Join(t.TempDir(), "plugins"),
		},
		store: store,
	}
	t.Cleanup(func() {
		_ = app.Close()
	})

	if err := app.initMarketplace(); err != nil {
		t.Fatalf("initMarketplace: %v", err)
	}
	if err := app.initAdminServer(); err != nil {
		t.Fatalf("initAdminServer: %v", err)
	}
	if app.admin == nil {
		t.Fatalf("expected admin server to be initialized")
	}
}

func TestStartControlOnlyUsesLivePostgres(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	tmp := t.TempDir()
	dsn := postgrestest.OpenSchemaDSN(t)

	app, err := New(Dependencies{
		Logger: logger,
		Config: config.Config{
			StorageBackend:      config.StorageBackendPostgres,
			PostgresDSN:         dsn,
			Migrations:          filepath.Join(repoRoot(t), "migrations", "postgres"),
			RuntimeRoles:        []config.RuntimeRole{config.RuntimeRoleControl},
			LocalesDir:          filepath.Join(repoRoot(t), "locales"),
			BundledPluginsDir:   filepath.Join(repoRoot(t), "plugins"),
			UserPluginsDir:      filepath.Join(tmp, "plugins"),
			MarketplaceCacheDir: filepath.Join(tmp, "marketplace_cache"),
			BundleBackend:       config.BundleBackendLocal,
			BundleStoreDir:      filepath.Join(tmp, "bundle_store"),
			BundleCacheDir:      filepath.Join(tmp, "bundle_cache"),
			PermissionsFile:     filepath.Join(repoRoot(t), "config", "permissions.json"),
			ModulesFile:         filepath.Join(repoRoot(t), "config", "modules.json"),
			TrustedKeysFile:     filepath.Join(repoRoot(t), "config", "trusted_keys.json"),
		},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(func() {
		_ = app.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- app.Start(ctx)
	}()

	deadline := time.Now().Add(5 * time.Second)
	for app.store == nil || app.migrationVersion == 0 {
		if time.Now().After(deadline) {
			t.Fatalf("app did not finish Postgres startup before deadline: store=%v version=%d", app.store != nil, app.migrationVersion)
		}
		select {
		case err := <-errCh:
			t.Fatalf("Start returned before startup completed: %v", err)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	if app.migrationVersion != 8 {
		t.Fatalf("unexpected migration version: %d", app.migrationVersion)
	}

	cancel()
	if err := <-errCh; err != context.Canceled {
		t.Fatalf("Start returned %v, want %v", err, context.Canceled)
	}
}

func TestInitAdminServerSkipsWhenControlRoleDisabled(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := &App{
		logger: logger,
		cfg: config.Config{
			AdminAddr:              "127.0.0.1:8081",
			DashboardSessionSecret: strings.Repeat("x", 32),
			RuntimeRoles:           []config.RuntimeRole{config.RuntimeRoleGateway},
		},
	}

	if err := app.initAdminServer(); err != nil {
		t.Fatalf("initAdminServer: %v", err)
	}
	if app.admin != nil {
		t.Fatal("expected admin server initialization to be skipped when control role is disabled")
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()

	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestInitDiscordBotSkipsWhenDiscordRolesDisabled(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := &App{
		logger: logger,
		cfg: config.Config{
			RuntimeRoles: []config.RuntimeRole{config.RuntimeRoleControl},
		},
	}

	if err := app.initDiscordBot(); err != nil {
		t.Fatalf("initDiscordBot: %v", err)
	}
	if app.bot != nil {
		t.Fatal("expected discord bot initialization to be skipped when gateway and scheduler roles are disabled")
	}
}

func TestOpsSnapshotReadyWhenDiscordRuntimeRolesDisabled(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg: config.Config{
			RuntimeRoles: []config.RuntimeRole{config.RuntimeRoleControl},
			ProdMode:     true,
		},
		metrics:   ops.NewMetrics(),
		startedAt: time.Unix(1_700_000_000, 0).UTC(),
	}

	snap := app.opsSnapshot()
	if !snap.Ready {
		t.Fatal("expected ops snapshot to report ready when no gateway or scheduler role is enabled")
	}
	if !snap.ProdMode {
		t.Fatal("expected ops snapshot to preserve prod mode")
	}
}

func TestRunStartupSequence_ControlOnlySkipsDiscordBoot(t *testing.T) {
	t.Parallel()

	var steps []string
	phase, err := runStartupSequence(context.Background(), startupSequence{
		controlEnabled: true,
		discordEnabled: false,
		initStorage: func(context.Context) error {
			steps = append(steps, "initStorage")
			return nil
		},
		initBundleRepository: func() error {
			steps = append(steps, "initBundleRepository")
			return nil
		},
		validatePluginTrust: func(context.Context) error {
			steps = append(steps, "validatePluginTrust")
			return nil
		},
		initI18n: func() error {
			steps = append(steps, "initI18n")
			return nil
		},
		initMarketplace: func() error {
			steps = append(steps, "initMarketplace")
			return nil
		},
		initOpsServer: func() error {
			steps = append(steps, "initOpsServer")
			return nil
		},
		initAdminServer: func() error {
			steps = append(steps, "initAdminServer")
			return nil
		},
		startOps: func() error {
			steps = append(steps, "startOps")
			return nil
		},
		startAdmin: func() error {
			steps = append(steps, "startAdmin")
			return nil
		},
		initDiscordBot: func() error {
			steps = append(steps, "initDiscordBot")
			return nil
		},
		startDiscordBot: func(context.Context) error {
			steps = append(steps, "startDiscordBot")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runStartupSequence: %v", err)
	}
	if phase != "" {
		t.Fatalf("unexpected discord failure phase: %q", phase)
	}

	want := []string{
		"initStorage",
		"initBundleRepository",
		"validatePluginTrust",
		"initI18n",
		"initMarketplace",
		"initOpsServer",
		"initAdminServer",
		"startOps",
		"startAdmin",
	}
	if strings.Join(steps, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected startup steps:\n got: %v\nwant: %v", steps, want)
	}
}

func TestRunStartupSequence_GatewayOnlySkipsControlPlaneBoot(t *testing.T) {
	t.Parallel()

	var steps []string
	phase, err := runStartupSequence(context.Background(), startupSequence{
		controlEnabled: false,
		discordEnabled: true,
		initStorage: func(context.Context) error {
			steps = append(steps, "initStorage")
			return nil
		},
		initBundleRepository: func() error {
			steps = append(steps, "initBundleRepository")
			return nil
		},
		validatePluginTrust: func(context.Context) error {
			steps = append(steps, "validatePluginTrust")
			return nil
		},
		initI18n: func() error {
			steps = append(steps, "initI18n")
			return nil
		},
		initMarketplace: func() error {
			steps = append(steps, "initMarketplace")
			return nil
		},
		initOpsServer: func() error {
			steps = append(steps, "initOpsServer")
			return nil
		},
		initAdminServer: func() error {
			steps = append(steps, "initAdminServer")
			return nil
		},
		startOps: func() error {
			steps = append(steps, "startOps")
			return nil
		},
		startAdmin: func() error {
			steps = append(steps, "startAdmin")
			return nil
		},
		initDiscordBot: func() error {
			steps = append(steps, "initDiscordBot")
			return nil
		},
		startDiscordBot: func(context.Context) error {
			steps = append(steps, "startDiscordBot")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runStartupSequence: %v", err)
	}
	if phase != "" {
		t.Fatalf("unexpected discord failure phase: %q", phase)
	}

	want := []string{
		"initStorage",
		"initBundleRepository",
		"validatePluginTrust",
		"initI18n",
		"initMarketplace",
		"initOpsServer",
		"startOps",
		"initDiscordBot",
		"startDiscordBot",
	}
	if strings.Join(steps, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected startup steps:\n got: %v\nwant: %v", steps, want)
	}
}

func TestRunStartupSequence_SchedulerOnlySkipsControlPlaneBoot(t *testing.T) {
	t.Parallel()

	var steps []string
	phase, err := runStartupSequence(context.Background(), startupSequence{
		controlEnabled: false,
		discordEnabled: true,
		initStorage: func(context.Context) error {
			steps = append(steps, "initStorage")
			return nil
		},
		initBundleRepository: func() error {
			steps = append(steps, "initBundleRepository")
			return nil
		},
		validatePluginTrust: func(context.Context) error {
			steps = append(steps, "validatePluginTrust")
			return nil
		},
		initI18n: func() error {
			steps = append(steps, "initI18n")
			return nil
		},
		initMarketplace: func() error {
			steps = append(steps, "initMarketplace")
			return nil
		},
		initOpsServer: func() error {
			steps = append(steps, "initOpsServer")
			return nil
		},
		initAdminServer: func() error {
			steps = append(steps, "initAdminServer")
			return nil
		},
		startOps: func() error {
			steps = append(steps, "startOps")
			return nil
		},
		startAdmin: func() error {
			steps = append(steps, "startAdmin")
			return nil
		},
		initDiscordBot: func() error {
			steps = append(steps, "initDiscordBot")
			return nil
		},
		startDiscordBot: func(context.Context) error {
			steps = append(steps, "startDiscordBot")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runStartupSequence: %v", err)
	}
	if phase != "" {
		t.Fatalf("unexpected discord failure phase: %q", phase)
	}

	want := []string{
		"initStorage",
		"initBundleRepository",
		"validatePluginTrust",
		"initI18n",
		"initMarketplace",
		"initOpsServer",
		"startOps",
		"initDiscordBot",
		"startDiscordBot",
	}
	if strings.Join(steps, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected startup steps:\n got: %v\nwant: %v", steps, want)
	}
}

func TestRunStartupSequence_ControlPlaneStartsBeforeDiscordBootWhenBothEnabled(t *testing.T) {
	t.Parallel()

	var steps []string
	phase, err := runStartupSequence(context.Background(), startupSequence{
		controlEnabled: true,
		discordEnabled: true,
		initStorage: func(context.Context) error {
			steps = append(steps, "initStorage")
			return nil
		},
		initBundleRepository: func() error {
			steps = append(steps, "initBundleRepository")
			return nil
		},
		validatePluginTrust: func(context.Context) error {
			steps = append(steps, "validatePluginTrust")
			return nil
		},
		initI18n: func() error {
			steps = append(steps, "initI18n")
			return nil
		},
		initMarketplace: func() error {
			steps = append(steps, "initMarketplace")
			return nil
		},
		initOpsServer: func() error {
			steps = append(steps, "initOpsServer")
			return nil
		},
		initAdminServer: func() error {
			steps = append(steps, "initAdminServer")
			return nil
		},
		startOps: func() error {
			steps = append(steps, "startOps")
			return nil
		},
		startAdmin: func() error {
			steps = append(steps, "startAdmin")
			return nil
		},
		initDiscordBot: func() error {
			steps = append(steps, "initDiscordBot")
			return nil
		},
		startDiscordBot: func(context.Context) error {
			steps = append(steps, "startDiscordBot")
			return nil
		},
	})
	if err != nil {
		t.Fatalf("runStartupSequence: %v", err)
	}
	if phase != "" {
		t.Fatalf("unexpected discord failure phase: %q", phase)
	}

	startAdminIdx := indexOfStep(steps, "startAdmin")
	initDiscordIdx := indexOfStep(steps, "initDiscordBot")
	if startAdminIdx < 0 || initDiscordIdx < 0 {
		t.Fatalf("expected both startAdmin and initDiscordBot in steps: %v", steps)
	}
	if startAdminIdx > initDiscordIdx {
		t.Fatalf("expected control plane to start before discord init, got steps: %v", steps)
	}
}

func TestDiscordBotDependenciesReflectGatewayOnlyRole(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg: config.Config{
			DiscordToken:   "token",
			RuntimeRoles:   []config.RuntimeRole{config.RuntimeRoleGateway},
			UserPluginsDir: filepath.Join(t.TempDir(), "plugins"),
		},
		metrics: ops.NewMetrics(),
	}

	deps := app.discordBotDependencies()
	if !deps.EnableGateway {
		t.Fatal("expected gateway role to enable gateway runtime")
	}
	if deps.EnableScheduler {
		t.Fatal("expected gateway-only role set to disable scheduler runtime")
	}
}

func TestDiscordBotDependenciesReflectSchedulerOnlyRole(t *testing.T) {
	t.Parallel()

	app := &App{
		cfg: config.Config{
			DiscordToken:   "token",
			RuntimeRoles:   []config.RuntimeRole{config.RuntimeRoleScheduler},
			UserPluginsDir: filepath.Join(t.TempDir(), "plugins"),
		},
		metrics: ops.NewMetrics(),
	}

	deps := app.discordBotDependencies()
	if deps.EnableGateway {
		t.Fatal("expected scheduler-only role set to disable gateway runtime")
	}
	if !deps.EnableScheduler {
		t.Fatal("expected scheduler role to enable scheduler runtime")
	}
}

func indexOfStep(steps []string, want string) int {
	for i, step := range steps {
		if step == want {
			return i
		}
	}
	return -1
}

func TestAppCloseClosesAbstractStore(t *testing.T) {
	t.Parallel()

	var closed atomic.Bool
	app := &App{
		store: fakeAppStore{
			closeFn: func() error {
				closed.Store(true)
				return nil
			},
		},
	}

	if err := app.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if !closed.Load() {
		t.Fatal("expected Close to use the abstract store closer")
	}
}

func TestInitStorageRejectsMalformedPostgresDSN(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app := &App{
		logger: logger,
		cfg: config.Config{
			StorageBackend: config.StorageBackendPostgres,
			PostgresDSN:    "://bad",
			Migrations:     filepath.Clean(filepath.Join("..", "..", "migrations", "postgres")),
		},
	}

	if err := app.initStorage(context.Background()); err == nil {
		t.Fatal("expected initStorage to reject malformed postgres DSN")
	}
}

type fakeAppStore struct {
	closeFn func() error
}

func (f fakeAppStore) Close() error {
	if f.closeFn != nil {
		return f.closeFn()
	}
	return nil
}

func (fakeAppStore) Restrictions() store.RestrictionStore                     { return nil }
func (fakeAppStore) Warnings() store.WarningStore                             { return nil }
func (fakeAppStore) Audit() store.AuditStore                                  { return nil }
func (fakeAppStore) TrustedSigners() store.TrustedSignerStore                 { return nil }
func (fakeAppStore) MarketplaceSources() store.MarketplaceSourceStore         { return nil }
func (fakeAppStore) MarketplaceSourceSyncs() store.MarketplaceSourceSyncStore { return nil }
func (fakeAppStore) PluginInstalls() store.PluginInstallStore                 { return nil }
func (fakeAppStore) TrustedVendors() store.TrustedVendorStore                 { return nil }
func (fakeAppStore) TrustedVendorKeys() store.TrustedVendorKeyStore           { return nil }
func (fakeAppStore) AdminSessions() store.AdminSessionStore                   { return nil }
func (fakeAppStore) PluginKV() store.PluginKVStore                            { return nil }
func (fakeAppStore) ModuleStates() store.ModuleStateStore                     { return nil }
func (fakeAppStore) Users() store.UserStore                                   { return nil }
func (fakeAppStore) Guilds() store.GuildStore                                 { return nil }
func (fakeAppStore) GuildMembers() store.GuildMemberStore                     { return nil }
func (fakeAppStore) UserSettings() store.UserSettingsStore                    { return nil }
func (fakeAppStore) Reminders() store.ReminderStore                           { return nil }
func (fakeAppStore) CheckIns() store.CheckInStore                             { return nil }

var _ commandruntime.Store = fakeAppStore{}
