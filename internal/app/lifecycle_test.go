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

	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/ops"
	postgresstore "github.com/xsyetopz/go-mamacord/internal/storage/postgres"
	pgtest "github.com/xsyetopz/go-mamacord/internal/storage/postgres/testkit"
)

func TestNewRejectsProdModeWithUnsignedPlugins(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := New(Dependencies{
		Logger: logger,
		Config: config.Config{
			Runtime: config.RuntimeConfig{
				ProdMode: true,
			},
			Plugins: config.PluginConfig{
				AllowUnsigned: true,
			},
		},
	})
	if err == nil {
		t.Fatalf("expected prod-mode plugin trust validation error")
	}
}

func TestInitAdminServerAllowsNilBot(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	db := pgtest.OpenMigratedDB(t)
	store, err := postgresstore.New(db)
	if err != nil {
		_ = db.Close()
		t.Fatalf("postgresstore.New: %v", err)
	}

	app := &App{
		appFoundation: appFoundation{
			logger: logger,
			cfg: config.Config{
				Runtime:   config.RuntimeConfig{AdminAddr: "127.0.0.1:8081"},
				Bundles:   config.BundleConfig{UserPluginsDir: filepath.Join(t.TempDir(), "plugins")},
				Dashboard: config.DashboardConfig{SessionSecret: strings.Repeat("x", 32)},
			},
		},
		appStorage: appStorage{store: store},
	}

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
	dsn := pgtest.OpenSchemaDSN(t)

	app, err := New(Dependencies{
		Logger: logger,
		Config: config.Config{
			Storage: config.StorageConfig{
				Backend:     config.StorageBackendPostgres,
				PostgresDSN: dsn,
				Migrations:  filepath.Join(repoRoot(t), "migrations", "postgres"),
			},
			Runtime: config.RuntimeConfig{
				Roles: []config.RuntimeRole{config.RuntimeRoleControl},
			},
			Files: config.FileConfig{
				LocalesDir:      filepath.Join(repoRoot(t), "locales"),
				PermissionsFile: filepath.Join(repoRoot(t), "config", "permissions.json"),
				ModulesFile:     filepath.Join(repoRoot(t), "config", "modules.json"),
			},
			Bundles: config.BundleConfig{
				BundledPluginsDir:   filepath.Join(repoRoot(t), "plugins"),
				UserPluginsDir:      filepath.Join(tmp, "plugins"),
				MarketplaceCacheDir: filepath.Join(tmp, "marketplace_cache"),
				Backend:             config.BundleBackendLocal,
				StoreDir:            filepath.Join(tmp, "bundle_store"),
				CacheDir:            filepath.Join(tmp, "bundle_cache"),
			},
			Plugins: config.PluginConfig{
				TrustedKeysFile: filepath.Join(repoRoot(t), "config", "trusted_keys.json"),
			},
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

	select {
	case <-app.startupComplete:
	case err := <-errCh:
		t.Fatalf("Start returned before startup completed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("app did not finish Postgres startup before deadline")
	}

	if app.migrationVersion != 9 {
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
		appFoundation: appFoundation{
			logger: logger,
			cfg: config.Config{
				Runtime:   config.RuntimeConfig{AdminAddr: "127.0.0.1:8081", Roles: []config.RuntimeRole{config.RuntimeRoleGateway}},
				Dashboard: config.DashboardConfig{SessionSecret: strings.Repeat("x", 32)},
			},
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
		appFoundation: appFoundation{
			logger: logger,
			cfg:    config.Config{Runtime: config.RuntimeConfig{Roles: []config.RuntimeRole{config.RuntimeRoleControl}}},
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
		appFoundation: appFoundation{
			cfg:       config.Config{Runtime: config.RuntimeConfig{Roles: []config.RuntimeRole{config.RuntimeRoleControl}, ProdMode: true}},
			startedAt: time.Unix(1_700_000_000, 0).UTC(),
		},
		appRuntime: appRuntime{metrics: ops.NewMetrics()},
	}

	snap := app.opsSnapshot()
	if !snap.Runtime.Ready {
		t.Fatal("expected ops snapshot to report ready when no gateway or scheduler role is enabled")
	}
	if !snap.Runtime.ProdMode {
		t.Fatal("expected ops snapshot to preserve prod mode")
	}
}

func TestRunStartupSequence_ControlOnlySkipsDiscordBoot(t *testing.T) {
	t.Parallel()

	var steps []string
	phase, err := runStartupSequence(context.Background(), startupSequence{
		startupModes: startupModes{
			controlEnabled: true,
			discordEnabled: false,
		},
		startupInitialization: startupInitialization{
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
			initDiscordBot: func() error {
				steps = append(steps, "initDiscordBot")
				return nil
			},
		},
		startupStarts: startupStarts{
			startOps: func() error {
				steps = append(steps, "startOps")
				return nil
			},
			startAdmin: func() error {
				steps = append(steps, "startAdmin")
				return nil
			},
			startDiscordBot: func(context.Context) error {
				steps = append(steps, "startDiscordBot")
				return nil
			},
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
		startupModes: startupModes{
			controlEnabled: false,
			discordEnabled: true,
		},
		startupInitialization: startupInitialization{
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
			initDiscordBot: func() error {
				steps = append(steps, "initDiscordBot")
				return nil
			},
		},
		startupStarts: startupStarts{
			startOps: func() error {
				steps = append(steps, "startOps")
				return nil
			},
			startAdmin: func() error {
				steps = append(steps, "startAdmin")
				return nil
			},
			startDiscordBot: func(context.Context) error {
				steps = append(steps, "startDiscordBot")
				return nil
			},
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
		startupModes: startupModes{
			controlEnabled: false,
			discordEnabled: true,
		},
		startupInitialization: startupInitialization{
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
			initDiscordBot: func() error {
				steps = append(steps, "initDiscordBot")
				return nil
			},
		},
		startupStarts: startupStarts{
			startOps: func() error {
				steps = append(steps, "startOps")
				return nil
			},
			startAdmin: func() error {
				steps = append(steps, "startAdmin")
				return nil
			},
			startDiscordBot: func(context.Context) error {
				steps = append(steps, "startDiscordBot")
				return nil
			},
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
		startupModes: startupModes{
			controlEnabled: true,
			discordEnabled: true,
		},
		startupInitialization: startupInitialization{
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
			initDiscordBot: func() error {
				steps = append(steps, "initDiscordBot")
				return nil
			},
		},
		startupStarts: startupStarts{
			startOps: func() error {
				steps = append(steps, "startOps")
				return nil
			},
			startAdmin: func() error {
				steps = append(steps, "startAdmin")
				return nil
			},
			startDiscordBot: func(context.Context) error {
				steps = append(steps, "startDiscordBot")
				return nil
			},
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
		appFoundation: appFoundation{
			cfg: config.Config{
				Discord: config.DiscordConfig{Token: "token"},
				Runtime: config.RuntimeConfig{Roles: []config.RuntimeRole{config.RuntimeRoleGateway}},
				Bundles: config.BundleConfig{UserPluginsDir: filepath.Join(t.TempDir(), "plugins")},
			},
		},
		appRuntime: appRuntime{metrics: ops.NewMetrics()},
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
		appFoundation: appFoundation{
			cfg: config.Config{
				Discord: config.DiscordConfig{Token: "token"},
				Runtime: config.RuntimeConfig{Roles: []config.RuntimeRole{config.RuntimeRoleScheduler}},
				Bundles: config.BundleConfig{UserPluginsDir: filepath.Join(t.TempDir(), "plugins")},
			},
		},
		appRuntime: appRuntime{metrics: ops.NewMetrics()},
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
		appStorage: appStorage{
			storeCloser: fakeAppStore{closeFn: func() error {
				closed.Store(true)
				return nil
			}},
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
		appFoundation: appFoundation{
			logger: logger,
			cfg: config.Config{Storage: config.StorageConfig{
				Backend: config.StorageBackendPostgres, PostgresDSN: "://bad",
				Migrations: filepath.Clean(filepath.Join("..", "..", "migrations", "postgres")),
			}},
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
