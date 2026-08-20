package marketplacepg

import (
	"database/sql"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
	"time"
)

type Store struct {
	db  *sql.DB
	now func() time.Time
}

func New(db *sql.DB, now func() time.Time) Store {
	if now == nil {
		now = time.Now
	}
	return Store{db: db, now: now}
}

func (s Store) TrustedSigners() storage.TrustedSignerStore {
	return signerStore{db: s.db, now: s.now}
}

func (s Store) Sources() storage.MarketplaceSourceStore {
	return marketplaceSourceStore{db: s.db, now: s.now}
}

func (s Store) SourceSyncs() storage.MarketplaceSourceSyncStore {
	return marketplaceSourceSyncStore{db: s.db, now: s.now}
}

func (s Store) PluginInstalls() storage.PluginInstallStore {
	return pluginInstallStore{db: s.db, now: s.now}
}

func (s Store) TrustedVendors() storage.TrustedVendorStore {
	return trustedVendorStore{db: s.db, now: s.now}
}

func (s Store) TrustedVendorKeys() storage.TrustedVendorKeyStore {
	return trustedVendorKeyStore{db: s.db}
}
