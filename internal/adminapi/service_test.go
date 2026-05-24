package adminapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/config"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

func TestScaffoldPluginCreatesExpectedFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	svc := Service{
		Config: config.Config{
			UserPluginsDir: dir,
		},
	}

	resp, err := svc.ScaffoldPlugin(PluginScaffoldRequest{
		ID:                 "sample",
		Name:               "Sample",
		Version:            "0.1.0",
		Locale:             "en-US",
		CommandName:        "sample",
		CommandDescription: "Sample command",
		ResponseMessage:    "Hello from Sample.",
	})
	if err != nil {
		t.Fatalf("ScaffoldPlugin: %v", err)
	}
	if resp.ID != "sample" {
		t.Fatalf("unexpected id: %q", resp.ID)
	}

	for _, rel := range []string{
		bundles.StateFileName,
		filepath.Join("bundles", "git-manual-v0.1.0", "plugin.json"),
		filepath.Join("bundles", "git-manual-v0.1.0", "plugin.lua"),
		filepath.Join("bundles", "git-manual-v0.1.0", "commands", "hello.lua"),
		filepath.Join("bundles", "git-manual-v0.1.0", "locales", "en-US", "messages.json"),
	} {
		if _, err := os.Stat(filepath.Join(dir, "sample", rel)); err != nil {
			t.Fatalf("expected file %q: %v", rel, err)
		}
	}

	if _, err := os.Stat(filepath.Join(dir, "sample", "plugin.json")); err == nil {
		t.Fatalf("did not expect live plugin root manifest at %q", filepath.Join(dir, "sample", "plugin.json"))
	}

	bytes, err := os.ReadFile(filepath.Join(dir, "sample", "bundles", "git-manual-v0.1.0", "plugin.json"))
	if err != nil {
		t.Fatalf("ReadFile(plugin.json): %v", err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(bytes, &manifest); err != nil {
		t.Fatalf("json.Unmarshal(plugin.json): %v", err)
	}
	if got, _ := manifest["id"].(string); got != "sample" {
		t.Fatalf("unexpected manifest id: %q", got)
	}
}

func TestSignPluginUsesResolvedBundleDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	keyPath := filepath.Join(dir, "signing.key")
	_, privateKey, err := pluginhost.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}
	if err := pluginhost.WriteEd25519PrivateKeyFile(keyPath, privateKey); err != nil {
		t.Fatalf("WriteEd25519PrivateKeyFile: %v", err)
	}

	svc := Service{
		Config: config.Config{
			UserPluginsDir:          dir,
			DashboardSigningKeyFile: keyPath,
			DashboardSigningKeyID:   "test-key",
			TrustedKeysFile:         filepath.Join(dir, "trusted_keys.json"),
		},
	}

	if _, err := svc.ScaffoldPlugin(PluginScaffoldRequest{
		ID:                 "sample",
		Name:               "Sample",
		Version:            "0.1.0",
		Locale:             "en-US",
		CommandName:        "sample",
		CommandDescription: "Sample command",
		ResponseMessage:    "Hello from Sample.",
	}); err != nil {
		t.Fatalf("ScaffoldPlugin: %v", err)
	}

	signaturePath, err := svc.SignPlugin("sample")
	if err != nil {
		t.Fatalf("SignPlugin: %v", err)
	}
	want := filepath.Join(dir, "sample", "bundles", "git-manual-v0.1.0", "signature.json")
	if signaturePath != want {
		t.Fatalf("unexpected signature path: got %q want %q", signaturePath, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected signature file at %q: %v", want, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sample", "signature.json")); err == nil {
		t.Fatalf("did not expect signature at plugin root")
	}
}

func TestSignPluginUsesConfiguredBundleRepositoryArtifactDir(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	repo, err := bundles.NewCachedRepository(bundles.CachedRepositoryOptions{
		StoreDir: filepath.Join(dir, "bundle-store"),
		CacheDir: filepath.Join(dir, "bundle-cache"),
	})
	if err != nil {
		t.Fatalf("NewCachedRepository: %v", err)
	}

	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(srcDir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "plugin.json"), []byte(`{"id":"sample","name":"sample","version":"0.1.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.json): %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "plugin.lua"), []byte(`return {}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.lua): %v", err)
	}

	pluginRoot := filepath.Join(dir, "plugins", "sample")
	bundle, err := repo.MaterializeBundle(srcDir, pluginRoot, "release-v0.1.0")
	if err != nil {
		t.Fatalf("MaterializeBundle: %v", err)
	}

	keyPath := filepath.Join(dir, "signing.key")
	_, privateKey, err := pluginhost.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}
	if err := pluginhost.WriteEd25519PrivateKeyFile(keyPath, privateKey); err != nil {
		t.Fatalf("WriteEd25519PrivateKeyFile: %v", err)
	}

	svc := Service{
		Config: config.Config{
			UserPluginsDir:          filepath.Join(dir, "plugins"),
			DashboardSigningKeyFile: keyPath,
			DashboardSigningKeyID:   "test-key",
			TrustedKeysFile:         filepath.Join(dir, "trusted_keys.json"),
		},
		Bundles: repo,
	}

	signaturePath, err := svc.SignPlugin("sample")
	if err != nil {
		t.Fatalf("SignPlugin: %v", err)
	}
	want := filepath.Join(bundle.BundleDir, "signature.json")
	if signaturePath != want {
		t.Fatalf("unexpected signature path: got %q want %q", signaturePath, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected signature file at artifact dir %q: %v", want, err)
	}
	if _, err := os.Stat(filepath.Join(bundle.ActiveDir, "signature.json")); err != nil {
		t.Fatalf("expected signature in active cache dir: %v", err)
	}
}

func TestSignPluginUsesObjectStoreBundleRepositoryAndPersistsSignature(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	storeBackend, err := bundles.NewDirObjectStore(filepath.Join(dir, "object-store"))
	if err != nil {
		t.Fatalf("NewDirObjectStore: %v", err)
	}
	repo, err := bundles.NewObjectStoreRepository(bundles.ObjectStoreRepositoryOptions{
		Store:    storeBackend,
		CacheDir: filepath.Join(dir, "bundle-cache"),
	})
	if err != nil {
		t.Fatalf("NewObjectStoreRepository: %v", err)
	}

	srcDir := filepath.Join(dir, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(srcDir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "plugin.json"), []byte(`{"id":"sample","name":"sample","version":"0.1.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.json): %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "plugin.lua"), []byte(`return {}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.lua): %v", err)
	}

	pluginRoot := filepath.Join(dir, "plugins", "sample")
	bundle, err := repo.MaterializeBundle(srcDir, pluginRoot, "release-v0.1.0")
	if err != nil {
		t.Fatalf("MaterializeBundle: %v", err)
	}

	keyPath := filepath.Join(dir, "signing.key")
	_, privateKey, err := pluginhost.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}
	if err := pluginhost.WriteEd25519PrivateKeyFile(keyPath, privateKey); err != nil {
		t.Fatalf("WriteEd25519PrivateKeyFile: %v", err)
	}

	svc := Service{
		Config: config.Config{
			UserPluginsDir:          filepath.Join(dir, "plugins"),
			DashboardSigningKeyFile: keyPath,
			DashboardSigningKeyID:   "test-key",
			TrustedKeysFile:         filepath.Join(dir, "trusted_keys.json"),
		},
		Bundles: repo,
	}

	signaturePath, err := svc.SignPlugin("sample")
	if err != nil {
		t.Fatalf("SignPlugin: %v", err)
	}
	want := filepath.Join(bundle.BundleDir, "signature.json")
	if signaturePath != want {
		t.Fatalf("unexpected signature path: got %q want %q", signaturePath, want)
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatalf("expected signature file at artifact cache dir %q: %v", want, err)
	}
	if _, err := os.Stat(filepath.Join(bundle.ActiveDir, "signature.json")); err != nil {
		t.Fatalf("expected signature in active cache dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "object-store", "sample", "bundles", "git-release-v0.1.0", "signature.json")); err != nil {
		t.Fatalf("expected signature persisted to object-store artifact: %v", err)
	}
}

func TestSetupResponseDisablesAdminWhenControlRoleMissing(t *testing.T) {
	t.Parallel()

	svc := Service{
		Config: config.Config{
			AdminAddr:       "127.0.0.1:8081",
			RuntimeRoles:    []config.RuntimeRole{config.RuntimeRoleGateway},
			OwnerUserID:     nil,
			ProdMode:        false,
			TrustedKeysFile: "",
		},
	}

	resp := svc.setupResponse(true)
	if resp.AdminEnabled {
		t.Fatal("expected admin setup response to disable admin when the control role is not enabled")
	}
	if !strings.Contains(strings.Join(resp.Hints, "\n"), "MAMACORD_RUNTIME_ROLES") {
		t.Fatalf("expected setup hints to mention MAMACORD_RUNTIME_ROLES when control role is missing, got %#v", resp.Hints)
	}
}
