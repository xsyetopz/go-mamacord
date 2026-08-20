package service

import (
	"context"
	"encoding/json"
	pluginmanifest "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/manifest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/config"
	marketstore "github.com/xsyetopz/go-mamacord/internal/storage/marketplace"
	postgresstore "github.com/xsyetopz/go-mamacord/internal/storage/postgres"
	pgtest "github.com/xsyetopz/go-mamacord/internal/storage/postgres/testkit"
)

func TestPluginsIncludesBundleStateManagedPluginRoot(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	userRoot := filepath.Join(tmp, "plugins")
	entryDir := filepath.Join(userRoot, "bundleme")
	bundleDir := filepath.Join(entryDir, "bundles", "git-abc123")
	writeServicePluginBundle(t, bundleDir, "bundleme")
	if err := bundles.NewLocalRepository().WriteState(entryDir, bundles.State{
		ActiveRelativeDir: filepath.Join("bundles", "git-abc123"),
		Revision:          "abc123",
	}); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	svc := &Service{
		ServiceCore: ServiceCore{
			Config: config.Config{
				Bundles: config.BundleConfig{
					UserPluginsDir: userRoot,
				},
			},
		},
	}

	plugins, err := svc.Plugins()
	if err != nil {
		t.Fatalf("Plugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin summary, got %d", len(plugins))
	}
	if plugins[0].ID != "bundleme" {
		t.Fatalf("unexpected plugin id: got %q want %q", plugins[0].ID, "bundleme")
	}
	if plugins[0].PluginRoot != entryDir {
		t.Fatalf("expected plugin summary plugin_root to stay on plugin root, got %q want %q", plugins[0].PluginRoot, entryDir)
	}
	if plugins[0].BundleRelativeDir != filepath.Join("bundles", "git-abc123") {
		t.Fatalf("expected plugin summary bundle_relative_dir to reflect active bundle, got %q", plugins[0].BundleRelativeDir)
	}
}

func TestPluginsUsesStoredBundleRelativeDirForMarketplacePlugin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmp := t.TempDir()
	userRoot := filepath.Join(tmp, "plugins")
	entryDir := filepath.Join(userRoot, "bundleme")
	bundleRel := filepath.Join("bundles", "git-abc123")
	bundleDir := filepath.Join(entryDir, bundleRel)
	writeServicePluginBundle(t, bundleDir, "bundleme")

	storage := newPluginTestStore(t, tmp)
	if err := storage.PluginInstalls().PutPluginInstall(ctx, marketstore.PluginInstall{
		PluginID: "bundleme",
		PluginInstallSource: marketstore.PluginInstallSource{
			InstallKind: "git",
			SourceID:    "demo",
			GitURL:      "https://example.invalid/demo.git",
			GitRevision: "abc123",
			SourcePath:  "bundleme",
		},
		PluginInstallArtifact: marketstore.PluginInstallArtifact{
			BundleRelativeDir: bundleRel,
			InstalledHashB64:  "hash",
		},
		InstalledAt: time.Unix(1_700_000_000, 0).UTC(),
	}); err != nil {
		t.Fatalf("PutPluginInstall: %v", err)
	}

	svc := &Service{
		ServiceCore: ServiceCore{
			Config: config.Config{
				Bundles: config.BundleConfig{
					UserPluginsDir: userRoot,
				},
			},
		},
		ServiceStores: ServiceStores{
			PluginInstalls: storage.PluginInstalls(),
			TrustedSigners: storage.TrustedSigners(),
		},
	}

	plugins, err := svc.Plugins()
	if err != nil {
		t.Fatalf("Plugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin summary, got %d", len(plugins))
	}
	if plugins[0].PluginRoot != entryDir {
		t.Fatalf("expected plugin summary plugin_root to stay on plugin root, got %q want %q", plugins[0].PluginRoot, entryDir)
	}
	if plugins[0].BundleRelativeDir != bundleRel {
		t.Fatalf("expected plugin summary bundle_relative_dir %q, got %q", bundleRel, plugins[0].BundleRelativeDir)
	}
	if plugins[0].GitRevision != "abc123" {
		t.Fatalf("expected plugin summary git_revision %q, got %q", "abc123", plugins[0].GitRevision)
	}
}

func TestPluginsFallsBackToActiveBundleWhenStoredBundleRelativeDirIsInvalid(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmp := t.TempDir()
	userRoot := filepath.Join(tmp, "plugins")
	entryDir := filepath.Join(userRoot, "bundleme")
	activeRel := filepath.Join("bundles", "git-abc123")
	bundleDir := filepath.Join(entryDir, activeRel)
	writeServicePluginBundle(t, bundleDir, "bundleme")
	if err := bundles.NewLocalRepository().WriteState(entryDir, bundles.State{
		ActiveRelativeDir: activeRel,
		Revision:          "abc123",
	}); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	storage := newPluginTestStore(t, tmp)
	if err := storage.PluginInstalls().PutPluginInstall(ctx, marketstore.PluginInstall{
		PluginID: "bundleme",
		PluginInstallSource: marketstore.PluginInstallSource{
			InstallKind: "git",
			SourceID:    "demo",
			GitURL:      "https://example.invalid/demo.git",
			GitRevision: "broken",
			SourcePath:  "bundleme",
		},
		PluginInstallArtifact: marketstore.PluginInstallArtifact{
			BundleRelativeDir: filepath.Join("..", "escape"),
			InstalledHashB64:  "hash",
		},
		InstalledAt: time.Unix(1_700_000_000, 0).UTC(),
	}); err != nil {
		t.Fatalf("PutPluginInstall: %v", err)
	}

	svc := &Service{
		ServiceCore: ServiceCore{
			Config: config.Config{
				Bundles: config.BundleConfig{
					UserPluginsDir: userRoot,
				},
			},
		},
		ServiceStores: ServiceStores{
			PluginInstalls: storage.PluginInstalls(),
			TrustedSigners: storage.TrustedSigners(),
		},
	}

	plugins, err := svc.Plugins()
	if err != nil {
		t.Fatalf("Plugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin summary, got %d", len(plugins))
	}
	if plugins[0].BundleRelativeDir != activeRel {
		t.Fatalf("expected invalid stored bundle path to fall back to active bundle %q, got %q", activeRel, plugins[0].BundleRelativeDir)
	}
	if plugins[0].Version != "0.1.0" {
		t.Fatalf("expected fallback bundle manifest version %q, got %q", "0.1.0", plugins[0].Version)
	}
}

func TestPluginsUsesRepositoryBundleModifiedForObjectStorePlugin(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmp := t.TempDir()
	storeBackend, err := bundles.NewDirObjectStore(filepath.Join(tmp, "object-store"))
	if err != nil {
		t.Fatalf("NewDirObjectStore: %v", err)
	}
	repo, err := bundles.NewObjectStoreRepository(bundles.ObjectStoreRepositoryOptions{
		Store:    storeBackend,
		CacheDir: filepath.Join(tmp, "bundle-cache"),
	})
	if err != nil {
		t.Fatalf("NewObjectStoreRepository: %v", err)
	}

	userRoot := filepath.Join(tmp, "plugins")
	entryDir := filepath.Join(userRoot, "bundleme")
	srcDir := filepath.Join(tmp, "src")
	writeServicePluginBundle(t, srcDir, "bundleme")
	bundle, err := repo.MaterializeBundle(srcDir, entryDir, "abc123")
	if err != nil {
		t.Fatalf("MaterializeBundle: %v", err)
	}

	storage := newPluginTestStore(t, tmp)
	if err := storage.PluginInstalls().PutPluginInstall(ctx, marketstore.PluginInstall{
		PluginID: "bundleme",
		PluginInstallSource: marketstore.PluginInstallSource{
			InstallKind: "git",
			SourceID:    "demo",
			GitURL:      "https://example.invalid/demo.git",
			GitRevision: "abc123",
			SourcePath:  "bundleme",
		},
		PluginInstallArtifact: marketstore.PluginInstallArtifact{
			BundleRelativeDir: bundle.BundleRelativeDir,
			InstalledHashB64:  bundle.HashB64,
		},
		InstalledAt: time.Unix(1_700_000_000, 0).UTC(),
	}); err != nil {
		t.Fatalf("PutPluginInstall: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmp, "object-store", "bundleme", bundle.BundleRelativeDir, "plugin.star"), []byte(`return { changed = true }`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.star): %v", err)
	}

	svc := &Service{
		ServiceCore: ServiceCore{
			Config: config.Config{
				Bundles: config.BundleConfig{
					UserPluginsDir: userRoot,
				},
			},
			Bundles: repo,
		},
		ServiceStores: ServiceStores{
			PluginInstalls: storage.PluginInstalls(),
			TrustedSigners: storage.TrustedSigners(),
		},
	}

	plugins, err := svc.Plugins()
	if err != nil {
		t.Fatalf("Plugins: %v", err)
	}
	if len(plugins) != 1 {
		t.Fatalf("expected 1 plugin summary, got %d", len(plugins))
	}
	if !plugins[0].LocalModified {
		t.Fatal("expected plugin summary local_modified to be computed through repository artifact resolution")
	}
}

func writeServicePluginBundle(t *testing.T, dir, pluginID string) {
	t.Helper()

	manifest := pluginmanifest.StarlarkManifest{Schema: pluginmanifest.StarlarkManifestSchema, ID: pluginID, Name: pluginID, Version: "0.1.0", Entrypoint: pluginmanifest.StarlarkEntrypoint, Permissions: pluginmanifest.StarlarkManifestPermissions{Network: pluginmanifest.StarlarkManifestNetworkPermissions{Hosts: []string{}}}, Locales: pluginmanifest.StarlarkManifestLocales{Default: "en-US", Supported: []string{"en-US"}}, StateKeys: []string{}, Assets: []string{}}
	payload, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(dir, "plugin.json"), string(payload))
	writeTestFile(t, filepath.Join(dir, "plugin.star"), `PLUGIN = None`)
}

func writeTestFile(t *testing.T, path, contents string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q): %v", path, err)
	}
}

func newPluginTestStore(t *testing.T, _ string) *postgresstore.Store {
	t.Helper()

	db := pgtest.OpenMigratedDB(t)
	t.Cleanup(func() { _ = db.Close() })
	storage, err := postgresstore.New(db)
	if err != nil {
		t.Fatalf("postgresstore.New: %v", err)
	}
	return storage
}
