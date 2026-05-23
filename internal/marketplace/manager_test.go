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

	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	migrate "github.com/xsyetopz/go-mamacord/internal/migration"
	"github.com/xsyetopz/go-mamacord/internal/sqlite"
	sqlitestore "github.com/xsyetopz/go-mamacord/internal/storage/sqlite"
)

func TestManagerInstallAndForceUpdate(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	manager, storage, userDir := newTestManager(t, false)
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

	targetDir := filepath.Join(userDir, "weather")
	if err := os.WriteFile(filepath.Join(targetDir, "plugin.lua"), []byte(`return { changed = true }`), 0o644); err != nil {
		t.Fatalf("WriteFile(local change): %v", err)
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

func newTestManager(t *testing.T, prod bool) (*marketplace.Manager, *sqlitestore.Store, string) {
	t.Helper()

	ctx := context.Background()
	tmp := t.TempDir()
	userDir := filepath.Join(tmp, "user")
	dbPath := filepath.Join(tmp, "marketplace.sqlite")
	runner, err := migrate.New(migrate.Options{
		Dir:       filepath.Join("..", "..", "migrations", "sqlite"),
		BackupDir: filepath.Join(tmp, "backups"),
	})
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}
	if _, err := runner.UpPath(ctx, dbPath); err != nil {
		t.Fatalf("UpPath: %v", err)
	}
	db, err := sqlite.Open(ctx, sqlite.Options{Path: dbPath})
	if err != nil {
		t.Fatalf("sqlite.Open: %v", err)
	}
	storage, err := sqlitestore.New(db)
	if err != nil {
		t.Fatalf("sqlitestore.New: %v", err)
	}
	manager, err := marketplace.New(marketplace.Options{
		Logger:            slog.New(slog.NewTextHandler(ioDiscard{}, nil)),
		Store:             storage,
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
