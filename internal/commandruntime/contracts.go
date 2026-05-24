package commandruntime

import (
	"context"
	"log/slog"

	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type Store interface {
	Restrictions() store.RestrictionStore
	Warnings() store.WarningStore
	Audit() store.AuditStore
	TrustedSigners() store.TrustedSignerStore
	MarketplaceSources() store.MarketplaceSourceStore
	MarketplaceSourceSyncs() store.MarketplaceSourceSyncStore
	PluginInstalls() store.PluginInstallStore
	TrustedVendors() store.TrustedVendorStore
	TrustedVendorKeys() store.TrustedVendorKeyStore
	AdminSessions() store.AdminSessionStore
	PluginKV() store.PluginKVStore
	ModuleStates() store.ModuleStateStore
	Users() store.UserStore
	Guilds() store.GuildStore
	GuildMembers() store.GuildMemberStore
	UserSettings() store.UserSettingsStore
	Reminders() store.ReminderStore
	CheckIns() store.CheckInStore
}

type PluginAdmin interface {
	Configured() bool
	Infos() []pluginhost.PluginInfo
	Reload(ctx context.Context) error
}

type MarketplaceAdmin interface {
	Configured() bool
	ListSources(ctx context.Context) ([]marketplace.Source, error)
	UpsertSource(ctx context.Context, req marketplace.SourceUpsert) (marketplace.Source, error)
	DeleteSource(ctx context.Context, sourceID string) error
	SyncSource(ctx context.Context, sourceID string) (marketplace.SyncResult, error)
	Search(ctx context.Context, query marketplace.SearchQuery) ([]marketplace.PluginCandidate, error)
	Install(ctx context.Context, req marketplace.InstallRequest) (marketplace.InstallResult, error)
	Update(ctx context.Context, req marketplace.UpdateRequest) (marketplace.UpdateResult, error)
	Uninstall(ctx context.Context, req marketplace.UninstallRequest) error
	TrustSigner(ctx context.Context, req marketplace.TrustSignerRequest) error
	TrustVendor(ctx context.Context, req marketplace.TrustVendorRequest) (marketplace.TrustVendorResult, error)
}

type Services struct {
	Logger   *slog.Logger
	Store    Store
	ProdMode bool

	IsOwner func(userID uint64) bool

	Plugins     PluginAdmin
	Marketplace MarketplaceAdmin
	Modules     moduleapi.Admin

	// HelpNames returns the localized slash command names for help output.
	HelpNames func(locale string) []string
}
