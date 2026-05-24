package bundles_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
)

func TestLocalRepositoryMaterializeBundleWritesStateAndResolvesActiveDir(t *testing.T) {
	t.Parallel()

	repo := bundles.NewLocalRepository()
	srcDir := filepath.Join(t.TempDir(), "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(srcDir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "plugin.json"), []byte(`{"id":"sample","name":"Sample","version":"0.1.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.json): %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "plugin.lua"), []byte(`return {}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.lua): %v", err)
	}

	rootDir := filepath.Join(t.TempDir(), "plugins", "sample")
	bundle, err := repo.MaterializeBundle(srcDir, rootDir, "release-v0.1.0")
	if err != nil {
		t.Fatalf("MaterializeBundle: %v", err)
	}
	if !strings.Contains(bundle.BundleDir, string(filepath.Separator)+"bundles"+string(filepath.Separator)) {
		t.Fatalf("expected materialized bundle dir under bundles/, got %q", bundle.BundleDir)
	}
	if bundle.BundleRelativeDir == "" {
		t.Fatal("expected materialized bundle to include bundle_relative_dir")
	}

	state, err := repo.ReadState(rootDir)
	if err != nil {
		t.Fatalf("ReadState: %v", err)
	}
	if state.ActiveRelativeDir != bundle.BundleRelativeDir {
		t.Fatalf("unexpected active_relative_dir: got %q want %q", state.ActiveRelativeDir, bundle.BundleRelativeDir)
	}

	resolvedDir, err := repo.ResolveActiveDir(rootDir)
	if err != nil {
		t.Fatalf("ResolveActiveDir: %v", err)
	}
	if resolvedDir != bundle.BundleDir {
		t.Fatalf("unexpected active dir: got %q want %q", resolvedDir, bundle.BundleDir)
	}
}

func TestLocalRepositoryListsAndRemovesPluginRoots(t *testing.T) {
	t.Parallel()

	repo := bundles.NewLocalRepository()
	pluginsRoot := filepath.Join(t.TempDir(), "plugins")
	if err := os.MkdirAll(filepath.Join(pluginsRoot, "alpha"), 0o755); err != nil {
		t.Fatalf("MkdirAll(alpha): %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginsRoot, "README.txt"), []byte("ignore"), 0o644); err != nil {
		t.Fatalf("WriteFile(README.txt): %v", err)
	}

	roots, err := repo.ListPluginRoots(pluginsRoot)
	if err != nil {
		t.Fatalf("ListPluginRoots: %v", err)
	}
	if len(roots) != 1 || roots[0].Name != "alpha" {
		t.Fatalf("unexpected plugin roots: %#v", roots)
	}

	if err := repo.RemovePluginRoot(filepath.Join(pluginsRoot, "alpha")); err != nil {
		t.Fatalf("RemovePluginRoot: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pluginsRoot, "alpha")); !os.IsNotExist(err) {
		t.Fatalf("expected plugin root removal, got err=%v", err)
	}
}
