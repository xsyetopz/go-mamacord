package service

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/buildinfo"
	"github.com/xsyetopz/go-mamacord/internal/bundles"
	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	"github.com/xsyetopz/go-mamacord/internal/config"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	migrate "github.com/xsyetopz/go-mamacord/internal/migration"
	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	"github.com/xsyetopz/go-mamacord/internal/ops"
	"github.com/xsyetopz/go-mamacord/internal/permissions"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
)

var pluginIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

type ServiceCore struct {
	Logger      *slog.Logger
	Config      config.Config
	Bundles     bundles.Repository
	Snapshot    func() ops.Snapshot
	BuildInfo   func() buildinfo.Info
	OwnerStatus func() OwnerStatus
}

type ServiceAdmins struct {
	ModuleAdmin moduleapi.Admin
	PluginAdmin commandruntime.PluginAdmin
	Marketplace commandruntime.MarketplaceAdmin
}

type ServiceStores struct {
	TrustedSigners storage.TrustedSignerStore
	PluginInstalls storage.PluginInstallStore
}

type Service struct {
	ServiceCore
	ServiceAdmins
	ServiceStores
}

type OwnerStatus struct {
	Configured      bool
	Resolved        bool
	Source          string
	EffectiveUserID *uint64
}

func optionalSnowflake(value *uint64) *Snowflake {
	if value == nil {
		return nil
	}
	v := Snowflake(*value)
	return &v
}

type StatusResponse struct {
	Snapshot SnapshotResponse `json:"snapshot"`
	Build    BuildResponse    `json:"build"`
	Config   StatusConfig     `json:"config"`
	Setup    SetupResponse    `json:"setup"`
}

type SnapshotResponse struct {
	RuntimeStatusResponse
	ModuleStatusResponse
	PluginStatusResponse
	CommandStatusResponse
	ActivityStatusResponse
}

type RuntimeStatusResponse struct {
	Ready             bool   `json:"ready"`
	StartedAt         string `json:"started_at"`
	MigrationVersion  int    `json:"migration_version"`
	ProdMode          bool   `json:"prod_mode"`
	DiscordStartError string `json:"discord_start_error,omitempty"`
}

type ModuleStatusResponse struct {
	ModuleCount        int `json:"module_count"`
	EnabledModuleCount int `json:"enabled_module_count"`
}

type PluginStatusResponse struct {
	PluginCount        int `json:"plugin_count"`
	EnabledPluginCount int `json:"enabled_plugin_count"`
}

type CommandStatusResponse struct {
	BuiltinCommandCount int `json:"builtin_command_count"`
	SlashCommandCount   int `json:"slash_command_count"`
	UserCommandCount    int `json:"user_command_count"`
	MessageCommandCount int `json:"message_command_count"`
}

type ActivityStatusResponse struct {
	InteractionsTotal   uint64 `json:"interactions_total"`
	InteractionFailures uint64 `json:"interaction_failures"`
	PluginFailures      uint64 `json:"plugin_failures"`
	AutomationFailures  uint64 `json:"automation_failures"`
	ReminderFailures    uint64 `json:"reminder_failures"`
}

type BuildResponse struct {
	Version          string `json:"version"`
	Repository       string `json:"repository,omitempty"`
	Description      string `json:"description,omitempty"`
	DeveloperURL     string `json:"developer_url,omitempty"`
	SupportServerURL string `json:"support_server_url,omitempty"`
	MascotImageURL   string `json:"mascot_image_url,omitempty"`
}

type StatusConfig struct {
	StorageStatusConfig
	FileStatusConfig
	EndpointStatusConfig
	RuntimeStatusConfig
}

type StorageStatusConfig struct {
	StorageBackend string `json:"storage_backend"`
	StorageTarget  string `json:"storage_target"`
	MigrationsDir  string `json:"migrations_dir"`
}

type FileStatusConfig struct {
	LocalesDir        string `json:"locales_dir"`
	BundledPluginsDir string `json:"bundled_plugins_dir"`
	UserPluginsDir    string `json:"user_plugins_dir"`
	PermissionsFile   string `json:"permissions_file"`
	ModulesFile       string `json:"modules_file"`
	TrustedKeysFile   string `json:"trusted_keys_file"`
}

type EndpointStatusConfig struct {
	OpsAddr   string `json:"ops_addr"`
	AdminAddr string `json:"admin_addr"`
}

type RuntimeStatusConfig struct {
	RuntimeRoles            []string   `json:"runtime_roles"`
	DevGuildID              *Snowflake `json:"dev_guild_id,omitempty"`
	CommandRegistrationMode string     `json:"command_registration_mode"`
	ProdMode                bool       `json:"prod_mode"`
	AllowUnsignedPlugins    bool       `json:"allow_unsigned_plugins"`
}

type SetupResponse struct {
	AdminSetupResponse
	OwnerSetupResponse
	SigningSetupResponse
	EndpointSetupResponse
	CredentialSetupResponse
	Hints []string `json:"hints"`
}

type AdminSetupResponse struct {
	AdminEnabled   bool `json:"admin_enabled"`
	AuthConfigured bool `json:"auth_configured"`
	LoginReady     bool `json:"login_ready"`
}

type OwnerSetupResponse struct {
	OwnerConfigured      bool       `json:"owner_configured"`
	OwnerResolved        bool       `json:"owner_resolved"`
	OwnerSource          string     `json:"owner_source"`
	EffectiveOwnerUserID *Snowflake `json:"effective_owner_user_id,omitempty"`
}

type SigningSetupResponse struct {
	SigningConfigured     bool `json:"signing_configured"`
	TrustedKeysConfigured bool `json:"trusted_keys_configured"`
}

type EndpointSetupResponse struct {
	AdminAddr          string `json:"admin_addr"`
	AppOrigin          string `json:"app_origin"`
	RedirectURL        string `json:"redirect_url"`
	InstallRedirectURL string `json:"install_redirect_url"`
}

type CredentialSetupResponse struct {
	HasClientID      bool `json:"has_client_id"`
	HasClientSecret  bool `json:"has_client_secret"`
	HasSessionSecret bool `json:"has_session_secret"`
}

type ModuleResponse struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Kind           string   `json:"kind"`
	Runtime        string   `json:"runtime"`
	Enabled        bool     `json:"enabled"`
	DefaultEnabled bool     `json:"default_enabled"`
	Toggleable     bool     `json:"toggleable"`
	Signed         bool     `json:"signed"`
	Source         string   `json:"source"`
	Commands       []string `json:"commands"`
}

type PluginSummary struct {
	PluginIdentitySummary
	PluginStateSummary
	PluginLocationSummary
	PluginProvenanceSummary
}

type PluginIdentitySummary struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Version  string   `json:"version"`
	Commands []string `json:"commands"`
}

type PluginStateSummary struct {
	Loaded           bool `json:"loaded"`
	Signed           bool `json:"signed"`
	HasSignatureFile bool `json:"has_signature_file"`
	LocalModified    bool `json:"local_modified"`
}

type PluginLocationSummary struct {
	PluginRoot        string `json:"plugin_root"`
	Bundled           bool   `json:"bundled"`
	BundleRelativeDir string `json:"bundle_relative_dir,omitempty"`
}

type PluginProvenanceSummary struct {
	ProvenanceKind string `json:"provenance_kind"`
	SourceID       string `json:"source_id,omitempty"`
	GitRevision    string `json:"git_revision,omitempty"`
	SignatureState string `json:"signature_state,omitempty"`
}

type MarketplaceSourcesResponse struct {
	Sources []marketplace.Source `json:"sources"`
}

type MarketplaceInstallRequest struct {
	SourceID string `json:"source_id"`
	PluginID string `json:"plugin_id"`
	Force    bool   `json:"force,omitempty"`
}

type MarketplaceUpdateRequest struct {
	PluginID string `json:"plugin_id"`
	Force    bool   `json:"force,omitempty"`
}

type MarketplaceUninstallRequest struct {
	PluginID string `json:"plugin_id"`
}

type MarketplaceTrustSignerRequest struct {
	KeyID        string `json:"key_id"`
	PublicKeyB64 string `json:"public_key_b64"`
	VendorID     string `json:"vendor_id,omitempty"`
}

type MarketplaceTrustVendorRequest struct {
	VendorID        string `json:"vendor_id"`
	Name            string `json:"name"`
	WebsiteURL      string `json:"website_url,omitempty"`
	SupportURL      string `json:"support_url,omitempty"`
	TrustedKeysPath string `json:"trusted_keys_path,omitempty"`
	SourceID        string `json:"source_id,omitempty"`
}

type TrustedKeysResponse struct {
	FileKeys []TrustedKeyResponse    `json:"file_keys"`
	DBKeys   []TrustedSignerResponse `json:"db_keys"`
}

type TrustedKeyResponse struct {
	KeyID        string `json:"key_id"`
	PublicKeyB64 string `json:"public_key_b64"`
}

type TrustedSignerResponse struct {
	KeyID        string `json:"key_id"`
	PublicKeyB64 string `json:"public_key_b64"`
	AddedAt      string `json:"added_at"`
}

type MigrationStatusResponse struct {
	CurrentVersion int                `json:"current_version"`
	Applied        []MigrationItemDTO `json:"applied"`
	Pending        []MigrationItemDTO `json:"pending"`
}

type MigrationItemDTO struct {
	Version int    `json:"version"`
	Name    string `json:"name"`
	Kind    string `json:"kind"`
}

type PluginScaffoldRequest struct {
	ID                 string                  `json:"id"`
	Name               string                  `json:"name"`
	Version            string                  `json:"version"`
	Locale             string                  `json:"locale"`
	CommandName        string                  `json:"command_name"`
	CommandDescription string                  `json:"command_description"`
	ResponseMessage    string                  `json:"response_message"`
	NetworkHosts       []string                `json:"network_hosts"`
	Permissions        permissions.Permissions `json:"permissions"`
	Sign               bool                    `json:"sign"`
}

type PluginScaffoldResponse struct {
	ID        string   `json:"id"`
	Dir       string   `json:"dir"`
	Files     []string `json:"files"`
	Signed    bool     `json:"signed"`
	Signature string   `json:"signature,omitempty"`
}

type SessionResponse struct {
	Authenticated bool `json:"authenticated"`
	User          struct {
		ID        Snowflake `json:"id"`
		Username  string    `json:"username"`
		Name      string    `json:"name"`
		AvatarURL string    `json:"avatar_url,omitempty"`
	} `json:"user"`
	IsOwner   bool   `json:"is_owner"`
	CSRFToken string `json:"csrf_token"`
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func fallbackString(primary, secondary string) string {
	if strings.TrimSpace(primary) != "" {
		return primary
	}
	return strings.TrimSpace(secondary)
}

func (s *Service) userPluginsDir() string {
	return strings.TrimSpace(s.Config.Bundles.UserPluginsDir)
}

func (s *Service) bundleRepo() bundles.Repository {
	if s != nil && s.Bundles != nil {
		return s.Bundles
	}
	return bundles.NewLocalRepository()
}

func (s *Service) pluginDir(pluginID string) (string, error) {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return "", errors.New("plugin id is required")
	}
	for _, root := range []string{
		strings.TrimSpace(s.Config.Bundles.UserPluginsDir),
		strings.TrimSpace(s.Config.Bundles.BundledPluginsDir),
	} {
		if root == "" {
			continue
		}
		dir := filepath.Join(root, pluginID)
		if fileExists(filepath.Join(dir, bundles.StateFileName)) {
			return dir, nil
		}
	}
	return filepath.Join(strings.TrimSpace(s.Config.Bundles.UserPluginsDir), pluginID), nil
}

func (s *Service) setupResponse(includeHints bool) SetupResponse {
	ownerStatus := OwnerStatus{
		Configured: s.Config.Discord.OwnerUserID != nil,
		Resolved:   s.Config.Discord.OwnerUserID != nil,
		Source:     "unresolved",
	}
	if s.Config.Discord.OwnerUserID != nil {
		ownerStatus.Source = "config_fallback"
		ownerStatus.EffectiveUserID = s.Config.Discord.OwnerUserID
	}
	if s.OwnerStatus != nil {
		ownerStatus = s.OwnerStatus()
	}

	resp := SetupResponse{
		AdminSetupResponse: AdminSetupResponse{
			AdminEnabled:   s.Config.ControlAPIEnabled(),
			AuthConfigured: dashboardAuthReady(s.Config),
			LoginReady:     dashboardAuthReady(s.Config),
		},
		OwnerSetupResponse: OwnerSetupResponse{
			OwnerConfigured:      ownerStatus.Configured,
			OwnerResolved:        ownerStatus.Resolved,
			OwnerSource:          strings.TrimSpace(ownerStatus.Source),
			EffectiveOwnerUserID: optionalSnowflake(ownerStatus.EffectiveUserID),
		},
		SigningSetupResponse: SigningSetupResponse{
			SigningConfigured: signingReady(s.Config),
		},
		EndpointSetupResponse: EndpointSetupResponse{
			AdminAddr: strings.TrimSpace(s.Config.Runtime.AdminAddr),
			// Public origins are filled by the HTTP layer.
			AppOrigin:   "",
			RedirectURL: "",
		},
		CredentialSetupResponse: CredentialSetupResponse{
			HasClientID:      strings.TrimSpace(s.Config.Dashboard.ClientID) != "",
			HasClientSecret:  strings.TrimSpace(s.Config.Dashboard.ClientSecret) != "",
			HasSessionSecret: len(strings.TrimSpace(s.Config.Dashboard.SessionSecret)) >= 32,
		},
		// Always encode as JSON array, never null (nil slice -> null).
		Hints: []string{},
	}
	if includeHints {
		resp.Hints = setupHints(resp)
	}
	return resp
}

func setupHints(resp SetupResponse) []string {
	hints := make([]string, 0, 6)
	if !resp.AdminEnabled {
		if strings.TrimSpace(resp.AdminAddr) == "" {
			hints = append(hints, "Set MAMACORD_ADMIN_ADDR to start the admin API.")
		} else {
			hints = append(hints, "Add control to MAMACORD_RUNTIME_ROLES to start the admin API.")
		}
	}
	if !resp.HasClientID {
		hints = append(hints, "Set MAMACORD_DASHBOARD_CLIENT_ID.")
	}
	if !resp.HasClientSecret {
		hints = append(hints, "Set MAMACORD_DASHBOARD_CLIENT_SECRET.")
	}
	if !resp.HasSessionSecret {
		hints = append(hints, "Set MAMACORD_DASHBOARD_SESSION_SECRET to at least 32 characters.")
	}
	if !resp.OwnerResolved {
		hints = append(hints, "Owner access is unavailable. Discord owner lookup did not resolve an owner, and no OWNER_USER_ID fallback is configured.")
	}
	return hints
}

func snapshotResponse(snap ops.Snapshot) SnapshotResponse {
	return SnapshotResponse{
		RuntimeStatusResponse: RuntimeStatusResponse{
			Ready:             snap.Runtime.Ready,
			StartedAt:         formatTime(snap.Runtime.StartedAt),
			MigrationVersion:  snap.Runtime.MigrationVersion,
			ProdMode:          snap.Runtime.ProdMode,
			DiscordStartError: strings.TrimSpace(snap.Runtime.DiscordStartError),
		},
		ModuleStatusResponse: ModuleStatusResponse{
			ModuleCount:        snap.Modules.Total,
			EnabledModuleCount: snap.Modules.Enabled,
		},
		PluginStatusResponse: PluginStatusResponse{
			PluginCount:        snap.Plugins.Total,
			EnabledPluginCount: snap.Plugins.Enabled,
		},
		CommandStatusResponse: CommandStatusResponse{
			BuiltinCommandCount: snap.Commands.Builtin,
			SlashCommandCount:   snap.Commands.Slash,
			UserCommandCount:    snap.Commands.User,
			MessageCommandCount: snap.Commands.Message,
		},
		ActivityStatusResponse: ActivityStatusResponse{
			InteractionsTotal:   snap.Activity.InteractionsTotal,
			InteractionFailures: snap.Activity.InteractionFailures,
			PluginFailures:      snap.Activity.PluginFailures,
			AutomationFailures:  snap.Activity.AutomationFailures,
			ReminderFailures:    snap.Activity.ReminderFailures,
		},
	}
}

func buildResponse(info buildinfo.Info) BuildResponse {
	return BuildResponse{
		Version:          info.Version,
		Repository:       info.Repository,
		Description:      info.Description,
		DeveloperURL:     info.DeveloperURL,
		SupportServerURL: info.SupportServerURL,
		MascotImageURL:   info.MascotImageURL,
	}
}

func migrationStatusResponse(status migrate.Status) MigrationStatusResponse {
	return MigrationStatusResponse{
		CurrentVersion: status.CurrentVersion,
		Applied:        migrationItems(status.Applied),
		Pending:        migrationItems(status.Pending),
	}
}

func migrationItems(items []migrate.Item) []MigrationItemDTO {
	out := make([]MigrationItemDTO, 0, len(items))
	for _, item := range items {
		out = append(out, MigrationItemDTO{
			Version: item.Version,
			Name:    item.Name,
			Kind:    string(item.Kind),
		})
	}
	return out
}

func formatTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.UTC().Format(time.RFC3339)
}

func dashboardAuthReady(cfg config.Config) bool {
	return cfg.ControlAPIEnabled() &&
		cfg.Dashboard.ClientID != "" &&
		cfg.Dashboard.ClientSecret != "" &&
		len(cfg.Dashboard.SessionSecret) >= 32
}

func signingReady(cfg config.Config) bool {
	return cfg.Dashboard.SigningKeyID != "" && cfg.Dashboard.SigningKeyFile != ""
}
