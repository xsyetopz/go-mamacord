package adminapi

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
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
	pluginhostlua "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

var pluginIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{1,31}$`)

type Service struct {
	Logger  *slog.Logger
	Config  config.Config
	Bundles bundles.Repository

	Snapshot      func() ops.Snapshot
	ModuleAdmin   moduleapi.Admin
	PluginAdmin   commandruntime.PluginAdmin
	Marketplace   commandruntime.MarketplaceAdmin
	Store         commandruntime.Store
	BuildInfo     func() buildinfo.Info
	OAuth         OAuthClient
	OwnerStatus   func() OwnerStatus
	KnownGuildIDs func() []uint64
	BotHasGuild   func(ctx context.Context, guildID uint64) (bool, error)

	ListGuildChannels  func(ctx context.Context, guildID uint64) ([]GuildChannelInfo, error)
	ListGuildRoles     func(ctx context.Context, guildID uint64) ([]GuildRoleInfo, error)
	SearchGuildMembers func(ctx context.Context, guildID uint64, query string, limit int) ([]GuildMemberInfo, error)
	ListGuildEmojis    func(ctx context.Context, guildID uint64) ([]GuildEmojiInfo, error)
	ListGuildStickers  func(ctx context.Context, guildID uint64) ([]GuildStickerInfo, error)

	SetSlowmode         func(ctx context.Context, channelID uint64, seconds int) error
	SetNickname         func(ctx context.Context, guildID, userID uint64, nickname *string) error
	TimeoutMember       func(ctx context.Context, guildID, userID uint64, untilUnix int64) error
	CreateRole          func(ctx context.Context, spec pluginhostlua.RoleCreateSpec) (pluginhostlua.RoleResult, error)
	EditRole            func(ctx context.Context, spec pluginhostlua.RoleEditSpec) (pluginhostlua.RoleResult, error)
	DeleteRole          func(ctx context.Context, guildID, roleID uint64) error
	AddRole             func(ctx context.Context, spec pluginhostlua.RoleMemberSpec) error
	RemoveRole          func(ctx context.Context, spec pluginhostlua.RoleMemberSpec) error
	PurgeMessages       func(ctx context.Context, spec pluginhostlua.PurgeSpec) (int, error)
	CreateEmojiUpload   func(ctx context.Context, guildID uint64, name, filename string, body []byte, width, height int) (pluginhostlua.EmojiResult, error)
	EditEmoji           func(ctx context.Context, spec pluginhostlua.EmojiEditSpec) (pluginhostlua.EmojiResult, error)
	DeleteEmoji         func(ctx context.Context, spec pluginhostlua.EmojiDeleteSpec) error
	CreateStickerUpload func(ctx context.Context, guildID uint64, name, description, emojiTag, filename string, body []byte, width, height int) (pluginhostlua.StickerResult, error)
	EditSticker         func(ctx context.Context, spec pluginhostlua.StickerEditSpec) (pluginhostlua.StickerResult, error)
	DeleteSticker       func(ctx context.Context, spec pluginhostlua.StickerDeleteSpec) error

	guildsMu       sync.Mutex
	guildsCache    map[string]guildsCacheEntry
	guildsInflight map[string]*guildsInflight
}

type guildsCacheEntry struct {
	fetchedAt    time.Time
	expiresAt    time.Time
	blockedUntil time.Time
	retryAfter   time.Duration
	guilds       []OAuthGuild
}

type guildsInflight struct {
	done   chan struct{}
	guilds []OAuthGuild
	err    error
}

type OwnerStatus struct {
	Configured      bool
	Resolved        bool
	Source          string
	EffectiveUserID *uint64
}

func cloneOptionalUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func (s *Service) init() {
	if s == nil {
		return
	}
	s.guildsMu.Lock()
	defer s.guildsMu.Unlock()
	if s.guildsCache == nil {
		s.guildsCache = map[string]guildsCacheEntry{}
	}
	if s.guildsInflight == nil {
		s.guildsInflight = map[string]*guildsInflight{}
	}
}

func tokenCacheKey(accessToken string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(accessToken)))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

const (
	// guildsCacheTTL keeps Discord OAuth `/users/@me/guilds` calls reasonably low
	// while still feeling fresh in the UI.
	guildsCacheTTL = 30 * time.Second
	// guildsStaleWhileRateLimited is the maximum age of a cached guild list that
	// we will still serve when Discord is rate limiting this user.
	guildsStaleWhileRateLimited = 10 * time.Minute
)

func cloneGuilds(in []OAuthGuild) []OAuthGuild {
	if len(in) == 0 {
		return nil
	}
	out := make([]OAuthGuild, len(in))
	copy(out, in)
	return out
}

func (s *Service) fetchGuildsCached(ctx context.Context, accessToken string) ([]OAuthGuild, error) {
	if s == nil || s.OAuth == nil {
		return nil, errors.New("oauth client is not configured")
	}

	key := tokenCacheKey(accessToken)
	now := time.Now()

	s.guildsMu.Lock()
	if s.guildsCache == nil {
		s.guildsCache = map[string]guildsCacheEntry{}
	}
	if s.guildsInflight == nil {
		s.guildsInflight = map[string]*guildsInflight{}
	}

	if entry, ok := s.guildsCache[key]; ok {
		if now.Before(entry.blockedUntil) {
			// Prefer serving cached data (even if a bit stale) to avoid showing
			// raw Discord rate-limit errors in the dashboard.
			if len(entry.guilds) > 0 && now.Sub(entry.fetchedAt) <= guildsStaleWhileRateLimited {
				out := cloneGuilds(entry.guilds)
				s.guildsMu.Unlock()
				return out, nil
			}
			retry := time.Until(entry.blockedUntil)
			if retry < 0 {
				retry = 0
			}
			s.guildsMu.Unlock()
			return nil, &PublicError{
				Status:     http.StatusTooManyRequests,
				Message:    "Discord is rate limiting right now. Please try again in a moment.",
				RetryAfter: retry,
			}
		}

		if now.Before(entry.expiresAt) && len(entry.guilds) > 0 {
			out := cloneGuilds(entry.guilds)
			s.guildsMu.Unlock()
			return out, nil
		}
	}

	if inflight, ok := s.guildsInflight[key]; ok && inflight != nil {
		done := inflight.done
		s.guildsMu.Unlock()
		select {
		case <-done:
			return cloneGuilds(inflight.guilds), inflight.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	inflight := &guildsInflight{done: make(chan struct{})}
	s.guildsInflight[key] = inflight
	s.guildsMu.Unlock()

	guilds, err := s.OAuth.FetchGuilds(ctx, accessToken)

	s.guildsMu.Lock()
	delete(s.guildsInflight, key)

	entry := s.guildsCache[key]
	if err == nil {
		entry.guilds = cloneGuilds(guilds)
		entry.fetchedAt = now
		entry.expiresAt = now.Add(guildsCacheTTL)
		entry.blockedUntil = time.Time{}
		entry.retryAfter = 0
	} else if rl, ok := isOAuthRateLimit(err); ok {
		// Back off. Keep old data if we have it.
		entry.retryAfter = rl.RetryAfter
		entry.blockedUntil = now.Add(rl.RetryAfter)
		// If we have cached guilds, keep them around long enough to bridge the RL.
		if len(entry.guilds) > 0 {
			if entry.expiresAt.Before(now.Add(rl.RetryAfter)) {
				entry.expiresAt = now.Add(rl.RetryAfter)
			}
		}
	} else {
		// On non-RL errors, keep the previous cache but don't extend it.
	}
	s.guildsCache[key] = entry

	inflight.guilds = cloneGuilds(guilds)
	inflight.err = err
	close(inflight.done)
	s.guildsMu.Unlock()

	// If we got rate-limited but still have usable cached data, serve it.
	if err != nil {
		if _, ok := isOAuthRateLimit(err); ok {
			s.guildsMu.Lock()
			cached := s.guildsCache[key]
			s.guildsMu.Unlock()
			if len(cached.guilds) > 0 && now.Sub(cached.fetchedAt) <= guildsStaleWhileRateLimited {
				return cloneGuilds(cached.guilds), nil
			}
			return nil, &PublicError{
				Status:     http.StatusTooManyRequests,
				Message:    "Discord is rate limiting right now. Please try again in a moment.",
				RetryAfter: cached.blockedUntil.Sub(now),
			}
		}
	}

	return cloneGuilds(guilds), err
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
	Ready               bool   `json:"ready"`
	StartedAt           string `json:"started_at"`
	MigrationVersion    int    `json:"migration_version"`
	ProdMode            bool   `json:"prod_mode"`
	DiscordStartError   string `json:"discord_start_error,omitempty"`
	ModuleCount         int    `json:"module_count"`
	EnabledModuleCount  int    `json:"enabled_module_count"`
	PluginCount         int    `json:"plugin_count"`
	EnabledPluginCount  int    `json:"enabled_plugin_count"`
	BuiltinCommandCount int    `json:"builtin_command_count"`
	SlashCommandCount   int    `json:"slash_command_count"`
	UserCommandCount    int    `json:"user_command_count"`
	MessageCommandCount int    `json:"message_command_count"`
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
	StorageBackend          string     `json:"storage_backend"`
	StorageTarget           string     `json:"storage_target"`
	MigrationsDir           string     `json:"migrations_dir"`
	LocalesDir              string     `json:"locales_dir"`
	BundledPluginsDir       string     `json:"bundled_plugins_dir"`
	UserPluginsDir          string     `json:"user_plugins_dir"`
	PermissionsFile         string     `json:"permissions_file"`
	ModulesFile             string     `json:"modules_file"`
	TrustedKeysFile         string     `json:"trusted_keys_file"`
	OpsAddr                 string     `json:"ops_addr"`
	AdminAddr               string     `json:"admin_addr"`
	RuntimeRoles            []string   `json:"runtime_roles"`
	DevGuildID              *Snowflake `json:"dev_guild_id,omitempty"`
	CommandRegistrationMode string     `json:"command_registration_mode"`
	ProdMode                bool       `json:"prod_mode"`
	AllowUnsignedPlugins    bool       `json:"allow_unsigned_plugins"`
}

type SetupResponse struct {
	AdminEnabled          bool       `json:"admin_enabled"`
	AuthConfigured        bool       `json:"auth_configured"`
	LoginReady            bool       `json:"login_ready"`
	OwnerConfigured       bool       `json:"owner_configured"`
	OwnerResolved         bool       `json:"owner_resolved"`
	OwnerSource           string     `json:"owner_source"`
	EffectiveOwnerUserID  *Snowflake `json:"effective_owner_user_id,omitempty"`
	SigningConfigured     bool       `json:"signing_configured"`
	TrustedKeysConfigured bool       `json:"trusted_keys_configured"`
	AdminAddr             string     `json:"admin_addr"`
	AppOrigin             string     `json:"app_origin"`
	RedirectURL           string     `json:"redirect_url"`
	InstallRedirectURL    string     `json:"install_redirect_url"`
	HasClientID           bool       `json:"has_client_id"`
	HasClientSecret       bool       `json:"has_client_secret"`
	HasSessionSecret      bool       `json:"has_session_secret"`
	Hints                 []string   `json:"hints"`
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
	ID                string   `json:"id"`
	Name              string   `json:"name"`
	Version           string   `json:"version"`
	Commands          []string `json:"commands"`
	Loaded            bool     `json:"loaded"`
	Signed            bool     `json:"signed"`
	HasSignatureFile  bool     `json:"has_signature_file"`
	PluginRoot        string   `json:"plugin_root"`
	Bundled           bool     `json:"bundled"`
	ProvenanceKind    string   `json:"provenance_kind"`
	SourceID          string   `json:"source_id,omitempty"`
	GitRevision       string   `json:"git_revision,omitempty"`
	BundleRelativeDir string   `json:"bundle_relative_dir,omitempty"`
	SignatureState    string   `json:"signature_state,omitempty"`
	LocalModified     bool     `json:"local_modified"`
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

type UserGuildSummary struct {
	ID           Snowflake `json:"id"`
	Name         string    `json:"name"`
	IconURL      string    `json:"icon_url,omitempty"`
	Owner        bool      `json:"owner"`
	CanManage    bool      `json:"can_manage"`
	BotInstalled bool      `json:"bot_installed"`
}

type GuildDashboardResponse struct {
	Guild       UserGuildSummary  `json:"guild"`
	InstallURL  string            `json:"install_url"`
	SetupChecks []SetupCheck      `json:"setup_checks"`
	Manager     ManagerSection    `json:"manager"`
	Moderation  ModerationSection `json:"moderation"`
	Fun         PluginSection     `json:"fun"`
	Info        PluginSection     `json:"info"`
	Wellness    WellnessSection   `json:"wellness"`
}

type SetupCheck struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

type PluginCommandState struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	Enabled bool   `json:"enabled"`
}

type PluginSection struct {
	ID            string               `json:"id"`
	Name          string               `json:"name"`
	Enabled       bool                 `json:"enabled"`
	GlobalEnabled bool                 `json:"global_enabled"`
	Commands      []PluginCommandState `json:"commands"`
}

type ManagerSection struct {
	PluginSection
	ChannelCount int `json:"channel_count"`
	RoleCount    int `json:"role_count"`
	EmojiCount   int `json:"emoji_count"`
	StickerCount int `json:"sticker_count"`
}

type ModerationSection struct {
	PluginSection
	WarningLimit     int `json:"warning_limit"`
	TimeoutThreshold int `json:"timeout_threshold"`
	TimeoutMinutes   int `json:"timeout_minutes"`
}

type WellnessSection struct {
	PluginSection
	AllowChannelReminders    bool      `json:"allow_channel_reminders"`
	DefaultReminderChannelID Snowflake `json:"default_reminder_channel_id,omitempty"`
}

type GuildChannelInfo struct {
	ID       Snowflake `json:"id"`
	Name     string    `json:"name"`
	Type     string    `json:"type"`
	ParentID Snowflake `json:"parent_id,omitempty"`
}

type GuildRoleInfo struct {
	ID          Snowflake `json:"id"`
	Name        string    `json:"name"`
	Color       int       `json:"color"`
	Position    int       `json:"position"`
	Managed     bool      `json:"managed"`
	Mentionable bool      `json:"mentionable"`
}

type GuildMemberInfo struct {
	UserID      Snowflake   `json:"user_id"`
	Username    string      `json:"username"`
	DisplayName string      `json:"display_name"`
	AvatarURL   string      `json:"avatar_url,omitempty"`
	Bot         bool        `json:"bot"`
	JoinedAt    int64       `json:"joined_at,omitempty"`
	RoleIDs     []Snowflake `json:"role_ids"`
}

type GuildEmojiInfo struct {
	ID       Snowflake `json:"id"`
	Name     string    `json:"name"`
	Animated bool      `json:"animated"`
}

type GuildStickerInfo struct {
	ID          Snowflake `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Tags        string    `json:"tags,omitempty"`
}

type WarningInfo struct {
	ID          string    `json:"id"`
	UserID      Snowflake `json:"user_id"`
	ModeratorID Snowflake `json:"moderator_id"`
	Reason      string    `json:"reason"`
	CreatedAt   string    `json:"created_at"`
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
	return strings.TrimSpace(s.Config.UserPluginsDir)
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
		strings.TrimSpace(s.Config.UserPluginsDir),
		strings.TrimSpace(s.Config.BundledPluginsDir),
	} {
		if root == "" {
			continue
		}
		dir := filepath.Join(root, pluginID)
		if fileExists(filepath.Join(dir, bundles.StateFileName)) {
			return dir, nil
		}
	}
	return filepath.Join(strings.TrimSpace(s.Config.UserPluginsDir), pluginID), nil
}

func (s *Service) setupResponse(includeHints bool) SetupResponse {
	ownerStatus := OwnerStatus{
		Configured: s.Config.OwnerUserID != nil,
		Resolved:   s.Config.OwnerUserID != nil,
		Source:     "unresolved",
	}
	if s.Config.OwnerUserID != nil {
		ownerStatus.Source = "config_fallback"
		ownerStatus.EffectiveUserID = s.Config.OwnerUserID
	}
	if s.OwnerStatus != nil {
		ownerStatus = s.OwnerStatus()
	}

	resp := SetupResponse{
		AdminEnabled:         s.Config.ControlAPIEnabled(),
		AuthConfigured:       dashboardAuthReady(s.Config),
		LoginReady:           dashboardAuthReady(s.Config),
		OwnerConfigured:      ownerStatus.Configured,
		OwnerResolved:        ownerStatus.Resolved,
		OwnerSource:          strings.TrimSpace(ownerStatus.Source),
		EffectiveOwnerUserID: optionalSnowflake(ownerStatus.EffectiveUserID),
		SigningConfigured:    signingReady(s.Config),
		AdminAddr:            strings.TrimSpace(s.Config.AdminAddr),
		// Filled by the HTTP layer based on configured public origins.
		AppOrigin:        "",
		RedirectURL:      "",
		HasClientID:      strings.TrimSpace(s.Config.DashboardClientID) != "",
		HasClientSecret:  strings.TrimSpace(s.Config.DashboardClientSecret) != "",
		HasSessionSecret: len(strings.TrimSpace(s.Config.DashboardSessionSecret)) >= 32,
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
		Ready:               snap.Ready,
		StartedAt:           formatTime(snap.StartedAt),
		MigrationVersion:    snap.MigrationVersion,
		ProdMode:            snap.ProdMode,
		DiscordStartError:   strings.TrimSpace(snap.DiscordStartError),
		ModuleCount:         snap.ModuleCount,
		EnabledModuleCount:  snap.EnabledModuleCount,
		PluginCount:         snap.PluginCount,
		EnabledPluginCount:  snap.EnabledPluginCount,
		BuiltinCommandCount: snap.BuiltinCommandCount,
		SlashCommandCount:   snap.SlashCommandCount,
		UserCommandCount:    snap.UserCommandCount,
		MessageCommandCount: snap.MessageCommandCount,
		InteractionsTotal:   snap.InteractionsTotal,
		InteractionFailures: snap.InteractionFailures,
		PluginFailures:      snap.PluginFailures,
		AutomationFailures:  snap.AutomationFailures,
		ReminderFailures:    snap.ReminderFailures,
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

func toUint64Set(fn func() []uint64) map[uint64]bool {
	out := map[uint64]bool{}
	if fn == nil {
		return out
	}
	for _, id := range fn() {
		out[id] = true
	}
	return out
}

func parseDiscordID(raw string) (uint64, error) {
	return strconv.ParseUint(strings.TrimSpace(raw), 10, 64)
}

func hasManageGuildPermissions(raw string) bool {
	value := strings.TrimSpace(raw)
	if value == "" {
		return false
	}
	perm, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return false
	}
	administrator := big.NewInt(0x8)
	manageGuild := big.NewInt(0x20)
	return new(big.Int).And(perm, administrator).Cmp(big.NewInt(0)) != 0 ||
		new(big.Int).And(perm, manageGuild).Cmp(big.NewInt(0)) != 0
}

func guildIconURL(guild OAuthGuild) string {
	id := strings.TrimSpace(guild.ID)
	icon := strings.TrimSpace(guild.Icon)
	if id == "" || icon == "" {
		return ""
	}
	return "https://cdn.discordapp.com/icons/" + id + "/" + icon + ".png"
}

func boolMessage(value bool, okMessage, noMessage string) string {
	if value {
		return okMessage
	}
	return noMessage
}

func dashboardAuthReady(cfg config.Config) bool {
	return cfg.ControlAPIEnabled() &&
		cfg.DashboardClientID != "" &&
		cfg.DashboardClientSecret != "" &&
		len(cfg.DashboardSessionSecret) >= 32
}

func signingReady(cfg config.Config) bool {
	return cfg.DashboardSigningKeyID != "" && cfg.DashboardSigningKeyFile != ""
}
