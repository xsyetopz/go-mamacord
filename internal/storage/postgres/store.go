package postgresstore

import (
	"database/sql"
	"errors"
	"time"

	accountstore "github.com/xsyetopz/go-mamacord/internal/storage/accounts"
	automationstore "github.com/xsyetopz/go-mamacord/internal/storage/automation"
	marketstore "github.com/xsyetopz/go-mamacord/internal/storage/marketplace"
	moderationstore "github.com/xsyetopz/go-mamacord/internal/storage/moderation"
	pluginstore "github.com/xsyetopz/go-mamacord/internal/storage/plugins"
	accountspg "github.com/xsyetopz/go-mamacord/internal/storage/postgres/accounts"
	automationpg "github.com/xsyetopz/go-mamacord/internal/storage/postgres/automation"
	marketplacepg "github.com/xsyetopz/go-mamacord/internal/storage/postgres/marketplace"
	moderationpg "github.com/xsyetopz/go-mamacord/internal/storage/postgres/moderation"
	pluginspg "github.com/xsyetopz/go-mamacord/internal/storage/postgres/plugins"
)

type Store struct {
	db          *sql.DB
	accounts    accountspg.Store
	automation  automationpg.Store
	marketplace marketplacepg.Store
	moderation  moderationpg.Store
	plugins     pluginspg.Store
}

func New(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, errors.New("db is required")
	}
	now := time.Now
	return &Store{
		db:          db,
		accounts:    accountspg.New(db, now),
		automation:  automationpg.New(db, now),
		marketplace: marketplacepg.New(db, now),
		moderation:  moderationpg.New(db, now),
		plugins:     pluginspg.New(db, now),
	}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Restrictions() moderationstore.RestrictionStore {
	return s.moderation.Restrictions()
}

func (s *Store) Warnings() moderationstore.WarningStore {
	return s.moderation.Warnings()
}

func (s *Store) Audit() moderationstore.AuditStore {
	return s.moderation.Audit()
}

func (s *Store) PluginKV() pluginstore.PluginKVStore {
	return s.plugins.PluginKV()
}

func (s *Store) AdminSessions() accountstore.AdminSessionStore {
	return s.accounts.AdminSessions()
}

func (s *Store) ModuleStates() pluginstore.ModuleStateStore {
	return s.plugins.ModuleStates()
}

func (s *Store) Users() accountstore.UserStore {
	return s.accounts.Users()
}

func (s *Store) Guilds() accountstore.GuildStore {
	return s.accounts.Guilds()
}

func (s *Store) GuildMembers() accountstore.GuildMemberStore {
	return s.accounts.GuildMembers()
}

func (s *Store) UserSettings() accountstore.UserSettingsStore {
	return s.accounts.UserSettings()
}

func (s *Store) Reminders() automationstore.ReminderStore {
	return s.automation.Reminders()
}

func (s *Store) CheckIns() automationstore.CheckInStore {
	return s.automation.CheckIns()
}

func (s *Store) DiscordOAuthTokens() accountstore.DiscordOAuthTokenStore {
	return s.accounts.DiscordOAuthTokens()
}

func (s *Store) PluginOAuthGrants() pluginstore.PluginOAuthGrantStore {
	return s.plugins.PluginOAuthGrants()
}

func (s *Store) TrustedSigners() marketstore.TrustedSignerStore {
	return s.marketplace.TrustedSigners()
}

func (s *Store) MarketplaceSources() marketstore.MarketplaceSourceStore {
	return s.marketplace.Sources()
}

func (s *Store) MarketplaceSourceSyncs() marketstore.MarketplaceSourceSyncStore {
	return s.marketplace.SourceSyncs()
}

func (s *Store) PluginInstalls() marketstore.PluginInstallStore {
	return s.marketplace.PluginInstalls()
}

func (s *Store) TrustedVendors() marketstore.TrustedVendorStore {
	return s.marketplace.TrustedVendors()
}

func (s *Store) TrustedVendorKeys() marketstore.TrustedVendorKeyStore {
	return s.marketplace.TrustedVendorKeys()
}
