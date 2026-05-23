package config

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	DiscordToken string

	SQLitePath          string
	Migrations          string
	MigrationBackups    string
	OpsAddr             string
	AdminAddr           string
	LocalesDir          string
	BundledPluginsDir   string
	UserPluginsDir      string
	MarketplaceCacheDir string
	PermissionsFile     string
	ModulesFile         string
	LogLevel            string
	ProdMode            bool
	OwnerUserID         *uint64
	DevGuildID          *uint64

	CommandRegistrationMode  string
	CommandGuildIDs          []uint64
	CommandRegisterAllGuilds bool

	AllowUnsignedPlugins bool
	TrustedKeysFile      string

	// PublicDashboardOrigin is the browser-visible dashboard origin (for example a
	// GitHub Pages site). Used for production setup responses and OAuth return
	// redirects when the dashboard is not served from the admin API origin.
	PublicDashboardOrigin string
	// PublicAPIOrigin is the browser-visible admin API origin (for example
	// https://api.example.com). Used for OAuth redirect URIs and install callbacks.
	PublicAPIOrigin string
	// DashboardAllowedOrigins is the explicit CORS allowlist for the dashboard in
	// production. Origins must include scheme + host (and port if non-default).
	DashboardAllowedOrigins []string

	DashboardClientID      string
	DashboardClientSecret  string
	DashboardSessionSecret string
	// DashboardSessionSecretGenerated is true when running in dev mode and an
	// ephemeral session secret was generated at startup (not read from env).
	DashboardSessionSecretGenerated bool
	DashboardSigningKeyID           string
	DashboardSigningKeyFile         string

	SlashCooldown          time.Duration
	ComponentCooldown      time.Duration
	ModalCooldown          time.Duration
	SlashCooldownBypass    []string
	SlashCooldownOverrides map[string]time.Duration
}

const (
	defaultSQLitePath          = "./data/mamacord.sqlite"
	defaultMigrationsDir       = "./migrations/sqlite"
	defaultMigrationBackups    = "./data/migration_backups"
	defaultOpsAddr             = ""
	defaultAdminAddr           = ""
	defaultLocalesDir          = "./locales"
	defaultBundledPluginsDir   = "./plugins"
	defaultUserPluginsDir      = "./data/plugins"
	defaultMarketplaceCacheDir = "./data/marketplace_cache"
	defaultPermissionsFile     = "./config/permissions.json"
	defaultModulesFile         = "./config/modules.json"
	defaultTrustedKeysFile     = "./config/trusted_keys.json"
	defaultLogLevel            = "info"
	defaultCommandRegMode      = "global"
	defaultSlashCooldownMS     = 5000
	defaultComponentCooldown   = 750
	defaultModalCooldownMS     = 1500
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

func loadFromEnv(requireDiscordToken bool) (Config, error) {
	var (
		discordToken string
		err          error
	)
	if requireDiscordToken {
		discordToken, err = requiredEnv("DISCORD_TOKEN")
		if err != nil {
			return Config{}, err
		}
	} else {
		discordToken = envDefault("DISCORD_TOKEN", "")
	}

	sqlitePath := envDefault("SQLITE_PATH", defaultSQLitePath)
	migrations := envDefault("MIGRATIONS_DIR", defaultMigrationsDir)
	migrationBackups := envDefault("MAMACORD_MIGRATION_BACKUPS_DIR", defaultMigrationBackups)
	opsAddr := envDefault("MAMACORD_OPS_ADDR", defaultOpsAddr)
	adminAddr := envDefault("MAMACORD_ADMIN_ADDR", defaultAdminAddr)
	localesDir := envDefault("LOCALES_DIR", defaultLocalesDir)
	bundledPluginsDir := envDefault("MAMACORD_BUNDLED_PLUGINS_DIR", defaultBundledPluginsDir)
	userPluginsDir := envDefault("MAMACORD_USER_PLUGINS_DIR", envDefault("PLUGINS_DIR", defaultUserPluginsDir))
	marketplaceCacheDir := envDefault("MAMACORD_MARKETPLACE_CACHE_DIR", defaultMarketplaceCacheDir)
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
	if strings.TrimSpace(adminAddr) != "" {
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
		DiscordToken:        discordToken,
		SQLitePath:          sqlitePath,
		Migrations:          migrations,
		MigrationBackups:    migrationBackups,
		OpsAddr:             opsAddr,
		AdminAddr:           adminAddr,
		LocalesDir:          localesDir,
		BundledPluginsDir:   bundledPluginsDir,
		UserPluginsDir:      userPluginsDir,
		MarketplaceCacheDir: marketplaceCacheDir,
		PermissionsFile:     permissionsFile,
		ModulesFile:         modulesFile,
		LogLevel:            logLevel,
		ProdMode:            prodMode,
		OwnerUserID:         ownerUserID,
		DevGuildID:          devGuildID,

		CommandRegistrationMode:  cmdRegMode,
		CommandGuildIDs:          cmdGuildIDs,
		CommandRegisterAllGuilds: cmdRegisterAllGuilds,

		AllowUnsignedPlugins:            allowUnsigned,
		TrustedKeysFile:                 trustedKeysFile,
		PublicDashboardOrigin:           publicDashboardOrigin,
		PublicAPIOrigin:                 publicAPIOrigin,
		DashboardAllowedOrigins:         dashboardAllowedOrigins,
		DashboardClientID:               dashboardClientID,
		DashboardClientSecret:           dashboardClientSecret,
		DashboardSessionSecret:          dashboardSessionSecret,
		DashboardSessionSecretGenerated: dashboardSessionSecretGenerated,
		DashboardSigningKeyID:           dashboardSigningKeyID,
		DashboardSigningKeyFile:         dashboardSigningKeyFile,

		SlashCooldown:          slashCooldown,
		ComponentCooldown:      componentCooldown,
		ModalCooldown:          modalCooldown,
		SlashCooldownBypass:    slashBypass,
		SlashCooldownOverrides: slashOverrides,
	}, nil
}

func splitHostPortLoose(addr string) (host string, port string, err error) {
	if strings.TrimSpace(addr) == "" {
		return "", "", errors.New("empty addr")
	}
	// net.SplitHostPort requires a port, but admin addr might be ":8081".
	if strings.HasPrefix(addr, ":") {
		return "", strings.TrimPrefix(addr, ":"), nil
	}
	h, p, err := net.SplitHostPort(addr)
	if err == nil {
		return h, p, nil
	}
	// Last resort: if the user passed "host:port" without IPv6 brackets but with extra colons,
	// this is likely invalid; keep it as an error.
	return "", "", err
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
