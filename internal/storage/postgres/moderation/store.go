package moderationpg

import (
	"database/sql"
	"time"

	moderationstore "github.com/xsyetopz/go-mamacord/internal/storage/moderation"
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

func (s Store) Restrictions() moderationstore.RestrictionStore {
	return restrictionStore{db: s.db, now: s.now}
}

func (s Store) Warnings() moderationstore.WarningStore {
	return warningStore{db: s.db, now: s.now}
}

func (s Store) Audit() moderationstore.AuditStore {
	return auditStore{db: s.db, now: s.now}
}
