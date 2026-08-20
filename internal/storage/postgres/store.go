package postgresstore

import (
	"database/sql"
	"errors"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
	accountspg "github.com/xsyetopz/go-mamacord/internal/storage/postgres/accounts"
	automationpg "github.com/xsyetopz/go-mamacord/internal/storage/postgres/automation"
	marketplacepg "github.com/xsyetopz/go-mamacord/internal/storage/postgres/marketplace"
	moderationpg "github.com/xsyetopz/go-mamacord/internal/storage/postgres/moderation"
	pluginspg "github.com/xsyetopz/go-mamacord/internal/storage/postgres/plugins"
	"time"
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

func (s *Store) Restrictions() storage.RestrictionStore {
	return s.moderation.Restrictions()
}

func (s *Store) Warnings() storage.WarningStore {
	return s.moderation.Warnings()
}

func (s *Store) Audit() storage.AuditStore {
	return s.moderation.Audit()
}

func (s *Store) PluginKV() storage.PluginKVStore {
	return s.plugins.PluginKV()
}

func (s *Store) AdminSessions() storage.AdminSessionStore {
	return s.accounts.AdminSessions()
}

func (s *Store) ModuleStates() storage.ModuleStateStore {
	return s.plugins.ModuleStates()
}

func (s *Store) Users() storage.UserStore {
	return s.accounts.Users()
}

func (s *Store) Guilds() storage.GuildStore {
	return s.accounts.Guilds()
}

func (s *Store) GuildMembers() storage.GuildMemberStore {
	return s.accounts.GuildMembers()
}

func (s *Store) UserSettings() storage.UserSettingsStore {
	return s.accounts.UserSettings()
}

func (s *Store) Reminders() storage.ReminderStore {
	return s.automation.Reminders()
}

func (s *Store) CheckIns() storage.CheckInStore {
	return s.automation.CheckIns()
}

func (s *Store) DiscordOAuthTokens() storage.DiscordOAuthTokenStore {
	return s.accounts.DiscordOAuthTokens()
}

func (s *Store) PluginOAuthGrants() storage.PluginOAuthGrantStore {
	return s.plugins.PluginOAuthGrants()
}

func (s *Store) TrustedSigners() storage.TrustedSignerStore {
	return s.marketplace.TrustedSigners()
}

func (s *Store) MarketplaceSources() storage.MarketplaceSourceStore {
	return s.marketplace.Sources()
}

func (s *Store) MarketplaceSourceSyncs() storage.MarketplaceSourceSyncStore {
	return s.marketplace.SourceSyncs()
}

func (s *Store) PluginInstalls() storage.PluginInstallStore {
	return s.marketplace.PluginInstalls()
}

func (s *Store) TrustedVendors() storage.TrustedVendorStore {
	return s.marketplace.TrustedVendors()
}

func (s *Store) TrustedVendorKeys() storage.TrustedVendorKeyStore {
	return s.marketplace.TrustedVendorKeys()
}
