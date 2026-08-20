package moderationpg

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

func (s Store) Restrictions() storage.RestrictionStore {
	return restrictionStore{db: s.db, now: s.now}
}

func (s Store) Warnings() storage.WarningStore {
	return warningStore{db: s.db, now: s.now}
}

func (s Store) Audit() storage.AuditStore {
	return auditStore{db: s.db, now: s.now}
}
