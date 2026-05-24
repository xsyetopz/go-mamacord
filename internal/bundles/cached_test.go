package bundles_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
)

func TestCachedRepositoryMaterializeBundleStoresArtifactAndResolvesActiveCache(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	repo, err := bundles.NewCachedRepository(bundles.CachedRepositoryOptions{
		StoreDir: filepath.Join(tmp, "bundle-store"),
		CacheDir: filepath.Join(tmp, "bundle-cache"),
	})
	if err != nil {
		t.Fatalf("NewCachedRepository: %v", err)
	}

	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(srcDir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "plugin.json"), []byte(`{"id":"sample","name":"Sample","version":"0.1.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.json): %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "plugin.lua"), []byte(`return {}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.lua): %v", err)
	}

	rootDir := filepath.Join(tmp, "plugins", "sample")
	bundle, err := repo.MaterializeBundle(srcDir, rootDir, "release-v0.1.0")
	if err != nil {
		t.Fatalf("MaterializeBundle: %v", err)
	}
	if !strings.Contains(bundle.BundleDir, string(filepath.Separator)+"bundle-store"+string(filepath.Separator)) {
		t.Fatalf("expected bundle dir under bundle store, got %q", bundle.BundleDir)
	}
	if !strings.Contains(bundle.ActiveDir, string(filepath.Separator)+"bundle-cache"+string(filepath.Separator)) {
		t.Fatalf("expected active dir under bundle cache, got %q", bundle.ActiveDir)
	}
	if bundle.BundleDir == bundle.ActiveDir {
		t.Fatalf("expected cached repository to separate artifact and active dirs, got %q", bundle.BundleDir)
	}
	if _, err := os.Stat(filepath.Join(rootDir, "bundles", "git-release-v0.1.0", "plugin.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no live bundle materialized under plugin root, err=%v", err)
	}

	resolvedBundleDir, err := repo.ResolveBundleDir(rootDir)
	if err != nil {
		t.Fatalf("ResolveBundleDir: %v", err)
	}
	if resolvedBundleDir != bundle.BundleDir {
		t.Fatalf("unexpected bundle dir: got %q want %q", resolvedBundleDir, bundle.BundleDir)
	}

	resolvedActiveDir, err := repo.ResolveActiveDir(rootDir)
	if err != nil {
		t.Fatalf("ResolveActiveDir: %v", err)
	}
	if resolvedActiveDir != bundle.ActiveDir {
		t.Fatalf("unexpected active dir: got %q want %q", resolvedActiveDir, bundle.ActiveDir)
	}
}

func TestCachedRepositoryResolveActiveDirUsesRootBundleAsSource(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	repo, err := bundles.NewCachedRepository(bundles.CachedRepositoryOptions{
		StoreDir: filepath.Join(tmp, "bundle-store"),
		CacheDir: filepath.Join(tmp, "bundle-cache"),
	})
	if err != nil {
		t.Fatalf("NewCachedRepository: %v", err)
	}

	rootDir := filepath.Join(tmp, "plugins", "sample")
	bundleRel := filepath.Join("bundles", "release-v0.1.0")
	bundleDir := filepath.Join(rootDir, bundleRel)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(bundleDir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "plugin.json"), []byte(`{"id":"sample","name":"Sample","version":"0.1.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.json): %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "plugin.lua"), []byte(`return {}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.lua): %v", err)
	}
	if err := repo.WriteState(rootDir, bundles.State{
		ActiveRelativeDir: bundleRel,
		Revision:          "release-v0.1.0",
	}); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	resolvedBundleDir, err := repo.ResolveBundleDir(rootDir)
	if err != nil {
		t.Fatalf("ResolveBundleDir: %v", err)
	}
	if resolvedBundleDir != bundleDir {
		t.Fatalf("expected root bundle dir to remain the artifact source, got %q want %q", resolvedBundleDir, bundleDir)
	}

	resolvedActiveDir, err := repo.ResolveActiveDir(rootDir)
	if err != nil {
		t.Fatalf("ResolveActiveDir: %v", err)
	}
	if resolvedActiveDir == bundleDir {
		t.Fatalf("expected active dir to materialize into cache, got source dir %q", resolvedActiveDir)
	}
	if !strings.Contains(resolvedActiveDir, string(filepath.Separator)+"bundle-cache"+string(filepath.Separator)) {
		t.Fatalf("expected active dir under bundle cache, got %q", resolvedActiveDir)
	}
	if _, err := os.Stat(filepath.Join(resolvedActiveDir, "plugin.json")); err != nil {
		t.Fatalf("expected cached plugin.json: %v", err)
	}
}
