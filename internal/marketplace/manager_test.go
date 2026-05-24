package marketplace_test

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	"github.com/xsyetopz/go-mamacord/internal/postgrestest"
	postgresstore "github.com/xsyetopz/go-mamacord/internal/storage/postgres"
)

func TestManagerInstallAndForceUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager, storage, _ := newTestManager(t, false)
	defer storage.Close()

	repoDir := t.TempDir()
	writePlugin(t, repoDir, "weather", "Weather", "0.1.0", `return {}`)
	gitCommitAll(t, repoDir, "initial")

	if _, err := manager.UpsertSource(ctx, marketplace.SourceUpsert{
		SourceID: "demo",
		GitURL:   repoDir,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("UpsertSource: %v", err)
	}

	results, err := manager.Search(ctx, marketplace.SearchQuery{Refresh: true})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 1 || results[0].PluginID != "weather" {
		t.Fatalf("unexpected search results: %#v", results)
	}

	install, err := manager.Install(ctx, marketplace.InstallRequest{
		SourceID: "demo",
		PluginID: "weather",
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if install.Enabled {
		t.Fatalf("expected marketplace installs to start disabled")
	}

	moduleState, ok, err := storage.ModuleStates().GetModuleState(ctx, "weather")
	if err != nil {
		t.Fatalf("GetModuleState: %v", err)
	}
	if !ok || moduleState.Enabled {
		t.Fatalf("expected disabled module state, got %#v ok=%t", moduleState, ok)
	}

	pluginRoot := install.PluginRoot
	targetDir, err := bundles.NewLocalRepository().ResolveBundleDir(pluginRoot)
	if err != nil {
		t.Fatalf("ResolveBundleDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "plugin.lua"), []byte(`return { changed = true }`), 0o644); err != nil {
		t.Fatalf("WriteFile(local change): %v", err)
	}
	if pluginRoot == "" {
		t.Fatal("expected install result to include plugin_root")
	}

	writePlugin(t, repoDir, "weather", "Weather", "0.2.0", `return { updated = true }`)
	gitCommitAll(t, repoDir, "update")

	if _, err := manager.Update(ctx, marketplace.UpdateRequest{PluginID: "weather"}); err == nil || !strings.Contains(err.Error(), "local modifications") {
		t.Fatalf("expected local modifications error, got %v", err)
	}

	update, err := manager.Update(ctx, marketplace.UpdateRequest{PluginID: "weather", Force: true})
	if err != nil {
		t.Fatalf("Update(force): %v", err)
	}
	if update.GitRevision == install.GitRevision {
		t.Fatalf("expected revision change after update")
	}
}

func TestManagerRejectsUnsignedInstallInProd(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager, storage, _ := newTestManager(t, true)
	defer storage.Close()

	repoDir := t.TempDir()
	writePlugin(t, repoDir, "forecast", "Forecast", "0.1.0", `return {}`)
	gitCommitAll(t, repoDir, "initial")

	if _, err := manager.UpsertSource(ctx, marketplace.SourceUpsert{
		SourceID: "prod",
		GitURL:   repoDir,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("UpsertSource: %v", err)
	}

	if _, err := manager.Install(ctx, marketplace.InstallRequest{
		SourceID: "prod",
		PluginID: "forecast",
	}); err == nil || !strings.Contains(err.Error(), "trusted signer") {
		t.Fatalf("expected trusted signer error, got %v", err)
	}
}

func TestManagerInstallCreatesBundleStateAndVersionedBundleDir(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager, storage, userDir := newTestManager(t, false)
	defer storage.Close()

	repoDir := t.TempDir()
	writePlugin(t, repoDir, "bundleme", "BundleMe", "0.1.0", `return {}`)
	gitCommitAll(t, repoDir, "initial")

	if _, err := manager.UpsertSource(ctx, marketplace.SourceUpsert{
		SourceID: "demo",
		GitURL:   repoDir,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("UpsertSource: %v", err)
	}

	install, err := manager.Install(ctx, marketplace.InstallRequest{
		SourceID: "demo",
		PluginID: "bundleme",
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	entryDir := filepath.Join(userDir, "bundleme")
	if install.PluginRoot != entryDir {
		t.Fatalf("expected install plugin_root to stay on plugin root, got %q want %q", install.PluginRoot, entryDir)
	}
	if install.BundleRelativeDir == "" {
		t.Fatal("expected install result to include bundle_relative_dir")
	}

	if _, err := os.Stat(filepath.Join(entryDir, "plugin.json")); err == nil {
		t.Fatalf("expected plugin root %q to stop being the live plugin directory", entryDir)
	}
	resolvedDir, err := bundles.NewLocalRepository().ResolveActiveDir(entryDir)
	if err != nil {
		t.Fatalf("ResolveActiveDir: %v", err)
	}
	if resolvedDir != filepath.Join(entryDir, install.BundleRelativeDir) {
		t.Fatalf("expected bundle state to resolve to bundle_relative_dir, got %q want %q", resolvedDir, filepath.Join(entryDir, install.BundleRelativeDir))
	}
	stored, ok, err := storage.PluginInstalls().GetPluginInstall(ctx, "bundleme")
	if err != nil {
		t.Fatalf("GetPluginInstall: %v", err)
	}
	if !ok {
		t.Fatal("expected plugin install record to exist")
	}
	if stored.BundleRelativeDir != install.BundleRelativeDir {
		t.Fatalf("expected stored bundle_relative_dir %q to match install result %q", stored.BundleRelativeDir, install.BundleRelativeDir)
	}
}

func TestManagerInstallWithCachedRepositoryStoresArtifactOutsidePluginRoot(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	tmp := t.TempDir()
	repo, err := bundles.NewCachedRepository(bundles.CachedRepositoryOptions{
		StoreDir: filepath.Join(tmp, "bundle-store"),
		CacheDir: filepath.Join(tmp, "bundle-cache"),
	})
	if err != nil {
		t.Fatalf("NewCachedRepository: %v", err)
	}

	manager, storage, userDir := newTestManager(t, false, repo)
	defer storage.Close()

	repoDir := t.TempDir()
	writePlugin(t, repoDir, "bundleme", "BundleMe", "0.1.0", `return {}`)
	gitCommitAll(t, repoDir, "initial")

	if _, err := manager.UpsertSource(ctx, marketplace.SourceUpsert{
		SourceID: "demo",
		GitURL:   repoDir,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("UpsertSource: %v", err)
	}

	install, err := manager.Install(ctx, marketplace.InstallRequest{
		SourceID: "demo",
		PluginID: "bundleme",
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	entryDir := filepath.Join(userDir, "bundleme")
	if install.PluginRoot != entryDir {
		t.Fatalf("expected install plugin_root to stay on plugin root, got %q want %q", install.PluginRoot, entryDir)
	}
	if _, err := os.Stat(filepath.Join(entryDir, "plugin.json")); err == nil {
		t.Fatalf("expected plugin root %q to remain metadata-only", entryDir)
	}
	activeDir, err := repo.ResolveActiveDir(entryDir)
	if err != nil {
		t.Fatalf("ResolveActiveDir: %v", err)
	}
	if !strings.Contains(activeDir, string(filepath.Separator)+"bundle-cache"+string(filepath.Separator)) {
		t.Fatalf("expected active dir under bundle cache, got %q", activeDir)
	}
	if activeDir == filepath.Join(entryDir, install.BundleRelativeDir) {
		t.Fatalf("expected active dir and plugin-root bundle path to differ for cached repository, both were %q", activeDir)
	}
}

func TestManagerInstallWithObjectStoreRepositoryStoresArtifactInArtifactCache(t *testing.T) {
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

	manager, storage, userDir := newTestManager(t, false, repo)
	defer storage.Close()

	repoDir := t.TempDir()
	writePlugin(t, repoDir, "bundleme", "BundleMe", "0.1.0", `return {}`)
	gitCommitAll(t, repoDir, "initial")

	if _, err := manager.UpsertSource(ctx, marketplace.SourceUpsert{
		SourceID: "demo",
		GitURL:   repoDir,
		Enabled:  true,
	}); err != nil {
		t.Fatalf("UpsertSource: %v", err)
	}

	install, err := manager.Install(ctx, marketplace.InstallRequest{
		SourceID: "demo",
		PluginID: "bundleme",
	})
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	entryDir := filepath.Join(userDir, "bundleme")
	if install.PluginRoot != entryDir {
		t.Fatalf("expected install plugin_root to stay on plugin root, got %q want %q", install.PluginRoot, entryDir)
	}
	if _, err := os.Stat(filepath.Join(entryDir, "plugin.json")); err == nil {
		t.Fatalf("expected plugin root %q to remain metadata-only", entryDir)
	}
	activeDir, err := repo.ResolveActiveDir(entryDir)
	if err != nil {
		t.Fatalf("ResolveActiveDir: %v", err)
	}
	if !strings.Contains(activeDir, string(filepath.Separator)+"bundle-cache"+string(filepath.Separator)+"active"+string(filepath.Separator)) {
		t.Fatalf("expected active dir under active cache, got %q", activeDir)
	}
	if activeDir == filepath.Join(entryDir, install.BundleRelativeDir) {
		t.Fatalf("expected active dir and plugin-root bundle path to differ for object-store repository, both were %q", activeDir)
	}
	if _, err := os.Stat(filepath.Join(tmp, "object-store", "bundleme")); err != nil {
		t.Fatalf("expected object-store plugin root to exist: %v", err)
	}
}

func newTestManager(t *testing.T, prod bool, bundleRepo ...bundles.Repository) (*marketplace.Manager, *postgresstore.Store, string) {
	t.Helper()

	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	db := postgrestest.OpenMigratedDB(t)
	storage, err := postgresstore.New(db)
	if err != nil {
		t.Fatalf("postgresstore.New: %v", err)
	}
	var repo bundles.Repository
	if len(bundleRepo) > 0 {
		repo = bundleRepo[0]
	}
	manager, err := marketplace.New(marketplace.Options{
		Logger:            slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
		Store:             storage,
		Bundles:           repo,
		BundledPluginsDir: filepath.Join(tmp, "bundled"),
		UserPluginsDir:    userDir,
		TrustedKeysFile:   filepath.Join(tmp, "trusted_keys.json"),
		CacheDir:          filepath.Join(tmp, "cache"),
		ProdMode:          prod,
		AllowUnsigned:     false,
	})
	if err != nil {
		t.Fatalf("marketplace.New: %v", err)
	}
	return manager, storage, userDir
}

func writePlugin(t *testing.T, repoRoot, pluginID, name, version, pluginLua string) {
	t.Helper()

	dir := filepath.Join(repoRoot, pluginID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	manifest := fmt.Sprintf(`{"id":"%s","name":"%s","version":"%s"}`, pluginID, name, version)
	if err := os.WriteFile(filepath.Join(dir, "plugin.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.json): %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugin.lua"), []byte(pluginLua), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.lua): %v", err)
	}
}

func gitCommitAll(t *testing.T, dir, message string) {
	t.Helper()
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "tests@example.com")
	runGit(t, dir, "config", "user.name", "Tests")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", message)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

type ioDiscard struct{}

func (ioDiscard) Write(p []byte) (int, error) { return len(p), nil }
