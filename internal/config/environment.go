package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Discord   DiscordConfig
	Storage   StorageConfig
	Runtime   RuntimeConfig
	Files     FileConfig
	Bundles   BundleConfig
	Commands  CommandConfig
	Plugins   PluginConfig
	Dashboard DashboardConfig
	Cooldowns CooldownConfig
}

type DiscordConfig struct {
	Token       string
	OwnerUserID *uint64
	DevGuildID  *uint64
}

type StorageConfig struct {
	Backend     StorageBackend
	PostgresDSN string
	Migrations  string
}

type RuntimeConfig struct {
	OpsAddr   string
	AdminAddr string
	Roles     []RuntimeRole
	LogLevel  string
	ProdMode  bool
}

type FileConfig struct {
	LocalesDir      string
	PermissionsFile string
	ModulesFile     string
}

type BundleConfig struct {
	Backend             BundleBackend
	StoreDir            string
	CacheDir            string
	BundledPluginsDir   string
	UserPluginsDir      string
	MarketplaceCacheDir string
}

type CommandConfig struct {
	RegistrationMode  string
	GuildIDs          []uint64
	RegisterAllGuilds bool
}

type PluginConfig struct {
	AllowUnsigned   bool
	TrustedKeysFile string
}

type DashboardConfig struct {
	PublicOrigin           string
	APIOrigin              string
	AllowedOrigins         []string
	ClientID               string
	ClientSecret           string
	SessionSecret          string
	SessionSecretGenerated bool
	SigningKeyID           string
	SigningKeyFile         string
}

type CooldownConfig struct {
	Slash          time.Duration
	Component      time.Duration
	Modal          time.Duration
	SlashBypass    []string
	SlashOverrides map[string]time.Duration
}

type StorageBackend string

const (
	StorageBackendPostgres StorageBackend = "postgres"
)

type BundleBackend string

const (
	BundleBackendLocal       BundleBackend = "local"
	BundleBackendCached      BundleBackend = "cached"
	BundleBackendObjectStore BundleBackend = "objectstore"
)

type RuntimeRole string

const (
	RuntimeRoleControl   RuntimeRole = "control"
	RuntimeRoleGateway   RuntimeRole = "gateway"
	RuntimeRoleScheduler RuntimeRole = "scheduler"
)

const (
	defaultPostgresDSN           = "postgres://mamacord:secret@127.0.0.1:5432/mamacord?sslmode=disable"
	defaultPostgresMigrationsDir = "./migrations/postgres"
	defaultOpsAddr               = ""
	defaultAdminAddr             = ""
	defaultLocalesDir            = "./locales"
	defaultBundledPluginsDir     = "./plugins"
	defaultUserPluginsDir        = "./data/plugins"
	defaultMarketplaceCacheDir   = "./data/marketplace_cache"
	defaultBundleStoreDir        = "./data/bundles/store"
	defaultBundleCacheDir        = "./data/bundles/cache"
	defaultPermissionsFile       = "./config/permissions.json"
	defaultModulesFile           = "./config/modules.json"
	defaultTrustedKeysFile       = "./config/trusted_keys.json"
	defaultLogLevel              = "info"
	defaultCommandRegMode        = "global"
	defaultSlashCooldownMS       = 5000
	defaultComponentCooldown     = 750
	defaultModalCooldownMS       = 1500
)

func LoadFromEnv() (Config, error) {
	return loadFromEnv(true)
}

// LoadFromEnvOptionalDiscordToken loads configuration from environment variables,
// but does not require DISCORD_TOKEN to be set. This is intended for helper
// commands like "doctor" and "init".
func LoadFromEnvOptionalDiscordToken() (Config, error) {
	return loadFromEnv(false)
}

func LoadStorageFromEnv() (Config, error) {
	return loadFromEnv(false)
}

func LoadBundleFromEnv() (Config, error) {
	return loadBundleFromEnv()
}

func loadFromEnv(requireDiscordToken bool) (Config, error) {
	runtimeRoles, err := parseRuntimeRoles(os.Getenv("MAMACORD_RUNTIME_ROLES"))
	if err != nil {
		return Config{}, err
	}

	var (
		discordToken string
	)
	if requireDiscordToken && runtimeRolesRequireDiscordToken(runtimeRoles) {
		var err error
		discordToken, err = requiredEnv("DISCORD_TOKEN")
		if err != nil {
			return Config{}, err
		}
	} else {
		discordToken = envDefault("DISCORD_TOKEN", "")
	}

	storageBackend := StorageBackend(strings.ToLower(envDefault("MAMACORD_STORAGE_BACKEND", string(StorageBackendPostgres))))
	switch storageBackend {
	case StorageBackendPostgres:
	default:
		return Config{}, fmt.Errorf("invalid MAMACORD_STORAGE_BACKEND %q", storageBackend)
	}

	postgresDSN := envDefault("MAMACORD_POSTGRES_DSN", defaultPostgresDSN)
	migrations := envDefault("MIGRATIONS_DIR", defaultPostgresMigrationsDir)
	opsAddr := envDefault("MAMACORD_OPS_ADDR", defaultOpsAddr)
	adminAddr := envDefault("MAMACORD_ADMIN_ADDR", defaultAdminAddr)
	localesDir := envDefault("LOCALES_DIR", defaultLocalesDir)
	bundledPluginsDir := envDefault("MAMACORD_BUNDLED_PLUGINS_DIR", defaultBundledPluginsDir)
	userPluginsDir := envDefault("MAMACORD_USER_PLUGINS_DIR", envDefault("PLUGINS_DIR", defaultUserPluginsDir))
	marketplaceCacheDir := envDefault("MAMACORD_MARKETPLACE_CACHE_DIR", defaultMarketplaceCacheDir)
	bundleBackend, bundleStoreDir, bundleCacheDir, err := parseBundleSettingsFromEnv()
	if err != nil {
		return Config{}, err
	}
	permissionsFile := envDefault("MAMACORD_PERMISSIONS_FILE", defaultPermissionsFile)
	modulesFile := envDefault("MAMACORD_MODULES_FILE", defaultModulesFile)
	logLevel := envDefault("LOG_LEVEL", defaultLogLevel)

	prodMode := envBool1("MAMACORD_PROD_MODE")
	allowUnsigned := envBool1("MAMACORD_ALLOW_UNSIGNED_PLUGINS")
	trustedKeysFile := envDefault("MAMACORD_TRUSTED_KEYS_FILE", defaultTrustedKeysFile)
	dashboardClientID := envDefault("MAMACORD_DASHBOARD_CLIENT_ID", "")
	dashboardClientSecret := envDefault("MAMACORD_DASHBOARD_CLIENT_SECRET", "")
	dashboardSessionSecret := envDefault("MAMACORD_DASHBOARD_SESSION_SECRET", "")
	dashboardSigningKeyID := envDefault("MAMACORD_DASHBOARD_SIGNING_KEY_ID", "")
	dashboardSigningKeyFile := envDefault("MAMACORD_DASHBOARD_SIGNING_KEY_FILE", "")
	dashboardSessionSecretGenerated := false
	publicDashboardOrigin := envDefault("MAMACORD_PUBLIC_DASHBOARD_ORIGIN", "")
	publicAPIOrigin := envDefault("MAMACORD_PUBLIC_API_ORIGIN", "")
	dashboardAllowedOrigins := parseCSV(os.Getenv("MAMACORD_DASHBOARD_ALLOWED_ORIGINS"))

	ownerUserID, err := parseOwnerID(os.Getenv("OWNER_USER_ID"))
	if err != nil {
		return Config{}, err
	}

	devGuildRaw := os.Getenv("DISCORD_DEV_GUILD_ID")
	devGuildVal, hasDevGuild, err := parseOptionalUint64(devGuildRaw)
	if err != nil {
		return Config{}, err
	}
	var devGuildID *uint64
	if hasDevGuild {
		v := devGuildVal
		devGuildID = &v
	}

	cmdRegMode := strings.ToLower(envDefault("MAMACORD_COMMAND_REGISTRATION_MODE", defaultCommandRegMode))
	switch cmdRegMode {
	case "global", "guilds", "hybrid":
	default:
		return Config{}, fmt.Errorf("invalid MAMACORD_COMMAND_REGISTRATION_MODE %q", cmdRegMode)
	}

	cmdGuildIDs, err := parseUint64List(os.Getenv("MAMACORD_COMMAND_GUILD_IDS"), "MAMACORD_COMMAND_GUILD_IDS")
	if err != nil {
		return Config{}, err
	}
	cmdRegisterAllGuilds := strings.TrimSpace(os.Getenv("MAMACORD_COMMAND_REGISTER_ALL_GUILDS")) == "1"

	slashCooldown, err := parseDurationMS(os.Getenv("MAMACORD_SLASH_COOLDOWN_MS"), defaultSlashCooldownMS)
	if err != nil {
		return Config{}, err
	}
	componentCooldown, err := parseDurationMS(os.Getenv("MAMACORD_COMPONENT_COOLDOWN_MS"), defaultComponentCooldown)
	if err != nil {
		return Config{}, err
	}
	modalCooldown, err := parseDurationMS(os.Getenv("MAMACORD_MODAL_COOLDOWN_MS"), defaultModalCooldownMS)
	if err != nil {
		return Config{}, err
	}
	slashBypass := parseCSV(os.Getenv("MAMACORD_SLASH_COOLDOWN_BYPASS"))
	if len(slashBypass) == 0 {
		slashBypass = []string{"ping", "help", "plugins", "modules", "block", "unblock"}
	}
	slashOverrides, err := parseCooldownOverridesMS(os.Getenv("MAMACORD_SLASH_COOLDOWN_OVERRIDES_MS"))
	if err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(postgresDSN) == "" {
		return Config{}, errors.New("MAMACORD_POSTGRES_DSN is required when MAMACORD_STORAGE_BACKEND=postgres")
	}
	if strings.TrimSpace(adminAddr) != "" && roleEnabled(runtimeRoles, RuntimeRoleControl) {
		// Production stays strict, but dev should still start the admin API so the
		// dashboard can show setup diagnostics instead of "connection refused".
		if prodMode {
			if strings.TrimSpace(dashboardClientID) == "" {
				return Config{}, errors.New("MAMACORD_DASHBOARD_CLIENT_ID is required when MAMACORD_ADMIN_ADDR is set")
			}
			if strings.TrimSpace(dashboardClientSecret) == "" {
				return Config{}, errors.New("MAMACORD_DASHBOARD_CLIENT_SECRET is required when MAMACORD_ADMIN_ADDR is set")
			}
			if len(strings.TrimSpace(dashboardSessionSecret)) < 32 {
				return Config{}, errors.New("MAMACORD_DASHBOARD_SESSION_SECRET must be at least 32 characters when MAMACORD_ADMIN_ADDR is set")
			}

			if strings.TrimSpace(publicDashboardOrigin) == "" {
				return Config{}, errors.New("MAMACORD_PUBLIC_DASHBOARD_ORIGIN is required in prod when MAMACORD_ADMIN_ADDR is set")
			}
			if strings.TrimSpace(publicAPIOrigin) == "" {
				return Config{}, errors.New("MAMACORD_PUBLIC_API_ORIGIN is required in prod when MAMACORD_ADMIN_ADDR is set")
			}
			if len(dashboardAllowedOrigins) == 0 {
				return Config{}, errors.New("MAMACORD_DASHBOARD_ALLOWED_ORIGINS is required in prod when MAMACORD_ADMIN_ADDR is set")
			}
		} else {
			if len(strings.TrimSpace(dashboardSessionSecret)) < 32 {
				dashboardSessionSecret = randomDevSecret()
				dashboardSessionSecretGenerated = true
			}
		}
	}

	return Config{
		Discord: DiscordConfig{
			Token:       discordToken,
			OwnerUserID: ownerUserID,
			DevGuildID:  devGuildID,
		},
		Storage: StorageConfig{
			Backend:     storageBackend,
			PostgresDSN: postgresDSN,
			Migrations:  migrations,
		},
		Runtime: RuntimeConfig{
			OpsAddr:   opsAddr,
			AdminAddr: adminAddr,
			Roles:     runtimeRoles,
			LogLevel:  logLevel,
			ProdMode:  prodMode,
		},
		Files: FileConfig{
			LocalesDir:      localesDir,
			PermissionsFile: permissionsFile,
			ModulesFile:     modulesFile,
		},
		Bundles: BundleConfig{
			Backend:             bundleBackend,
			StoreDir:            bundleStoreDir,
			CacheDir:            bundleCacheDir,
			BundledPluginsDir:   bundledPluginsDir,
			UserPluginsDir:      userPluginsDir,
			MarketplaceCacheDir: marketplaceCacheDir,
		},
		Commands: CommandConfig{
			RegistrationMode:  cmdRegMode,
			GuildIDs:          cmdGuildIDs,
			RegisterAllGuilds: cmdRegisterAllGuilds,
		},
		Plugins: PluginConfig{
			AllowUnsigned:   allowUnsigned,
			TrustedKeysFile: trustedKeysFile,
		},
		Dashboard: DashboardConfig{
			PublicOrigin:           publicDashboardOrigin,
			APIOrigin:              publicAPIOrigin,
			AllowedOrigins:         dashboardAllowedOrigins,
			ClientID:               dashboardClientID,
			ClientSecret:           dashboardClientSecret,
			SessionSecret:          dashboardSessionSecret,
			SessionSecretGenerated: dashboardSessionSecretGenerated,
			SigningKeyID:           dashboardSigningKeyID,
			SigningKeyFile:         dashboardSigningKeyFile,
		},
		Cooldowns: CooldownConfig{
			Slash:          slashCooldown,
			Component:      componentCooldown,
			Modal:          modalCooldown,
			SlashBypass:    slashBypass,
			SlashOverrides: slashOverrides,
		},
	}, nil
}

func loadBundleFromEnv() (Config, error) {
	bundleBackend, bundleStoreDir, bundleCacheDir, err := parseBundleSettingsFromEnv()
	if err != nil {
		return Config{}, err
	}
	return Config{
		Bundles: BundleConfig{
			Backend:  bundleBackend,
			StoreDir: bundleStoreDir,
			CacheDir: bundleCacheDir,
		},
	}, nil
}

func (c Config) HasRuntimeRole(role RuntimeRole) bool {
	return roleEnabled(c.Runtime.Roles, role)
}

func (c Config) RuntimeRoleStrings() []string {
	roles := normalizeRuntimeRoles(c.Runtime.Roles)
	out := make([]string, 0, len(roles))
	for _, role := range roles {
		out = append(out, string(role))
	}
	return out
}

func (c Config) UsesDiscordRuntime() bool {
	return c.HasRuntimeRole(RuntimeRoleGateway) || c.HasRuntimeRole(RuntimeRoleScheduler)
}

func (c Config) ControlAPIEnabled() bool {
	return strings.TrimSpace(c.Runtime.AdminAddr) != "" && c.HasRuntimeRole(RuntimeRoleControl)
}

func parseBundleSettingsFromEnv() (BundleBackend, string, string, error) {
	bundleBackend := BundleBackend(strings.ToLower(envDefault("MAMACORD_BUNDLE_BACKEND", string(BundleBackendLocal))))
	switch bundleBackend {
	case BundleBackendLocal, BundleBackendCached, BundleBackendObjectStore:
	default:
		return "", "", "", fmt.Errorf("invalid MAMACORD_BUNDLE_BACKEND %q", bundleBackend)
	}
	bundleStoreDir := envDefault("MAMACORD_BUNDLE_STORE_DIR", defaultBundleStoreDir)
	bundleCacheDir := envDefault("MAMACORD_BUNDLE_CACHE_DIR", defaultBundleCacheDir)
	if bundleBackend == BundleBackendCached || bundleBackend == BundleBackendObjectStore {
		if strings.TrimSpace(bundleStoreDir) == "" {
			return "", "", "", errors.New("MAMACORD_BUNDLE_STORE_DIR is required when MAMACORD_BUNDLE_BACKEND is cached or objectstore")
		}
		if strings.TrimSpace(bundleCacheDir) == "" {
			return "", "", "", errors.New("MAMACORD_BUNDLE_CACHE_DIR is required when MAMACORD_BUNDLE_BACKEND is cached or objectstore")
		}
	}
	return bundleBackend, bundleStoreDir, bundleCacheDir, nil
}

func parseRuntimeRoles(raw string) ([]RuntimeRole, error) {
	items := parseCSV(strings.ToLower(strings.TrimSpace(raw)))
	if len(items) == 0 {
		return defaultRuntimeRoles(), nil
	}

	out := make([]RuntimeRole, 0, len(items))
	for _, item := range items {
		role := RuntimeRole(strings.TrimSpace(item))
		switch role {
		case RuntimeRoleControl, RuntimeRoleGateway, RuntimeRoleScheduler:
			out = append(out, role)
		default:
			return nil, fmt.Errorf("invalid MAMACORD_RUNTIME_ROLES entry %q", item)
		}
	}
	return normalizeRuntimeRoles(out), nil
}

func defaultRuntimeRoles() []RuntimeRole {
	return []RuntimeRole{
		RuntimeRoleControl,
		RuntimeRoleGateway,
		RuntimeRoleScheduler,
	}
}

func normalizeRuntimeRoles(in []RuntimeRole) []RuntimeRole {
	ordered := defaultRuntimeRoles()
	if len(in) == 0 {
		return append([]RuntimeRole(nil), ordered...)
	}

	seen := map[RuntimeRole]struct{}{}
	for _, role := range in {
		role = RuntimeRole(strings.ToLower(strings.TrimSpace(string(role))))
		switch role {
		case RuntimeRoleControl, RuntimeRoleGateway, RuntimeRoleScheduler:
			seen[role] = struct{}{}
		}
	}

	out := make([]RuntimeRole, 0, len(ordered))
	for _, role := range ordered {
		if _, ok := seen[role]; ok {
			out = append(out, role)
		}
	}
	return out
}

func roleEnabled(roles []RuntimeRole, target RuntimeRole) bool {
	target = RuntimeRole(strings.ToLower(strings.TrimSpace(string(target))))
	for _, role := range normalizeRuntimeRoles(roles) {
		if role == target {
			return true
		}
	}
	return false
}

func runtimeRolesRequireDiscordToken(roles []RuntimeRole) bool {
	return roleEnabled(roles, RuntimeRoleGateway) || roleEnabled(roles, RuntimeRoleScheduler)
}

func randomDevSecret() string {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		// Not expected; fallback keeps behavior deterministic.
		return strings.Repeat("x", 32)
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func parseOwnerID(raw string) (*uint64, error) {
	v, ok, err := parseOptionalUint64(raw)
	if err != nil {
		return nil, fmt.Errorf("invalid OWNER_USER_ID: %w", err)
	}
	if !ok {
		return nil, nil
	}
	return &v, nil
}

func parseOptionalUint64(raw string) (uint64, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, false, nil
	}

	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false, fmt.Errorf("invalid uint64 %q: %w", raw, err)
	}

	return v, true, nil
}

func envBool1(name string) bool {
	return strings.TrimSpace(os.Getenv(name)) == "1"
}

func envDefault(name, def string) string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return def
	}
	return raw
}

func requiredEnv(name string) (string, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return "", errors.New(name + " is required")
	}
	return raw, nil
}

func parseDurationMS(raw string, def int) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Duration(def) * time.Millisecond, nil
	}

	ms, err := strconv.Atoi(raw)
	if err != nil || ms < 0 {
		return 0, fmt.Errorf("invalid milliseconds %q", raw)
	}
	return time.Duration(ms) * time.Millisecond, nil
}

func parseCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		s := strings.TrimSpace(part)
		if s == "" {
			continue
		}
		out = append(out, s)
	}
	return out
}

func parseCooldownOverridesMS(raw string) (map[string]time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]time.Duration{}, nil
	}
	items := parseCSV(raw)
	if len(items) == 0 {
		return map[string]time.Duration{}, nil
	}

	out := make(map[string]time.Duration, len(items))
	for _, item := range items {
		key, msRaw, ok := strings.Cut(item, "=")
		if !ok {
			return nil, fmt.Errorf("invalid cooldown override %q (expected name=ms)", item)
		}

		key = strings.ToLower(strings.TrimSpace(key))
		if key == "" {
			return nil, fmt.Errorf("invalid cooldown override %q (empty name)", item)
		}

		msRaw = strings.TrimSpace(msRaw)
		ms, err := strconv.Atoi(msRaw)
		if err != nil || ms < 0 {
			return nil, fmt.Errorf("invalid cooldown override %q (invalid ms %q)", item, msRaw)
		}

		out[key] = time.Duration(ms) * time.Millisecond
	}
	return out, nil
}

func parseUint64List(raw string, envName string) ([]uint64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	parts := strings.Split(raw, ",")
	out := make([]uint64, 0, len(parts))

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		id, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s contains invalid snowflake %q: %w", strings.TrimSpace(envName), part, err)
		}

		out = append(out, id)
	}

	return out, nil
}
