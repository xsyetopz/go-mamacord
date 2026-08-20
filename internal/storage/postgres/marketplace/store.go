package marketplacepg

import (
	"database/sql"
	"time"

	marketstore "github.com/xsyetopz/go-mamacord/internal/storage/marketplace"
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

func (s Store) TrustedSigners() marketstore.TrustedSignerStore {
	return signerStore{db: s.db, now: s.now}
}

func (s Store) Sources() marketstore.MarketplaceSourceStore {
	return marketplaceSourceStore{db: s.db, now: s.now}
}

func (s Store) SourceSyncs() marketstore.MarketplaceSourceSyncStore {
	return marketplaceSourceSyncStore{db: s.db, now: s.now}
}

func (s Store) PluginInstalls() marketstore.PluginInstallStore {
	return pluginInstallStore{db: s.db, now: s.now}
}

func (s Store) TrustedVendors() marketstore.TrustedVendorStore {
	return trustedVendorStore{db: s.db, now: s.now}
}

func (s Store) TrustedVendorKeys() marketstore.TrustedVendorKeyStore {
	return trustedVendorKeyStore{db: s.db}
}
