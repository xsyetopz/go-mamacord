package bundles_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
)

func TestObjectStoreRepositoryMaterializeBundleStoresObjectAndResolvesArtifactAndActiveCaches(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	store, err := bundles.NewDirObjectStore(filepath.Join(tmp, "object-store"))
	if err != nil {
		t.Fatalf("NewDirObjectStore: %v", err)
	}
	repo, err := bundles.NewObjectStoreRepository(bundles.ObjectStoreRepositoryOptions{
		Store:    store,
		CacheDir: filepath.Join(tmp, "bundle-cache"),
	})
	if err != nil {
		t.Fatalf("NewObjectStoreRepository: %v", err)
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
	if !strings.Contains(bundle.BundleDir, string(filepath.Separator)+"bundle-cache"+string(filepath.Separator)+"artifacts"+string(filepath.Separator)) {
		t.Fatalf("expected bundle dir under artifact cache, got %q", bundle.BundleDir)
	}
	if !strings.Contains(bundle.ActiveDir, string(filepath.Separator)+"bundle-cache"+string(filepath.Separator)+"active"+string(filepath.Separator)) {
		t.Fatalf("expected active dir under active cache, got %q", bundle.ActiveDir)
	}
	if bundle.BundleDir == bundle.ActiveDir {
		t.Fatalf("expected separate artifact and active dirs, got %q", bundle.BundleDir)
	}
	if _, err := os.Stat(filepath.Join(rootDir, "bundles", "git-release-v0.1.0", "plugin.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no live bundle under plugin root, err=%v", err)
	}
	if _, err := os.Stat(filepath.Join(tmp, "object-store", "sample", "bundles", "git-release-v0.1.0", "plugin.json")); err != nil {
		t.Fatalf("expected object-store bundle contents: %v", err)
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

func TestObjectStoreRepositoryResolveBundleDirUsesRootBundleWhenPresent(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	store, err := bundles.NewDirObjectStore(filepath.Join(tmp, "object-store"))
	if err != nil {
		t.Fatalf("NewDirObjectStore: %v", err)
	}
	repo, err := bundles.NewObjectStoreRepository(bundles.ObjectStoreRepositoryOptions{
		Store:    store,
		CacheDir: filepath.Join(tmp, "bundle-cache"),
	})
	if err != nil {
		t.Fatalf("NewObjectStoreRepository: %v", err)
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
		t.Fatalf("expected local root bundle dir to win, got %q want %q", resolvedBundleDir, bundleDir)
	}
}

func TestObjectStoreRepositoryInspectPreferredOrActiveBundleUsesPreferredActiveCache(t *testing.T) {
	t.Parallel()

	tmp := t.TempDir()
	store, err := bundles.NewDirObjectStore(filepath.Join(tmp, "object-store"))
	if err != nil {
		t.Fatalf("NewDirObjectStore: %v", err)
	}
	repo, err := bundles.NewObjectStoreRepository(bundles.ObjectStoreRepositoryOptions{
		Store:    store,
		CacheDir: filepath.Join(tmp, "bundle-cache"),
	})
	if err != nil {
		t.Fatalf("NewObjectStoreRepository: %v", err)
	}

	rootDir := filepath.Join(tmp, "plugins", "sample")
	oldSrcDir := filepath.Join(tmp, "old-src")
	newSrcDir := filepath.Join(tmp, "new-src")
	for _, item := range []struct {
		dir     string
		version string
	}{
		{dir: oldSrcDir, version: "0.1.0"},
		{dir: newSrcDir, version: "0.2.0"},
	} {
		if err := os.MkdirAll(item.dir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%q): %v", item.dir, err)
		}
		if err := os.WriteFile(filepath.Join(item.dir, "plugin.json"), []byte(`{"id":"sample","name":"Sample","version":"`+item.version+`"}`), 0o644); err != nil {
			t.Fatalf("WriteFile(plugin.json): %v", err)
		}
		if err := os.WriteFile(filepath.Join(item.dir, "plugin.lua"), []byte(`return {}`), 0o644); err != nil {
			t.Fatalf("WriteFile(plugin.lua): %v", err)
		}
	}

	oldBundle, err := repo.MaterializeBundle(oldSrcDir, rootDir, "git-old")
	if err != nil {
		t.Fatalf("MaterializeBundle(old): %v", err)
	}
	newBundle, err := repo.MaterializeBundle(newSrcDir, rootDir, "git-new")
	if err != nil {
		t.Fatalf("MaterializeBundle(new): %v", err)
	}
	if err := repo.WriteState(rootDir, bundles.State{
		ActiveRelativeDir: oldBundle.BundleRelativeDir,
		Revision:          "old",
		HashB64:           oldBundle.HashB64,
	}); err != nil {
		t.Fatalf("WriteState(old): %v", err)
	}

	inspection, err := bundles.InspectPreferredOrActiveBundle(repo, rootDir, newBundle.BundleRelativeDir)
	if err != nil {
		t.Fatalf("InspectPreferredOrActiveBundle: %v", err)
	}
	if inspection.BundleRelativeDir != newBundle.BundleRelativeDir {
		t.Fatalf("expected preferred bundle relative dir %q, got %q", newBundle.BundleRelativeDir, inspection.BundleRelativeDir)
	}
	if inspection.BundleDir != newBundle.BundleDir {
		t.Fatalf("expected bundle artifact dir %q, got %q", newBundle.BundleDir, inspection.BundleDir)
	}
	if inspection.LoadDir != newBundle.ActiveDir {
		t.Fatalf("expected preferred active cache dir %q, got %q", newBundle.ActiveDir, inspection.LoadDir)
	}
	if strings.Contains(inspection.LoadDir, string(filepath.Separator)+"object-store"+string(filepath.Separator)) {
		t.Fatalf("expected inspected load dir to avoid object-store contents, got %q", inspection.LoadDir)
	}
	if string(inspection.ManifestBytes) != `{"id":"sample","name":"Sample","version":"0.2.0"}` {
		t.Fatalf("unexpected manifest bytes: %s", string(inspection.ManifestBytes))
	}
}
