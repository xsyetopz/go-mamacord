package main

import (
	"crypto/ed25519"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

func TestRunDoctorCommand_PrintsSigningDiagnostics(t *testing.T) {
	resetMainEnv(t)
	t.Setenv("DISCORD_TOKEN", "discord-token")
	t.Setenv("MAMACORD_PROD_MODE", "1")
	t.Setenv("MAMACORD_ALLOW_UNSIGNED_PLUGINS", "0")
	t.Setenv("MAMACORD_TRUSTED_KEYS_FILE", "/tmp/trusted_keys.json")
	t.Setenv("MAMACORD_DASHBOARD_SIGNING_KEY_ID", "owner")
	t.Setenv("MAMACORD_DASHBOARD_SIGNING_KEY_FILE", "./data/keys/owner.key")
	t.Setenv("MAMACORD_LOADED_ENV_FILE", ".env.prod")
	t.Setenv("MAMACORD_LOADED_ENV_SOURCE", "working_dir")

	output := captureStdout(t, func() {
		if code := runDoctorCommand(nil); code != 0 {
			t.Fatalf("runDoctorCommand returned %d", code)
		}
	})

	for _, want := range []string{
		"env_file_loaded: .env.prod",
		"discord_token: true",
		"runtime_roles: control,gateway,scheduler",
		"control_role_enabled: true",
		"gateway_role_enabled: true",
		"scheduler_role_enabled: true",
		"prod_mode: true",
		"allow_unsigned_plugins: false",
		"trusted_keys_file: /tmp/trusted_keys.json",
		"trusted_keys_file_exists: false",
		"trusted_keys_count_file: 0",
		"dashboard_signing_configured: true",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output missing %q\n%s", want, output)
		}
	}
}

func TestRunDoctorCommand_ControlOnlyDoesNotRequireDiscordToken(t *testing.T) {
	resetMainEnv(t)
	t.Setenv("MAMACORD_RUNTIME_ROLES", "control")

	output := captureStdout(t, func() {
		if code := runDoctorCommand(nil); code != 0 {
			t.Fatalf("runDoctorCommand returned %d", code)
		}
	})

	for _, want := range []string{
		"discord_token: false",
		"runtime_roles: control",
		"control_role_enabled: true",
		"gateway_role_enabled: false",
		"scheduler_role_enabled: false",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output missing %q\n%s", want, output)
		}
	}
	if strings.Contains(output, "next: set DISCORD_TOKEN to start the bot") {
		t.Fatalf("doctor output should not demand DISCORD_TOKEN for control-only runtime\n%s", output)
	}
}

func TestRunDoctorCommand_GatewayAndSchedulerRolesDisableAdminWithoutControl(t *testing.T) {
	resetMainEnv(t)
	t.Setenv("DISCORD_TOKEN", "discord-token")
	t.Setenv("MAMACORD_ADMIN_ADDR", "127.0.0.1:8081")

	for _, tc := range []struct {
		name  string
		roles string
	}{
		{name: "gateway-only", roles: "gateway"},
		{name: "scheduler-only", roles: "scheduler"},
		{name: "gateway-and-scheduler", roles: "gateway,scheduler"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resetMainEnv(t)
			t.Setenv("DISCORD_TOKEN", "discord-token")
			t.Setenv("MAMACORD_ADMIN_ADDR", "127.0.0.1:8081")
			t.Setenv("MAMACORD_RUNTIME_ROLES", tc.roles)

			output := captureStdout(t, func() {
				if code := runDoctorCommand(nil); code != 0 {
					t.Fatalf("runDoctorCommand returned %d", code)
				}
			})

			for _, want := range []string{
				"runtime_roles: " + tc.roles,
				"admin_api_enabled: false",
			} {
				if !strings.Contains(output, want) {
					t.Fatalf("doctor output missing %q\n%s", want, output)
				}
			}
			if strings.Contains(output, "setup_url:") {
				t.Fatalf("doctor output should not expose setup_url when the control role is disabled\n%s", output)
			}
		})
	}
}

func TestRunDoctorCommand_PrintsPostgresStorageDiagnostics(t *testing.T) {
	resetMainEnv(t)
	t.Setenv("DISCORD_TOKEN", "discord-token")
	t.Setenv("MAMACORD_STORAGE_BACKEND", "postgres")
	t.Setenv("MAMACORD_POSTGRES_DSN", "postgres://bot:secret@db:5432/mamacord?sslmode=disable")

	output := captureStdout(t, func() {
		if code := runDoctorCommand(nil); code != 0 {
			t.Fatalf("runDoctorCommand returned %d", code)
		}
	})

	for _, want := range []string{
		"storage_backend: postgres",
		"storage_target: postgres://bot:***@db:5432/mamacord?sslmode=disable",
		"migrations_dir: ./migrations/postgres",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("doctor output missing %q\n%s", want, output)
		}
	}
}

func TestRunInitCommand_ProdWritesPostgresDefaults(t *testing.T) {
	resetMainEnv(t)

	dir := t.TempDir()
	originalWD, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir(%q): %v", dir, err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(originalWD)
	})

	output := captureStdout(t, func() {
		if code := runInitCommand([]string{"--mode", "prod"}); code != 0 {
			t.Fatalf("runInitCommand returned %d", code)
		}
	})

	content, err := os.ReadFile(filepath.Join(dir, ".env.prod"))
	if err != nil {
		t.Fatalf("ReadFile(.env.prod): %v", err)
	}
	for _, want := range []string{
		"MAMACORD_STORAGE_BACKEND=postgres",
		"MAMACORD_POSTGRES_DSN=postgres://mamacord:secret@127.0.0.1:5432/mamacord?sslmode=disable",
	} {
		if !strings.Contains(string(content), want) {
			t.Fatalf(".env.prod missing %q\n%s", want, string(content))
		}
	}
	if !strings.Contains(output, "wrote: .env.prod") {
		t.Fatalf("init output missing env path\n%s", output)
	}
}

func TestRunGenSigningKeyCommand_CreatesFiles(t *testing.T) {
	resetMainEnv(t)

	dir := t.TempDir()
	privateKeyPath := filepath.Join(dir, "data", "keys", "owner.key")
	trustedKeysPath := filepath.Join(dir, "config", "trusted_keys.json")

	output := captureStdout(t, func() {
		code := runGenSigningKeyCommand([]string{
			"--key-id", "owner",
			"--private-key-file", privateKeyPath,
			"--trusted-keys-file", trustedKeysPath,
		})
		if code != 0 {
			t.Fatalf("runGenSigningKeyCommand returned %d", code)
		}
	})

	if _, err := os.Stat(privateKeyPath); err != nil {
		t.Fatalf("private key missing: %v", err)
	}
	if _, err := os.Stat(trustedKeysPath); err != nil {
		t.Fatalf("trusted keys file missing: %v", err)
	}

	for _, want := range []string{
		"key_id: owner",
		"private_key_file: " + privateKeyPath,
		"trusted_keys_file: " + trustedKeysPath,
		"public_key_b64: ",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("keygen output missing %q\n%s", want, output)
		}
	}
}

func TestRunSignPluginCommand_ResolvesActiveBundleDir(t *testing.T) {
	resetMainEnv(t)

	dir := t.TempDir()
	pluginRoot := filepath.Join(dir, "plugins", "sample")
	bundleRel := filepath.Join("bundles", "release-v0.1.0")
	bundleDir := filepath.Join(pluginRoot, bundleRel)
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(bundleDir): %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "plugin.json"), []byte(`{"id":"sample","name":"sample","version":"0.1.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.json): %v", err)
	}
	if err := os.WriteFile(filepath.Join(bundleDir, "plugin.lua"), []byte(`return {}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.lua): %v", err)
	}
	if err := bundles.NewLocalRepository().WriteState(pluginRoot, bundles.State{
		ActiveRelativeDir: bundleRel,
		Revision:          "release-v0.1.0",
	}); err != nil {
		t.Fatalf("WriteState: %v", err)
	}

	publicKey, privateKey, err := pluginhost.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}
	privateKeyPath := filepath.Join(dir, "keys", "owner.key")
	if err := pluginhost.WriteEd25519PrivateKeyFile(privateKeyPath, privateKey); err != nil {
		t.Fatalf("WriteEd25519PrivateKeyFile: %v", err)
	}

	output := captureStdout(t, func() {
		code := runSignPluginCommand([]string{
			"--dir", pluginRoot,
			"--key-id", "owner",
			"--private-key-file", privateKeyPath,
		})
		if code != 0 {
			t.Fatalf("runSignPluginCommand returned %d", code)
		}
	})

	signaturePath := filepath.Join(bundleDir, "signature.json")
	if _, err := os.Stat(signaturePath); err != nil {
		t.Fatalf("expected signature in active bundle dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(pluginRoot, "signature.json")); !os.IsNotExist(err) {
		t.Fatalf("expected no root signature.json, got err=%v", err)
	}
	sig, err := pluginhost.ReadSignature(signaturePath)
	if err != nil {
		t.Fatalf("ReadSignature: %v", err)
	}
	if err := pluginhost.VerifyDirSignature(bundleDir, sig, map[string]ed25519.PublicKey{"owner": publicKey}); err != nil {
		t.Fatalf("VerifyDirSignature: %v", err)
	}
	if !strings.Contains(output, "signature: "+signaturePath) {
		t.Fatalf("sign output missing bundle signature path\n%s", output)
	}
}

func TestRunSignPluginCommand_RejectsFlatPluginRoot(t *testing.T) {
	resetMainEnv(t)

	dir := t.TempDir()
	pluginRoot := filepath.Join(dir, "plugins", "flat")
	if err := os.MkdirAll(pluginRoot, 0o755); err != nil {
		t.Fatalf("MkdirAll(pluginRoot): %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "plugin.json"), []byte(`{"id":"flat","name":"flat","version":"0.1.0"}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.json): %v", err)
	}
	if err := os.WriteFile(filepath.Join(pluginRoot, "plugin.lua"), []byte(`return {}`), 0o644); err != nil {
		t.Fatalf("WriteFile(plugin.lua): %v", err)
	}

	_, privateKey, err := pluginhost.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}
	privateKeyPath := filepath.Join(dir, "keys", "owner.key")
	if err := pluginhost.WriteEd25519PrivateKeyFile(privateKeyPath, privateKey); err != nil {
		t.Fatalf("WriteEd25519PrivateKeyFile: %v", err)
	}

	code := runSignPluginCommand([]string{
		"--dir", pluginRoot,
		"--key-id", "owner",
		"--private-key-file", privateKeyPath,
	})
	if code == 0 {
		t.Fatal("expected sign-plugin to reject flat plugin roots without bundle state")
	}
	if _, err := os.Stat(filepath.Join(pluginRoot, "signature.json")); err == nil {
		t.Fatal("did not expect signature.json to be written at a flat plugin root")
	}
}

func TestRunSignPluginCommand_UsesConfiguredCachedBundleRepositoryArtifactDir(t *testing.T) {
	resetMainEnv(t)

	dir := t.TempDir()
	t.Setenv("MAMACORD_BUNDLE_BACKEND", "cached")
	t.Setenv("MAMACORD_BUNDLE_STORE_DIR", filepath.Join(dir, "bundle-store"))
	t.Setenv("MAMACORD_BUNDLE_CACHE_DIR", filepath.Join(dir, "bundle-cache"))

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

	publicKey, privateKey, err := pluginhost.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}
	privateKeyPath := filepath.Join(dir, "keys", "owner.key")
	if err := pluginhost.WriteEd25519PrivateKeyFile(privateKeyPath, privateKey); err != nil {
		t.Fatalf("WriteEd25519PrivateKeyFile: %v", err)
	}

	output := captureStdout(t, func() {
		code := runSignPluginCommand([]string{
			"--dir", pluginRoot,
			"--key-id", "owner",
			"--private-key-file", privateKeyPath,
		})
		if code != 0 {
			t.Fatalf("runSignPluginCommand returned %d", code)
		}
	})

	signaturePath := filepath.Join(bundle.BundleDir, "signature.json")
	if _, err := os.Stat(signaturePath); err != nil {
		t.Fatalf("expected signature in bundle artifact dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle.ActiveDir, "signature.json")); err != nil {
		t.Fatalf("expected signature in active cache dir: %v", err)
	}
	sig, err := pluginhost.ReadSignature(signaturePath)
	if err != nil {
		t.Fatalf("ReadSignature: %v", err)
	}
	if err := pluginhost.VerifyDirSignature(bundle.BundleDir, sig, map[string]ed25519.PublicKey{"owner": publicKey}); err != nil {
		t.Fatalf("VerifyDirSignature: %v", err)
	}
	if !strings.Contains(output, "signature: "+signaturePath) {
		t.Fatalf("sign output missing artifact signature path\n%s", output)
	}
}

func TestRunSignPluginCommand_UsesConfiguredObjectStoreBundleRepositoryArtifactDir(t *testing.T) {
	resetMainEnv(t)

	dir := t.TempDir()
	t.Setenv("MAMACORD_BUNDLE_BACKEND", "objectstore")
	t.Setenv("MAMACORD_BUNDLE_STORE_DIR", filepath.Join(dir, "object-store"))
	t.Setenv("MAMACORD_BUNDLE_CACHE_DIR", filepath.Join(dir, "bundle-cache"))

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

	publicKey, privateKey, err := pluginhost.GenerateEd25519Key()
	if err != nil {
		t.Fatalf("GenerateEd25519Key: %v", err)
	}
	privateKeyPath := filepath.Join(dir, "keys", "owner.key")
	if err := pluginhost.WriteEd25519PrivateKeyFile(privateKeyPath, privateKey); err != nil {
		t.Fatalf("WriteEd25519PrivateKeyFile: %v", err)
	}

	output := captureStdout(t, func() {
		code := runSignPluginCommand([]string{
			"--dir", pluginRoot,
			"--key-id", "owner",
			"--private-key-file", privateKeyPath,
		})
		if code != 0 {
			t.Fatalf("runSignPluginCommand returned %d", code)
		}
	})

	signaturePath := filepath.Join(bundle.BundleDir, "signature.json")
	if _, err := os.Stat(signaturePath); err != nil {
		t.Fatalf("expected signature in bundle artifact dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(bundle.ActiveDir, "signature.json")); err != nil {
		t.Fatalf("expected signature in active cache dir: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "object-store", "sample", "bundles", "git-release-v0.1.0", "signature.json")); err != nil {
		t.Fatalf("expected signature persisted to object-store artifact: %v", err)
	}
	sig, err := pluginhost.ReadSignature(signaturePath)
	if err != nil {
		t.Fatalf("ReadSignature: %v", err)
	}
	if err := pluginhost.VerifyDirSignature(bundle.BundleDir, sig, map[string]ed25519.PublicKey{"owner": publicKey}); err != nil {
		t.Fatalf("VerifyDirSignature: %v", err)
	}
	if !strings.Contains(output, "signature: "+signaturePath) {
		t.Fatalf("sign output missing artifact signature path\n%s", output)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	original := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()

	os.Stdout = w
	defer func() {
		os.Stdout = original
	}()
	fn()
	_ = w.Close()

	bytes, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("io.ReadAll: %v", err)
	}
	return string(bytes)
}

func resetMainEnv(t *testing.T) {
	t.Helper()

	for _, name := range []string{
		"DISCORD_TOKEN",
		"MAMACORD_STORAGE_BACKEND",
		"MAMACORD_POSTGRES_DSN",
		"MIGRATIONS_DIR",
		"MAMACORD_PROD_MODE",
		"MAMACORD_ALLOW_UNSIGNED_PLUGINS",
		"MAMACORD_TRUSTED_KEYS_FILE",
		"MAMACORD_DASHBOARD_SIGNING_KEY_ID",
		"MAMACORD_DASHBOARD_SIGNING_KEY_FILE",
		"MAMACORD_DASHBOARD_CLIENT_ID",
		"MAMACORD_DASHBOARD_CLIENT_SECRET",
		"MAMACORD_DASHBOARD_SESSION_SECRET",
		"MAMACORD_ADMIN_ADDR",
		"MAMACORD_LOADED_ENV_FILE",
		"MAMACORD_LOADED_ENV_SOURCE",
	} {
		t.Setenv(name, "")
	}
}
