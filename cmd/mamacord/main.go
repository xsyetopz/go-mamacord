package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/xsyetopz/go-mamacord/internal/app"
	"github.com/xsyetopz/go-mamacord/internal/buildinfo"
	"github.com/xsyetopz/go-mamacord/internal/bundles"
	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/dotenv"
	"github.com/xsyetopz/go-mamacord/internal/logging"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	migrate "github.com/xsyetopz/go-mamacord/internal/migration"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	"github.com/xsyetopz/go-mamacord/internal/storagebootstrap"
)

func main() {
	os.Exit(runMain())
}

func runMain() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	loadedEnv, envErr := autoLoadEnvFile()
	if envErr != nil {
		_, _ = os.Stderr.WriteString(envErr.Error() + "\n")
		return 1
	}
	if loadedEnv.Path != "" {
		_ = os.Setenv("MAMACORD_LOADED_ENV_FILE", loadedEnv.Path)
		_ = os.Setenv("MAMACORD_LOADED_ENV_SOURCE", loadedEnv.Source)
	}

	// Low mental-load workflow: allow env files to be used without requiring users
	// to manually export variables before running.
	args := os.Args[1:]
	if len(args) > 0 && args[0] == "init" {
		return runInitCommand(args[1:])
	}
	if len(args) > 0 && args[0] == "doctor" {
		return runDoctorCommand(args[1:])
	}
	if len(args) > 0 && args[0] == "dev" {
		return runDevCommand(ctx)
	}
	if len(args) > 0 && args[0] == "migrate" {
		return runMigrateCommand(ctx, args[1:])
	}
	if len(args) > 0 && args[0] == "version" {
		printVersion()
		return 0
	}
	if len(args) > 0 && args[0] == "sign-plugin" {
		return runSignPluginCommand(args[1:])
	}
	if len(args) > 0 && args[0] == "gen-signing-key" {
		return runGenSigningKeyCommand(args[1:])
	}
	if len(args) > 0 && args[0] == "plugins" {
		return runPluginsCommand(ctx, args[1:])
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}

	logger, err := logging.New(cfg.LogLevel)
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}

	if runErr := run(ctx, logger, cfg); runErr != nil {
		logger.ErrorContext(ctx, "fatal", slog.String("err", runErr.Error()))
		return 1
	}

	return 0
}

func runDoctorCommand(args []string) int {
	fs := flag.NewFlagSet("doctor", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	if err := fs.Parse(args); err != nil {
		return 1
	}

	cfg, err := config.LoadFromEnvOptionalDiscordToken()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}

	writeLine := func(format string, a ...any) {
		_, _ = fmt.Fprintf(os.Stdout, format+"\n", a...)
	}

	loadedEnv := strings.TrimSpace(os.Getenv("MAMACORD_LOADED_ENV_FILE"))
	loadedEnvSource := strings.TrimSpace(os.Getenv("MAMACORD_LOADED_ENV_SOURCE"))
	if loadedEnv == "" {
		writeLine("env_file_loaded: false")
	} else {
		writeLine("env_file_loaded: %s", loadedEnv)
		writeLine("env_file_source: %s", loadedEnvSource)
	}

	hasToken := strings.TrimSpace(cfg.DiscordToken) != ""
	writeLine("discord_token: %t", hasToken)
	writeLine("storage_backend: %s", cfg.StorageBackend)
	writeLine("storage_target: %s", doctorStorageTarget(cfg))
	writeLine("migrations_dir: %s", cfg.Migrations)
	writeLine("runtime_roles: %s", strings.Join(cfg.RuntimeRoleStrings(), ","))
	writeLine("control_role_enabled: %t", cfg.HasRuntimeRole(config.RuntimeRoleControl))
	writeLine("gateway_role_enabled: %t", cfg.HasRuntimeRole(config.RuntimeRoleGateway))
	writeLine("scheduler_role_enabled: %t", cfg.HasRuntimeRole(config.RuntimeRoleScheduler))
	writeLine("prod_mode: %t", cfg.ProdMode)
	writeLine("admin_api_enabled: %t", cfg.ControlAPIEnabled())
	writeLine("allow_unsigned_plugins: %t", cfg.AllowUnsignedPlugins)
	writeLine("trusted_keys_file: %s", cfg.TrustedKeysFile)
	trustedKeysPath := strings.TrimSpace(cfg.TrustedKeysFile)
	trustedKeysExists := false
	trustedKeysCount := 0
	if trustedKeysPath != "" {
		if keys, err := pluginhost.ReadTrustedKeysFile(trustedKeysPath); err == nil {
			trustedKeysExists = true
			trustedKeysCount = len(keys)
		} else if !os.IsNotExist(err) {
			writeLine("trusted_keys_file_error: %s", err)
		}
	}
	writeLine("trusted_keys_file_exists: %t", trustedKeysExists)
	writeLine("trusted_keys_count_file: %d", trustedKeysCount)
	writeLine(
		"dashboard_signing_configured: %t",
		strings.TrimSpace(cfg.DashboardSigningKeyID) != "" && strings.TrimSpace(cfg.DashboardSigningKeyFile) != "",
	)
	if cfg.ControlAPIEnabled() {
		writeLine("admin_addr: %s", cfg.AdminAddr)
		writeLine("setup_url: %s/api/setup", httpBaseFromAddr(cfg.AdminAddr))
	}

	if cfg.ControlAPIEnabled() {
		base := httpBaseFromAddr(cfg.AdminAddr)
		writeLine("dashboard_base_url: %s", base)
		writeLine("dashboard_oauth_redirect_url: %s/api/auth/callback", base)
		writeLine("dashboard_client_id_set: %t", strings.TrimSpace(cfg.DashboardClientID) != "")
		writeLine("dashboard_client_secret_set: %t", strings.TrimSpace(cfg.DashboardClientSecret) != "")
		writeLine("dashboard_session_secret_set: %t", len(strings.TrimSpace(cfg.DashboardSessionSecret)) >= 32)
		if cfg.DashboardSessionSecretGenerated {
			writeLine("dashboard_session_secret_generated: true (dev-only, ephemeral)")
		}
	}

	if cfg.ControlAPIEnabled() && cfg.ProdMode {
		if strings.TrimSpace(cfg.DashboardClientID) == "" ||
			strings.TrimSpace(cfg.DashboardClientSecret) == "" ||
			len(strings.TrimSpace(cfg.DashboardSessionSecret)) < 32 {
			writeLine("")
			writeLine("next: admin api is enabled in prod mode but oauth/session config is incomplete")
			writeLine("next: fill MAMACORD_DASHBOARD_* vars (client id/secret/session secret)")
			return 1
		}
	}

	if cfg.UsesDiscordRuntime() && !hasToken {
		writeLine("")
		writeLine("next: set DISCORD_TOKEN to start the bot")
		return 1
	}

	return 0
}

func autoLoadEnvFile() (dotenv.SearchResult, error) {
	if strings.TrimSpace(os.Getenv("MAMACORD_DISABLE_DOTENV")) == "1" {
		return dotenv.SearchResult{}, nil
	}
	if bad := forbiddenDotenvFile(); bad != "" {
		return dotenv.SearchResult{}, fmt.Errorf("forbidden env file detected: %s (only .env.dev/.env.prod are allowed)", bad)
	}

	searchDirs := []dotenv.SearchResult{
		{Path: ".", Source: "working_dir"},
	}
	if execPath, err := os.Executable(); err == nil {
		if execDir := strings.TrimSpace(filepath.Dir(execPath)); execDir != "" && execDir != "." {
			searchDirs = append(searchDirs, dotenv.SearchResult{Path: execDir, Source: "executable_dir"})
		}
	}

	if explicit := strings.TrimSpace(os.Getenv("MAMACORD_ENV_FILE")); explicit != "" {
		base := filepath.Base(explicit)
		if base != ".env.dev" && base != ".env.prod" {
			return dotenv.SearchResult{}, fmt.Errorf("refusing to load non-standard env file %s; use .env.dev or .env.prod instead", base)
		}
		return dotenv.LoadAutoWithSearch([]string{explicit}, searchDirs)
	}

	subcmd := ""
	if len(os.Args) > 1 {
		subcmd = strings.TrimSpace(os.Args[1])
	}

	switch subcmd {
	case "dev", "init":
		return dotenv.LoadAutoWithSearch([]string{".env.dev"}, searchDirs)
	case "doctor":
		return dotenv.LoadAutoWithSearch([]string{".env.prod", ".env.dev"}, searchDirs)
	default:
		return dotenv.LoadAutoWithSearch([]string{".env.prod"}, searchDirs)
	}
}

func httpBaseFromAddr(addr string) string {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return "http://127.0.0.1:8081"
	}
	if strings.HasPrefix(addr, ":") {
		return "http://127.0.0.1" + addr
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		return "http://" + addr
	}
	switch strings.TrimSpace(host) {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return "http://" + host + ":" + port
}

func runDevCommand(ctx context.Context) int {
	// Lowest-effort path: if you run "mamacord dev" you get the admin API too.
	_ = os.Setenv("MAMACORD_PROD_MODE", "0")
	if strings.TrimSpace(os.Getenv("MAMACORD_ADMIN_ADDR")) == "" {
		_ = os.Setenv("MAMACORD_ADMIN_ADDR", "127.0.0.1:8081")
	}
	if strings.TrimSpace(os.Getenv("MAMACORD_ALLOW_UNSIGNED_PLUGINS")) == "" {
		_ = os.Setenv("MAMACORD_ALLOW_UNSIGNED_PLUGINS", "1")
	}

	cfg, err := config.LoadFromEnv()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
	logger, err := logging.New(cfg.LogLevel)
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
	base := httpBaseFromAddr(cfg.AdminAddr)
	_, _ = fmt.Fprintf(os.Stdout, "admin_setup_url: %s/api/setup\n", base)
	_, _ = fmt.Fprintf(os.Stdout, "dashboard_url: %s/\n", base)
	_, _ = os.Stdout.WriteString("dashboard_dev: cd apps/dashboard && bun run dev\n")
	_, _ = os.Stdout.WriteString("dashboard_dev_note: you can open dashboard_url (recommended) or http://127.0.0.1:5173/ (Vite proxies /api)\n")

	if runErr := run(ctx, logger, cfg); runErr != nil {
		logger.ErrorContext(ctx, "fatal", slog.String("err", runErr.Error()))
		return 1
	}
	return 0
}

func runInitCommand(args []string) int {
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	mode := fs.String("mode", "dev", "mode: dev|prod")
	force := fs.Bool("force", false, "overwrite existing files")
	discordToken := fs.String("discord-token", "", "discord bot token")
	clientID := fs.String("client-id", "", "discord oauth client id")
	clientSecret := fs.String("client-secret", "", "discord oauth client secret")
	adminAddr := fs.String("admin-addr", "", "admin api listen addr (host:port)")
	sessionSecret := fs.String("session-secret", "", "session secret (32+ chars)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	rawMode := strings.ToLower(strings.TrimSpace(*mode))
	modeKind := ""
	switch rawMode {
	case "dev":
		modeKind = "dev"
	case "prod":
		modeKind = "prod"
	default:
		_, _ = os.Stderr.WriteString("init: --mode must be dev|prod\n")
		return 1
	}

	rootEnv := ".env.dev"
	if modeKind == "prod" {
		rootEnv = ".env.prod"
	}

	if !*force {
		if _, err := os.Stat(rootEnv); err == nil {
			_, _ = os.Stderr.WriteString("init: " + rootEnv + " already exists (use --force to overwrite)\n")
			return 1
		}
	}

	if strings.TrimSpace(*adminAddr) == "" && modeKind == "dev" {
		*adminAddr = "127.0.0.1:8081"
	}
	if strings.TrimSpace(*sessionSecret) == "" {
		*sessionSecret = genHexSecret(32)
	}

	root := strings.Builder{}
	root.WriteString("# mamacord\n")
	root.WriteString("DISCORD_TOKEN=" + strings.TrimSpace(*discordToken) + "\n")
	if modeKind == "prod" {
		root.WriteString("MAMACORD_PROD_MODE=1\n")
		root.WriteString("MAMACORD_ALLOW_UNSIGNED_PLUGINS=0\n")
	} else {
		root.WriteString("MAMACORD_PROD_MODE=0\n")
		root.WriteString("MAMACORD_ALLOW_UNSIGNED_PLUGINS=1\n")
	}
	root.WriteString("MAMACORD_STORAGE_BACKEND=postgres\n")
	root.WriteString("MAMACORD_POSTGRES_DSN=postgres://mamacord:secret@127.0.0.1:5432/mamacord?sslmode=disable\n")
	if strings.TrimSpace(*adminAddr) != "" {
		root.WriteString("\n# Admin API + dashboard OAuth\n")
		root.WriteString("MAMACORD_ADMIN_ADDR=" + strings.TrimSpace(*adminAddr) + "\n")
		root.WriteString("MAMACORD_DASHBOARD_CLIENT_ID=" + strings.TrimSpace(*clientID) + "\n")
		root.WriteString("MAMACORD_DASHBOARD_CLIENT_SECRET=" + strings.TrimSpace(*clientSecret) + "\n")
		root.WriteString("MAMACORD_DASHBOARD_SESSION_SECRET=" + strings.TrimSpace(*sessionSecret) + "\n")
	}

	if err := os.WriteFile(rootEnv, []byte(root.String()), 0o600); err != nil {
		_, _ = os.Stderr.WriteString("init: write " + rootEnv + ": " + err.Error() + "\n")
		return 1
	}

	_, _ = fmt.Fprintf(os.Stdout, "wrote: %s\n", rootEnv)
	if modeKind == "dev" {
		_, _ = os.Stdout.WriteString("next: mamacord dev\n")
		_, _ = os.Stdout.WriteString("next: cd apps/dashboard && bun install && bun run dev\n")
	}
	return 0
}

func genHexSecret(nBytes int) string {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return strings.Repeat("x", nBytes)
	}
	return hex.EncodeToString(buf)
}

func doctorStorageTarget(cfg config.Config) string {
	switch cfg.StorageBackend {
	case config.StorageBackendPostgres:
		dsn := strings.TrimSpace(cfg.PostgresDSN)
		if dsn == "" {
			return ""
		}
		parsed, err := url.Parse(dsn)
		if err != nil {
			return "<invalid postgres dsn>"
		}
		if parsed.User != nil {
			username := parsed.User.Username()
			_, hasPassword := parsed.User.Password()
			if username != "" {
				parsed.User = url.User(username)
				redacted := parsed.String()
				if hasPassword {
					return strings.Replace(redacted, username+"@", username+":***@", 1)
				}
				return redacted
			}
			parsed.User = nil
		}
		return parsed.String()
	default:
		return ""
	}
}

func forbiddenDotenvFile() string {
	// Hardline policy: only .env.dev and .env.prod are permitted.
	forbidden := []string{
		".env.local",
		".env.development",
		".env.production",
		".env.production.local",
		".env.dev.local",
		".env.prod.local",
		".env",
	}
	for _, path := range forbidden {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func run(ctx context.Context, logger *slog.Logger, cfg config.Config) error {
	mamacord, err := app.New(app.Dependencies{
		Logger: logger,
		Config: cfg,
	})
	if err != nil {
		return err
	}
	defer mamacord.Close()

	if startErr := mamacord.Start(ctx); startErr != nil {
		if errors.Is(startErr, context.Canceled) {
			return nil
		}
		return startErr
	}

	return nil
}

func runMigrateCommand(ctx context.Context, args []string) int {
	cfg, err := config.LoadStorageFromEnv()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}

	if len(args) == 0 {
		printMigrateUsage()
		return 1
	}

	switch args[0] {
	case "status":
		status, err := storagebootstrap.MigrationStatus(ctx, cfg)
		if err != nil {
			_, _ = os.Stderr.WriteString(err.Error() + "\n")
			return 1
		}
		printStatus(status)
		return 0
	case "up":
		status, err := storagebootstrap.MigrateUp(ctx, cfg)
		if err != nil {
			_, _ = os.Stderr.WriteString(err.Error() + "\n")
			return 1
		}
		printStatus(status)
		return 0
	default:
		printMigrateUsage()
		return 1
	}
}

func printStatus(status migrate.Status) {
	_, _ = fmt.Fprintf(os.Stdout, "current_version: %d\n", status.CurrentVersion)
	if len(status.Applied) == 0 {
		_, _ = os.Stdout.WriteString("applied: none\n")
	} else {
		_, _ = os.Stdout.WriteString("applied:\n")
		for _, item := range status.Applied {
			_, _ = fmt.Fprintf(os.Stdout, "  - %03d %s (%s)\n", item.Version, item.Name, item.Kind)
		}
	}
	if len(status.Pending) == 0 {
		_, _ = os.Stdout.WriteString("pending: none\n")
		return
	}
	_, _ = os.Stdout.WriteString("pending:\n")
	for _, item := range status.Pending {
		_, _ = fmt.Fprintf(os.Stdout, "  - %03d %s (%s)\n", item.Version, item.Name, item.Kind)
	}
}

func printMigrateUsage() {
	_, _ = os.Stderr.WriteString(
		"usage:\n" +
			"  mamacord migrate status\n" +
			"  mamacord migrate up\n" +
			"",
	)
}

func printVersion() {
	info := buildinfo.Current()
	_, _ = fmt.Fprintf(os.Stdout, "version: %s\n", info.Version)
	_, _ = fmt.Fprintf(os.Stdout, "description: %s\n", info.Description)
	if info.Repository != "" {
		_, _ = fmt.Fprintf(os.Stdout, "repository: %s\n", info.Repository)
	}
	if info.DeveloperURL != "" {
		_, _ = fmt.Fprintf(os.Stdout, "developer_url: %s\n", info.DeveloperURL)
	}
	if info.SupportServerURL != "" {
		_, _ = fmt.Fprintf(os.Stdout, "support_server_url: %s\n", info.SupportServerURL)
	}
	if info.MascotImageURL != "" {
		_, _ = fmt.Fprintf(os.Stdout, "mascot_image_url: %s\n", info.MascotImageURL)
	}
}

func runSignPluginCommand(args []string) int {
	fs := flag.NewFlagSet("sign-plugin", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	dir := fs.String("dir", "", "plugin directory to sign")
	keyID := fs.String("key-id", "", "signer key id")
	privateKeyFile := fs.String("private-key-file", "", "file containing base64 ed25519 private key bytes or seed")
	out := fs.String("out", "", "output signature path (default: active plugin bundle dir/signature.json)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if *dir == "" || *keyID == "" || *privateKeyFile == "" {
		_, _ = os.Stderr.WriteString("usage: mamacord sign-plugin --dir <plugin_root> --key-id <key_id> --private-key-file <path> [--out <signature.json>]\n")
		return 1
	}

	privateKey, err := pluginhost.ReadEd25519PrivateKeyFile(*privateKeyFile)
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}

	cfg, err := config.LoadBundleFromEnv()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
	bundleRepo, err := bundles.Open(cfg)
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}

	pluginDir, err := bundleRepo.ResolveBundleDir(*dir)
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}

	sig, publicKey, err := pluginhost.SignDir(pluginDir, *keyID, privateKey)
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}

	target := strings.TrimSpace(*out)

	payload := map[string]any{
		"$schema":       pluginhost.SignatureSchemaURL,
		"key_id":        sig.KeyID,
		"hash_b64":      sig.HashB64,
		"signature_b64": sig.SignatureB64,
		"algorithm":     sig.Algorithm,
	}
	bytes, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
	bytes = append(bytes, '\n')
	if target == "" {
		target, err = bundleRepo.WriteBundleSignature(*dir, bytes)
		if err != nil {
			_, _ = os.Stderr.WriteString(err.Error() + "\n")
			return 1
		}
	} else if err := os.WriteFile(target, bytes, 0o644); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}

	_, _ = fmt.Fprintf(os.Stdout, "signature: %s\n", target)
	_, _ = fmt.Fprintf(os.Stdout, "public_key_b64: %s\n", base64.StdEncoding.EncodeToString(publicKey))
	return 0
}

func runGenSigningKeyCommand(args []string) int {
	fs := flag.NewFlagSet("gen-signing-key", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	keyID := fs.String("key-id", "", "signer key id")
	privateKeyFile := fs.String("private-key-file", "", "output private key file (default: ./data/keys/<key_id>.key)")
	trustedKeysFile := fs.String("trusted-keys-file", "", "trusted keys file to create/update (default: ./config/trusted_keys.json)")
	if err := fs.Parse(args); err != nil {
		return 1
	}

	if strings.TrimSpace(*keyID) == "" {
		_, _ = os.Stderr.WriteString("usage: mamacord gen-signing-key --key-id <key_id> [--private-key-file <path>] [--trusted-keys-file <path>]\n")
		return 1
	}

	keyPath := strings.TrimSpace(*privateKeyFile)
	if keyPath == "" {
		keyPath = filepath.Join(".", "data", "keys", strings.TrimSpace(*keyID)+".key")
	}
	trustPath := strings.TrimSpace(*trustedKeysFile)
	if trustPath == "" {
		trustPath = strings.TrimSpace(os.Getenv("MAMACORD_TRUSTED_KEYS_FILE"))
	}
	if trustPath == "" {
		trustPath = "./config/trusted_keys.json"
	}

	publicKey, privateKey, err := pluginhost.GenerateEd25519Key()
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
	if err := pluginhost.WriteEd25519PrivateKeyFile(keyPath, privateKey); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}

	publicKeyB64 := base64.StdEncoding.EncodeToString(publicKey)
	if err := pluginhost.UpsertTrustedKeyFile(trustPath, pluginhost.TrustedKey{
		KeyID:        strings.TrimSpace(*keyID),
		PublicKeyB64: publicKeyB64,
	}); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}

	_, _ = fmt.Fprintf(os.Stdout, "key_id: %s\n", strings.TrimSpace(*keyID))
	_, _ = fmt.Fprintf(os.Stdout, "private_key_file: %s\n", keyPath)
	_, _ = fmt.Fprintf(os.Stdout, "trusted_keys_file: %s\n", trustPath)
	_, _ = fmt.Fprintf(os.Stdout, "public_key_b64: %s\n", publicKeyB64)
	return 0
}

func runPluginsCommand(ctx context.Context, args []string) int {
	if len(args) == 0 {
		_, _ = os.Stderr.WriteString("usage: mamacord plugins <sources|search|install|update|uninstall|trust>\n")
		return 1
	}

	manager, closeFn, err := openMarketplaceManager(ctx)
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
	defer closeFn()

	switch args[0] {
	case "sources":
		return runPluginSourcesCommand(ctx, manager, args[1:])
	case "search":
		return runPluginSearchCommand(ctx, manager, args[1:])
	case "install":
		return runPluginInstallCommand(ctx, manager, args[1:])
	case "update":
		return runPluginUpdateCommand(ctx, manager, args[1:])
	case "uninstall":
		return runPluginUninstallCommand(ctx, manager, args[1:])
	case "trust":
		return runPluginTrustCommand(ctx, manager, args[1:])
	default:
		_, _ = os.Stderr.WriteString("usage: mamacord plugins <sources|search|install|update|uninstall|trust>\n")
		return 1
	}
}

func openMarketplaceManager(ctx context.Context) (*marketplace.Manager, func(), error) {
	cfg, err := config.LoadStorageFromEnv()
	if err != nil {
		return nil, nil, err
	}
	logger, err := logging.New(cfg.LogLevel)
	if err != nil {
		return nil, nil, err
	}
	store, _, err := storagebootstrap.OpenRuntimeStore(ctx, cfg)
	if err != nil {
		return nil, nil, err
	}
	bundleRepo, err := bundles.Open(cfg)
	if err != nil {
		_ = store.Close()
		return nil, nil, err
	}
	manager, err := marketplace.New(marketplace.Options{
		Logger:            logger,
		Store:             store,
		Bundles:           bundleRepo,
		BundledPluginsDir: cfg.BundledPluginsDir,
		UserPluginsDir:    cfg.UserPluginsDir,
		TrustedKeysFile:   cfg.TrustedKeysFile,
		CacheDir:          cfg.MarketplaceCacheDir,
		ProdMode:          cfg.ProdMode,
		AllowUnsigned:     cfg.AllowUnsignedPlugins,
	})
	if err != nil {
		_ = store.Close()
		return nil, nil, err
	}
	return manager, func() { _ = store.Close() }, nil
}

func runPluginSourcesCommand(ctx context.Context, manager *marketplace.Manager, args []string) int {
	if len(args) == 0 {
		_, _ = os.Stderr.WriteString("usage: mamacord plugins sources <list|add|remove|sync>\n")
		return 1
	}
	switch args[0] {
	case "list":
		items, err := manager.ListSources(ctx)
		if err != nil {
			_, _ = os.Stderr.WriteString(err.Error() + "\n")
			return 1
		}
		for _, item := range items {
			_, _ = fmt.Fprintf(os.Stdout, "%s %s ref=%s enabled=%t revision=%s\n", item.SourceID, item.GitURL, item.GitRef, item.Enabled, item.LastRevision)
		}
		return 0
	case "add":
		fs := flag.NewFlagSet("plugins sources add", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sourceID := fs.String("source-id", "", "source id")
		url := fs.String("url", "", "git URL")
		ref := fs.String("ref", "", "git ref")
		subdir := fs.String("subdir", "", "git subdir")
		tokenEnvVar := fs.String("token-env-var", "", "env var name containing HTTPS token")
		enabled := fs.Bool("enabled", true, "enable this source")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		item, err := manager.UpsertSource(ctx, marketplace.SourceUpsert{
			SourceID:    strings.TrimSpace(*sourceID),
			GitURL:      strings.TrimSpace(*url),
			GitRef:      strings.TrimSpace(*ref),
			GitSubdir:   strings.TrimSpace(*subdir),
			TokenEnvVar: strings.TrimSpace(*tokenEnvVar),
			Enabled:     *enabled,
		})
		if err != nil {
			_, _ = os.Stderr.WriteString(err.Error() + "\n")
			return 1
		}
		_, _ = fmt.Fprintf(os.Stdout, "source_id: %s\n", item.SourceID)
		return 0
	case "remove":
		fs := flag.NewFlagSet("plugins sources remove", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sourceID := fs.String("source-id", "", "source id")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		if err := manager.DeleteSource(ctx, *sourceID); err != nil {
			_, _ = os.Stderr.WriteString(err.Error() + "\n")
			return 1
		}
		return 0
	case "sync":
		fs := flag.NewFlagSet("plugins sources sync", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		sourceID := fs.String("source-id", "", "source id")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		resp, err := manager.SyncSource(ctx, *sourceID)
		if err != nil {
			_, _ = os.Stderr.WriteString(err.Error() + "\n")
			return 1
		}
		_, _ = fmt.Fprintf(os.Stdout, "source_id: %s\nrevision: %s\n", resp.SourceID, resp.Revision)
		return 0
	default:
		_, _ = os.Stderr.WriteString("usage: mamacord plugins sources <list|add|remove|sync>\n")
		return 1
	}
}

func runPluginSearchCommand(ctx context.Context, manager *marketplace.Manager, args []string) int {
	fs := flag.NewFlagSet("plugins search", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sourceID := fs.String("source-id", "", "source id")
	term := fs.String("term", "", "search term")
	refresh := fs.Bool("refresh", false, "sync before searching")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	results, err := manager.Search(ctx, marketplace.SearchQuery{
		SourceID: strings.TrimSpace(*sourceID),
		Term:     strings.TrimSpace(*term),
		Refresh:  *refresh,
	})
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
	for _, item := range results {
		_, _ = fmt.Fprintf(os.Stdout, "%s %s %s %s\n", item.PluginID, item.SourceID, item.Version, item.SignatureState)
	}
	return 0
}

func runPluginInstallCommand(ctx context.Context, manager *marketplace.Manager, args []string) int {
	fs := flag.NewFlagSet("plugins install", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	sourceID := fs.String("source-id", "", "source id")
	pluginID := fs.String("plugin-id", "", "plugin id")
	force := fs.Bool("force", false, "replace existing marketplace install")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	resp, err := manager.Install(ctx, marketplace.InstallRequest{
		SourceID: strings.TrimSpace(*sourceID),
		PluginID: strings.TrimSpace(*pluginID),
		Force:    *force,
	})
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
	_, _ = fmt.Fprintf(os.Stdout, "plugin_id: %s\nsource_id: %s\nrevision: %s\n", resp.PluginID, resp.SourceID, resp.GitRevision)
	return 0
}

func runPluginUpdateCommand(ctx context.Context, manager *marketplace.Manager, args []string) int {
	fs := flag.NewFlagSet("plugins update", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pluginID := fs.String("plugin-id", "", "plugin id")
	force := fs.Bool("force", false, "replace local modifications")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	resp, err := manager.Update(ctx, marketplace.UpdateRequest{
		PluginID: strings.TrimSpace(*pluginID),
		Force:    *force,
	})
	if err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
	_, _ = fmt.Fprintf(os.Stdout, "plugin_id: %s\nsource_id: %s\nrevision: %s\n", resp.PluginID, resp.SourceID, resp.GitRevision)
	return 0
}

func runPluginUninstallCommand(ctx context.Context, manager *marketplace.Manager, args []string) int {
	fs := flag.NewFlagSet("plugins uninstall", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	pluginID := fs.String("plugin-id", "", "plugin id")
	if err := fs.Parse(args); err != nil {
		return 1
	}
	if err := manager.Uninstall(ctx, marketplace.UninstallRequest{PluginID: strings.TrimSpace(*pluginID)}); err != nil {
		_, _ = os.Stderr.WriteString(err.Error() + "\n")
		return 1
	}
	return 0
}

func runPluginTrustCommand(ctx context.Context, manager *marketplace.Manager, args []string) int {
	if len(args) == 0 {
		_, _ = os.Stderr.WriteString("usage: mamacord plugins trust <signer|vendor>\n")
		return 1
	}
	switch args[0] {
	case "signer":
		fs := flag.NewFlagSet("plugins trust signer", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		keyID := fs.String("key-id", "", "key id")
		publicKeyB64 := fs.String("public-key-b64", "", "base64 public key")
		vendorID := fs.String("vendor-id", "", "vendor id")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		if err := manager.TrustSigner(ctx, marketplace.TrustSignerRequest{
			KeyID:        strings.TrimSpace(*keyID),
			PublicKeyB64: strings.TrimSpace(*publicKeyB64),
			VendorID:     strings.TrimSpace(*vendorID),
		}); err != nil {
			_, _ = os.Stderr.WriteString(err.Error() + "\n")
			return 1
		}
		return 0
	case "vendor":
		fs := flag.NewFlagSet("plugins trust vendor", flag.ContinueOnError)
		fs.SetOutput(os.Stderr)
		vendorID := fs.String("vendor-id", "", "vendor id")
		name := fs.String("name", "", "vendor name")
		sourceID := fs.String("source-id", "", "source id")
		trustedKeysPath := fs.String("trusted-keys-path", "", "trusted keys file path")
		websiteURL := fs.String("website-url", "", "website URL")
		supportURL := fs.String("support-url", "", "support URL")
		if err := fs.Parse(args[1:]); err != nil {
			return 1
		}
		resp, err := manager.TrustVendor(ctx, marketplace.TrustVendorRequest{
			VendorID:        strings.TrimSpace(*vendorID),
			Name:            strings.TrimSpace(*name),
			SourceID:        strings.TrimSpace(*sourceID),
			TrustedKeysPath: strings.TrimSpace(*trustedKeysPath),
			WebsiteURL:      strings.TrimSpace(*websiteURL),
			SupportURL:      strings.TrimSpace(*supportURL),
		})
		if err != nil {
			_, _ = os.Stderr.WriteString(err.Error() + "\n")
			return 1
		}
		_, _ = fmt.Fprintf(os.Stdout, "vendor_id: %s\nkeys: %s\n", resp.VendorID, strings.Join(resp.KeyIDs, ","))
		return 0
	default:
		_, _ = os.Stderr.WriteString("usage: mamacord plugins trust <signer|vendor>\n")
		return 1
	}
}
