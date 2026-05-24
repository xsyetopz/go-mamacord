package config_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/config"
)

func TestLoadFromEnv_Defaults(t *testing.T) {
	resetConfigEnv(t)
	t.Setenv("DISCORD_TOKEN", "discord-token")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}

	if cfg.DiscordToken != "discord-token" {
		t.Fatalf("unexpected discord token: %q", cfg.DiscordToken)
	}
	if cfg.StorageBackend != config.StorageBackendPostgres {
		t.Fatalf("unexpected storage backend: %q", cfg.StorageBackend)
	}
	if cfg.PostgresDSN != "postgres://mamacord:secret@127.0.0.1:5432/mamacord?sslmode=disable" {
		t.Fatalf("unexpected postgres dsn: %q", cfg.PostgresDSN)
	}
	if cfg.Migrations != "./migrations/postgres" {
		t.Fatalf("unexpected migrations dir: %q", cfg.Migrations)
	}
	if cfg.OpsAddr != "" {
		t.Fatalf("unexpected ops addr: %q", cfg.OpsAddr)
	}
	if cfg.AdminAddr != "" {
		t.Fatalf("unexpected admin addr: %q", cfg.AdminAddr)
	}
	if cfg.LocalesDir != "./locales" {
		t.Fatalf("unexpected locales dir: %q", cfg.LocalesDir)
	}
	if cfg.BundledPluginsDir != "./plugins" {
		t.Fatalf("unexpected bundled plugins dir: %q", cfg.BundledPluginsDir)
	}
	if cfg.UserPluginsDir != "./data/plugins" {
		t.Fatalf("unexpected user plugins dir: %q", cfg.UserPluginsDir)
	}
	if cfg.MarketplaceCacheDir != "./data/marketplace_cache" {
		t.Fatalf("unexpected marketplace cache dir: %q", cfg.MarketplaceCacheDir)
	}
	if cfg.BundleBackend != config.BundleBackendLocal {
		t.Fatalf("unexpected bundle backend: %q", cfg.BundleBackend)
	}
	if cfg.BundleStoreDir != "./data/bundles/store" {
		t.Fatalf("unexpected bundle store dir: %q", cfg.BundleStoreDir)
	}
	if cfg.BundleCacheDir != "./data/bundles/cache" {
		t.Fatalf("unexpected bundle cache dir: %q", cfg.BundleCacheDir)
	}
	if cfg.PermissionsFile != "./config/permissions.json" {
		t.Fatalf("unexpected permissions file: %q", cfg.PermissionsFile)
	}
	if cfg.ModulesFile != "./config/modules.json" {
		t.Fatalf("unexpected modules file: %q", cfg.ModulesFile)
	}
	if cfg.TrustedKeysFile != "./config/trusted_keys.json" {
		t.Fatalf("unexpected trusted keys file: %q", cfg.TrustedKeysFile)
	}
	if cfg.DashboardClientID != "" || cfg.DashboardClientSecret != "" || cfg.DashboardSessionSecret != "" {
		t.Fatalf("unexpected dashboard auth defaults: %#v", cfg)
	}
	if cfg.CommandRegistrationMode != "global" {
		t.Fatalf("unexpected command registration mode: %q", cfg.CommandRegistrationMode)
	}
	if cfg.SlashCooldown != 5*time.Second {
		t.Fatalf("unexpected slash cooldown: %s", cfg.SlashCooldown)
	}
	if cfg.ComponentCooldown != 750*time.Millisecond {
		t.Fatalf("unexpected component cooldown: %s", cfg.ComponentCooldown)
	}
	if cfg.ModalCooldown != 1500*time.Millisecond {
		t.Fatalf("unexpected modal cooldown: %s", cfg.ModalCooldown)
	}

	wantBypass := []string{"ping", "help", "plugins", "modules", "block", "unblock"}
	if !reflect.DeepEqual(cfg.SlashCooldownBypass, wantBypass) {
		t.Fatalf("unexpected bypass list: %#v", cfg.SlashCooldownBypass)
	}
	if len(cfg.SlashCooldownOverrides) != 0 {
		t.Fatalf("expected no cooldown overrides, got %#v", cfg.SlashCooldownOverrides)
	}
}

func TestLoadStorageFromEnv_ParsesPostgresBackend(t *testing.T) {
	resetConfigEnv(t)
	t.Setenv("MAMACORD_STORAGE_BACKEND", "postgres")
	t.Setenv("MAMACORD_POSTGRES_DSN", "postgres://bot:secret@localhost:5432/mamacord?sslmode=disable")

	cfg, err := config.LoadStorageFromEnv()
	if err != nil {
		t.Fatalf("LoadStorageFromEnv: %v", err)
	}
	if cfg.StorageBackend != config.StorageBackendPostgres {
		t.Fatalf("unexpected storage backend: %q", cfg.StorageBackend)
	}
	if cfg.PostgresDSN != "postgres://bot:secret@localhost:5432/mamacord?sslmode=disable" {
		t.Fatalf("unexpected postgres dsn: %q", cfg.PostgresDSN)
	}
	if cfg.Migrations != "./migrations/postgres" {
		t.Fatalf("unexpected migrations dir: %q", cfg.Migrations)
	}
}

func TestLoadStorageFromEnv_ParsesCachedBundleBackend(t *testing.T) {
	resetConfigEnv(t)
	t.Setenv("MAMACORD_BUNDLE_BACKEND", "cached")
	t.Setenv("MAMACORD_BUNDLE_STORE_DIR", "/srv/mamacord/bundles")
	t.Setenv("MAMACORD_BUNDLE_CACHE_DIR", "/var/cache/mamacord/bundles")

	cfg, err := config.LoadStorageFromEnv()
	if err != nil {
		t.Fatalf("LoadStorageFromEnv: %v", err)
	}
	if cfg.BundleBackend != config.BundleBackendCached {
		t.Fatalf("unexpected bundle backend: %q", cfg.BundleBackend)
	}
	if cfg.BundleStoreDir != "/srv/mamacord/bundles" {
		t.Fatalf("unexpected bundle store dir: %q", cfg.BundleStoreDir)
	}
	if cfg.BundleCacheDir != "/var/cache/mamacord/bundles" {
		t.Fatalf("unexpected bundle cache dir: %q", cfg.BundleCacheDir)
	}
}

func TestLoadStorageFromEnv_ParsesObjectStoreBundleBackend(t *testing.T) {
	resetConfigEnv(t)
	t.Setenv("MAMACORD_BUNDLE_BACKEND", "objectstore")
	t.Setenv("MAMACORD_BUNDLE_STORE_DIR", "/srv/mamacord/object-store")
	t.Setenv("MAMACORD_BUNDLE_CACHE_DIR", "/var/cache/mamacord/bundles")

	cfg, err := config.LoadStorageFromEnv()
	if err != nil {
		t.Fatalf("LoadStorageFromEnv: %v", err)
	}
	if cfg.BundleBackend != config.BundleBackendObjectStore {
		t.Fatalf("unexpected bundle backend: %q", cfg.BundleBackend)
	}
	if cfg.BundleStoreDir != "/srv/mamacord/object-store" {
		t.Fatalf("unexpected bundle store dir: %q", cfg.BundleStoreDir)
	}
	if cfg.BundleCacheDir != "/var/cache/mamacord/bundles" {
		t.Fatalf("unexpected bundle cache dir: %q", cfg.BundleCacheDir)
	}
}

func TestLoadStorageFromEnv_DefaultsPostgresDSN(t *testing.T) {
	resetConfigEnv(t)
	t.Setenv("MAMACORD_STORAGE_BACKEND", "postgres")

	cfg, err := config.LoadStorageFromEnv()
	if err != nil {
		t.Fatalf("LoadStorageFromEnv: %v", err)
	}
	if cfg.PostgresDSN != "postgres://mamacord:secret@127.0.0.1:5432/mamacord?sslmode=disable" {
		t.Fatalf("unexpected postgres dsn: %q", cfg.PostgresDSN)
	}
}

func TestLoadFromEnv_DefaultRuntimeRoles(t *testing.T) {
	resetConfigEnv(t)
	t.Setenv("DISCORD_TOKEN", "discord-token")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}

	want := []config.RuntimeRole{
		config.RuntimeRoleControl,
		config.RuntimeRoleGateway,
		config.RuntimeRoleScheduler,
	}
	if !reflect.DeepEqual(cfg.RuntimeRoles, want) {
		t.Fatalf("unexpected runtime roles: got %#v want %#v", cfg.RuntimeRoles, want)
	}
}

func TestLoadFromEnv_ParsesRuntimeRoles(t *testing.T) {
	resetConfigEnv(t)
	t.Setenv("DISCORD_TOKEN", "discord-token")
	t.Setenv("MAMACORD_RUNTIME_ROLES", "scheduler,control")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}

	want := []config.RuntimeRole{
		config.RuntimeRoleControl,
		config.RuntimeRoleScheduler,
	}
	if !reflect.DeepEqual(cfg.RuntimeRoles, want) {
		t.Fatalf("unexpected runtime roles: got %#v want %#v", cfg.RuntimeRoles, want)
	}
}

func TestLoadFromEnv_ControlRoleDoesNotRequireDiscordToken(t *testing.T) {
	resetConfigEnv(t)
	t.Setenv("MAMACORD_RUNTIME_ROLES", "control")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}
	if cfg.DiscordToken != "" {
		t.Fatalf("expected empty discord token for control-only runtime, got %q", cfg.DiscordToken)
	}
}

func TestLoadFromEnv_ParsesOverrides(t *testing.T) {
	resetConfigEnv(t)
	t.Setenv("DISCORD_TOKEN", "discord-token")
	t.Setenv("OWNER_USER_ID", "11")
	t.Setenv("DISCORD_DEV_GUILD_ID", "33")
	t.Setenv("MAMACORD_COMMAND_REGISTRATION_MODE", "hybrid")
	t.Setenv("MAMACORD_COMMAND_GUILD_IDS", "44,55")
	t.Setenv("MAMACORD_COMMAND_REGISTER_ALL_GUILDS", "1")
	t.Setenv("MAMACORD_OPS_ADDR", ":8080")
	t.Setenv("MAMACORD_ADMIN_ADDR", ":8081")
	t.Setenv("MAMACORD_PROD_MODE", "0")
	t.Setenv("MAMACORD_ALLOW_UNSIGNED_PLUGINS", "1")
	t.Setenv("MAMACORD_DASHBOARD_CLIENT_ID", "client-id")
	t.Setenv("MAMACORD_DASHBOARD_CLIENT_SECRET", "client-secret")
	t.Setenv("MAMACORD_DASHBOARD_SESSION_SECRET", strings.Repeat("s", 32))
	t.Setenv("MAMACORD_DASHBOARD_SIGNING_KEY_ID", "official")
	t.Setenv("MAMACORD_DASHBOARD_SIGNING_KEY_FILE", "./data/keys/official.key")
	t.Setenv("MAMACORD_SLASH_COOLDOWN_MS", "9000")
	t.Setenv("MAMACORD_COMPONENT_COOLDOWN_MS", "250")
	t.Setenv("MAMACORD_MODAL_COOLDOWN_MS", "350")
	t.Setenv("MAMACORD_SLASH_COOLDOWN_BYPASS", "ping, lookup:user")
	t.Setenv("MAMACORD_SLASH_COOLDOWN_OVERRIDES_MS", "lookup:user=2500,manager:roles:add=1000")

	cfg, err := config.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv: %v", err)
	}

	if cfg.OwnerUserID == nil || *cfg.OwnerUserID != 11 {
		t.Fatalf("unexpected owner id: %#v", cfg.OwnerUserID)
	}
	if cfg.DevGuildID == nil || *cfg.DevGuildID != 33 {
		t.Fatalf("unexpected dev guild id: %v", cfg.DevGuildID)
	}
	if cfg.CommandRegistrationMode != "hybrid" {
		t.Fatalf("unexpected command registration mode: %q", cfg.CommandRegistrationMode)
	}
	if !reflect.DeepEqual(cfg.CommandGuildIDs, []uint64{44, 55}) {
		t.Fatalf("unexpected command guild ids: %#v", cfg.CommandGuildIDs)
	}
	if !cfg.CommandRegisterAllGuilds {
		t.Fatalf("expected register-all-guilds to be enabled")
	}
	if cfg.ProdMode {
		t.Fatalf("expected prod mode to be disabled")
	}
	if !cfg.AllowUnsignedPlugins {
		t.Fatalf("expected unsigned plugins flag to be enabled")
	}
	if cfg.OpsAddr != ":8080" {
		t.Fatalf("unexpected ops addr: %q", cfg.OpsAddr)
	}
	if cfg.AdminAddr != ":8081" {
		t.Fatalf("unexpected admin addr: %q", cfg.AdminAddr)
	}
	if cfg.DashboardClientID != "client-id" || cfg.DashboardClientSecret != "client-secret" {
		t.Fatalf("unexpected dashboard auth config: %#v", cfg)
	}
	if cfg.DashboardSigningKeyID != "official" || cfg.DashboardSigningKeyFile != "./data/keys/official.key" {
		t.Fatalf("unexpected dashboard signing config: %#v", cfg)
	}
	if cfg.SlashCooldown != 9*time.Second {
		t.Fatalf("unexpected slash cooldown: %s", cfg.SlashCooldown)
	}
	if cfg.ComponentCooldown != 250*time.Millisecond {
		t.Fatalf("unexpected component cooldown: %s", cfg.ComponentCooldown)
	}
	if cfg.ModalCooldown != 350*time.Millisecond {
		t.Fatalf("unexpected modal cooldown: %s", cfg.ModalCooldown)
	}

	wantBypass := []string{"ping", "lookup:user"}
	if !reflect.DeepEqual(cfg.SlashCooldownBypass, wantBypass) {
		t.Fatalf("unexpected bypass list: %#v", cfg.SlashCooldownBypass)
	}

	wantOverrides := map[string]time.Duration{
		"lookup:user":       2500 * time.Millisecond,
		"manager:roles:add": 1000 * time.Millisecond,
	}
	if !reflect.DeepEqual(cfg.SlashCooldownOverrides, wantOverrides) {
		t.Fatalf("unexpected cooldown overrides: %#v", cfg.SlashCooldownOverrides)
	}
}

func TestLoadFromEnv_RejectsInvalidInputs(t *testing.T) {
	t.Run("registration mode", func(t *testing.T) {
		resetConfigEnv(t)
		t.Setenv("DISCORD_TOKEN", "discord-token")
		t.Setenv("MAMACORD_COMMAND_REGISTRATION_MODE", "broken")

		if _, err := config.LoadFromEnv(); err == nil {
			t.Fatalf("expected invalid registration mode error")
		}
	})

	t.Run("cooldown override", func(t *testing.T) {
		resetConfigEnv(t)
		t.Setenv("DISCORD_TOKEN", "discord-token")
		t.Setenv("MAMACORD_SLASH_COOLDOWN_OVERRIDES_MS", "lookup:user=nope")

		if _, err := config.LoadFromEnv(); err == nil {
			t.Fatalf("expected invalid cooldown override error")
		}
	})

	t.Run("owner id", func(t *testing.T) {
		resetConfigEnv(t)
		t.Setenv("DISCORD_TOKEN", "discord-token")
		t.Setenv("OWNER_USER_ID", "nope")

		if _, err := config.LoadFromEnv(); err == nil {
			t.Fatalf("expected invalid owner id error")
		}
	})

	t.Run("admin config", func(t *testing.T) {
		resetConfigEnv(t)
		t.Setenv("DISCORD_TOKEN", "discord-token")
		t.Setenv("MAMACORD_PROD_MODE", "1")
		t.Setenv("MAMACORD_ADMIN_ADDR", ":8081")
		t.Setenv("MAMACORD_DASHBOARD_CLIENT_ID", "client-id")
		t.Setenv("MAMACORD_DASHBOARD_CLIENT_SECRET", "client-secret")
		t.Setenv("MAMACORD_DASHBOARD_SESSION_SECRET", "too-short")

		if _, err := config.LoadFromEnv(); err == nil {
			t.Fatalf("expected invalid dashboard session secret error")
		}
	})

}

func TestLoadFromEnvOptionalDiscordToken_ReadsTokenWhenPresent(t *testing.T) {
	resetConfigEnv(t)
	t.Setenv("DISCORD_TOKEN", "discord-token")

	cfg, err := config.LoadFromEnvOptionalDiscordToken()
	if err != nil {
		t.Fatalf("LoadFromEnvOptionalDiscordToken: %v", err)
	}
	if cfg.DiscordToken != "discord-token" {
		t.Fatalf("unexpected discord token: %q", cfg.DiscordToken)
	}
}

func TestShippedSchemaURLs(t *testing.T) {
	t.Parallel()

	const schemaBaseURL = "https://raw.githubusercontent.com/xsyetopz/go-mamacord/refs/heads/main/schemas/"
	repoRoot := filepath.Clean(filepath.Join("..", ".."))

	cases := []struct {
		path string
		key  string
		want string
	}{
		{path: "config/trusted_keys.json", key: "$schema", want: schemaBaseURL + "trusted_keys.schema.v1.json"},
		{path: "config/permissions.json", key: "$schema", want: schemaBaseURL + "permissions.schema.v1.json"},
		{path: "config/modules.json", key: "$schema", want: schemaBaseURL + "modules.schema.v1.json"},
		{path: "schemas/messages.schema.v1.json", key: "$id", want: schemaBaseURL + "messages.schema.v1.json"},
		{path: "schemas/modules.schema.v1.json", key: "$id", want: schemaBaseURL + "modules.schema.v1.json"},
		{path: "schemas/permissions.schema.v1.json", key: "$id", want: schemaBaseURL + "permissions.schema.v1.json"},
		{path: "schemas/plugin.schema.v1.json", key: "$id", want: schemaBaseURL + "plugin.schema.v1.json"},
		{path: "schemas/signature.schema.v1.json", key: "$id", want: schemaBaseURL + "signature.schema.v1.json"},
		{path: "schemas/trusted_keys.schema.v1.json", key: "$id", want: schemaBaseURL + "trusted_keys.schema.v1.json"},
	}

	exampleBundleDir := examplePluginDir(t, repoRoot)
	cases = append(cases, struct {
		path string
		key  string
		want string
	}{
		path: repoRelativePath(t, repoRoot, filepath.Join(exampleBundleDir, "plugin.json")),
		key:  "$schema",
		want: schemaBaseURL + "plugin.schema.v1.json",
	})

	for _, pluginID := range bundledFirstPartyPluginIDs() {
		bundleDir := bundledFirstPartyPluginDir(t, repoRoot, pluginID)
		cases = append(cases,
			struct {
				path string
				key  string
				want string
			}{
				path: repoRelativePath(t, repoRoot, filepath.Join(bundleDir, "plugin.json")),
				key:  "$schema",
				want: schemaBaseURL + "plugin.schema.v1.json",
			},
			struct {
				path string
				key  string
				want string
			}{
				path: repoRelativePath(t, repoRoot, filepath.Join(bundleDir, "signature.json")),
				key:  "$schema",
				want: schemaBaseURL + "signature.schema.v1.json",
			},
		)
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()

			filePath := filepath.Join(repoRoot, tc.path)
			bytes, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("ReadFile(%q): %v", filePath, err)
			}

			var payload map[string]any
			if err := json.Unmarshal(bytes, &payload); err != nil {
				t.Fatalf("json.Unmarshal(%q): %v", filePath, err)
			}

			got, ok := payload[tc.key].(string)
			if !ok {
				t.Fatalf("missing %q in %s", tc.key, tc.path)
			}
			if got != tc.want {
				t.Fatalf("unexpected %s in %s: got %q want %q", tc.key, tc.path, got, tc.want)
			}
		})
	}
}

func TestAuthoringAssetsLayout(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	exampleBundleDir := examplePluginDir(t, repoRoot)

	luarcPath := filepath.Join(repoRoot, ".luarc.json")
	luarcBytes, err := os.ReadFile(luarcPath)
	if err != nil {
		t.Fatalf("ReadFile(%q): %v", luarcPath, err)
	}

	var luarc map[string]any
	if err := json.Unmarshal(luarcBytes, &luarc); err != nil {
		t.Fatalf("json.Unmarshal(%q): %v", luarcPath, err)
	}

	libraryEntries, ok := luarc["workspace.library"].([]any)
	if !ok {
		t.Fatalf("workspace.library missing or invalid in %s", luarcPath)
	}

	var hasBotAPI bool
	for _, entry := range libraryEntries {
		pathValue, ok := entry.(string)
		if !ok || pathValue != "./sdk/lua/bot_api.lua" {
			continue
		}
		hasBotAPI = true

		fullPath := filepath.Join(repoRoot, pathValue)
		if _, err := os.Stat(fullPath); err != nil {
			t.Fatalf("Stat(%q): %v", fullPath, err)
		}
	}
	if !hasBotAPI {
		t.Fatalf("workspace.library does not include ./sdk/lua/bot_api.lua")
	}

	paths := []string{
		"config/trusted_keys.json",
		filepath.Join("examples", "plugins", "example", bundles.StateFileName),
		repoRelativePath(t, repoRoot, filepath.Join(exampleBundleDir, "plugin.json")),
		repoRelativePath(t, repoRoot, filepath.Join(exampleBundleDir, "plugin.lua")),
		repoRelativePath(t, repoRoot, filepath.Join(exampleBundleDir, "lib", "counter.lua")),
		repoRelativePath(t, repoRoot, filepath.Join(exampleBundleDir, "locales", "en-US", "messages.json")),
		repoRelativePath(t, repoRoot, filepath.Join(exampleBundleDir, "locales", "en-GB", "messages.json")),
	}
	for _, pluginID := range bundledFirstPartyPluginIDs() {
		bundleDir := bundledFirstPartyPluginDir(t, repoRoot, pluginID)
		paths = append(paths,
			filepath.Join("plugins", pluginID, bundles.StateFileName),
			repoRelativePath(t, repoRoot, filepath.Join(bundleDir, "plugin.json")),
			repoRelativePath(t, repoRoot, filepath.Join(bundleDir, "plugin.lua")),
		)
	}
	for _, relPath := range paths {
		fullPath := filepath.Join(repoRoot, relPath)
		if _, err := os.Stat(fullPath); err != nil {
			t.Fatalf("Stat(%q): %v", fullPath, err)
		}
	}

	localeEntries, err := os.ReadDir(filepath.Join(repoRoot, "locales"))
	if err != nil {
		t.Fatalf("ReadDir(locales): %v", err)
	}
	for _, entry := range localeEntries {
		if !entry.IsDir() {
			continue
		}

		funLocalePath := filepath.Join(bundledFirstPartyPluginDir(t, repoRoot, "fun"), "locales", entry.Name(), "messages.json")
		if _, err := os.Stat(funLocalePath); err != nil {
			t.Fatalf("Stat(%q): %v", funLocalePath, err)
		}

		wellnessLocalePath := filepath.Join(bundledFirstPartyPluginDir(t, repoRoot, "wellness"), "locales", entry.Name(), "messages.json")
		if _, err := os.Stat(wellnessLocalePath); err != nil {
			t.Fatalf("Stat(%q): %v", wellnessLocalePath, err)
		}

		infoLocalePath := filepath.Join(bundledFirstPartyPluginDir(t, repoRoot, "info"), "locales", entry.Name(), "messages.json")
		if _, err := os.Stat(infoLocalePath); err != nil {
			t.Fatalf("Stat(%q): %v", infoLocalePath, err)
		}

		moderationLocalePath := filepath.Join(bundledFirstPartyPluginDir(t, repoRoot, "moderation"), "locales", entry.Name(), "messages.json")
		if _, err := os.Stat(moderationLocalePath); err != nil {
			t.Fatalf("Stat(%q): %v", moderationLocalePath, err)
		}

		managerLocalePath := filepath.Join(bundledFirstPartyPluginDir(t, repoRoot, "manager"), "locales", entry.Name(), "messages.json")
		if _, err := os.Stat(managerLocalePath); err != nil {
			t.Fatalf("Stat(%q): %v", managerLocalePath, err)
		}

		coreLocalePath := filepath.Join(repoRoot, "locales", entry.Name(), "messages.json")
		coreBytes, err := os.ReadFile(coreLocalePath)
		if err != nil {
			t.Fatalf("ReadFile(%q): %v", coreLocalePath, err)
		}

		var coreMessages []map[string]any
		if err := json.Unmarshal(coreBytes, &coreMessages); err != nil {
			t.Fatalf("json.Unmarshal(%q): %v", coreLocalePath, err)
		}
		for _, message := range coreMessages {
			id, _ := message["id"].(string)
			if strings.HasPrefix(id, "cmd.flip") ||
				strings.HasPrefix(id, "cmd.roll") ||
				strings.HasPrefix(id, "cmd.8ball") ||
				strings.HasPrefix(id, "cmd.hug") ||
				strings.HasPrefix(id, "cmd.pat") ||
				strings.HasPrefix(id, "cmd.poke") ||
				strings.HasPrefix(id, "cmd.shrug") ||
				strings.HasPrefix(id, "fun.") {
				t.Fatalf("core locale %q still contains migrated fun id %q", coreLocalePath, id)
			}
			if strings.HasPrefix(id, "cmd.timezone") ||
				strings.HasPrefix(id, "cmd.checkin") ||
				strings.HasPrefix(id, "cmd.remind") ||
				strings.HasPrefix(id, "wellness.") {
				t.Fatalf("core locale %q still contains migrated wellness id %q", coreLocalePath, id)
			}
			if strings.HasPrefix(id, "cmd.about") ||
				strings.HasPrefix(id, "cmd.lookup") ||
				strings.HasPrefix(id, "info.about") ||
				strings.HasPrefix(id, "info.lookup") {
				t.Fatalf("core locale %q still contains migrated info id %q", coreLocalePath, id)
			}
			if strings.HasPrefix(id, "cmd.warn") ||
				strings.HasPrefix(id, "cmd.unwarn") ||
				strings.HasPrefix(id, "mod.") {
				t.Fatalf("core locale %q still contains migrated moderation id %q", coreLocalePath, id)
			}
			if strings.HasPrefix(id, "cmd.slowmode") ||
				strings.HasPrefix(id, "cmd.nick") ||
				strings.HasPrefix(id, "cmd.roles") ||
				strings.HasPrefix(id, "cmd.purge") ||
				strings.HasPrefix(id, "cmd.emojis") ||
				strings.HasPrefix(id, "cmd.stickers") ||
				strings.HasPrefix(id, "mgr.") {
				t.Fatalf("core locale %q still contains migrated manager id %q", coreLocalePath, id)
			}
		}
	}

	for _, relPath := range []string{
		"migrations/postgres/001_init.up.sql",
		"migrations/postgres/002_guilds_users.up.sql",
		"migrations/postgres/003_wellness.up.sql",
		"migrations/postgres/004_modules.up.sql",
		"migrations/postgres/005_admin_sessions.up.sql",
		"migrations/postgres/006_discord_oauth.up.sql",
		"migrations/postgres/007_marketplace.up.sql",
		"migrations/postgres/008_plugin_bundle_registry.up.sql",
	} {
		fullPath := filepath.Join(repoRoot, relPath)
		if _, err := os.Stat(fullPath); err != nil {
			t.Fatalf("Stat(%q): %v", fullPath, err)
		}
	}
}

func bundledFirstPartyPluginIDs() []string {
	return []string{"fun", "info", "manager", "moderation", "wellness"}
}

func bundledFirstPartyPluginDir(t *testing.T, repoRoot string, pluginID string) string {
	t.Helper()

	dir, err := bundles.NewLocalRepository().ResolveActiveDir(filepath.Join(repoRoot, "plugins", pluginID))
	if err != nil {
		t.Fatalf("ResolveActiveDir(%q): %v", pluginID, err)
	}
	return dir
}

func examplePluginDir(t *testing.T, repoRoot string) string {
	t.Helper()

	dir, err := bundles.NewLocalRepository().ResolveActiveDir(filepath.Join(repoRoot, "examples", "plugins", "example"))
	if err != nil {
		t.Fatalf("ResolveActiveDir(example): %v", err)
	}
	return dir
}

func repoRelativePath(t *testing.T, repoRoot string, fullPath string) string {
	t.Helper()

	relPath, err := filepath.Rel(repoRoot, fullPath)
	if err != nil {
		t.Fatalf("filepath.Rel(%q, %q): %v", repoRoot, fullPath, err)
	}
	return relPath
}

func TestMigrationLayoutAndSchemaHygiene(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Clean(filepath.Join("..", ".."))
	migrationsDir := filepath.Join(repoRoot, "migrations", "postgres")

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("ReadDir(%q): %v", migrationsDir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue
		}
		if strings.HasSuffix(name, ".up.sql") {
			continue
		}
		t.Fatalf("legacy migration filename still present: %s", name)
	}
}

func resetConfigEnv(t *testing.T) {
	t.Helper()

	for _, name := range []string{
		"DISCORD_TOKEN",
		"MAMACORD_STORAGE_BACKEND",
		"MAMACORD_POSTGRES_DSN",
		"MIGRATIONS_DIR",
		"MAMACORD_OPS_ADDR",
		"MAMACORD_ADMIN_ADDR",
		"MAMACORD_PUBLIC_DASHBOARD_ORIGIN",
		"MAMACORD_PUBLIC_API_ORIGIN",
		"MAMACORD_DASHBOARD_ALLOWED_ORIGINS",
		"LOCALES_DIR",
		"PLUGINS_DIR",
		"MAMACORD_PERMISSIONS_FILE",
		"MAMACORD_MODULES_FILE",
		"LOG_LEVEL",
		"MAMACORD_PROD_MODE",
		"MAMACORD_ALLOW_UNSIGNED_PLUGINS",
		"MAMACORD_TRUSTED_KEYS_FILE",
		"MAMACORD_DASHBOARD_CLIENT_ID",
		"MAMACORD_DASHBOARD_CLIENT_SECRET",
		"MAMACORD_DASHBOARD_SESSION_SECRET",
		"MAMACORD_DASHBOARD_SIGNING_KEY_ID",
		"MAMACORD_DASHBOARD_SIGNING_KEY_FILE",
		"OWNER_USER_ID",
		"DISCORD_DEV_GUILD_ID",
		"MAMACORD_COMMAND_REGISTRATION_MODE",
		"MAMACORD_COMMAND_GUILD_IDS",
		"MAMACORD_COMMAND_REGISTER_ALL_GUILDS",
		"MAMACORD_SLASH_COOLDOWN_MS",
		"MAMACORD_COMPONENT_COOLDOWN_MS",
		"MAMACORD_MODAL_COOLDOWN_MS",
		"MAMACORD_SLASH_COOLDOWN_BYPASS",
		"MAMACORD_SLASH_COOLDOWN_OVERRIDES_MS",
	} {
		t.Setenv(name, "")
	}
}
