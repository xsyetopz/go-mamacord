package automationpg

import (
	"database/sql"
	"time"

	automationstore "github.com/xsyetopz/go-mamacord/internal/storage/automation"
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

func (s Store) Reminders() automationstore.ReminderStore {
	return reminderStore{db: s.db, now: s.now}
}

func (s Store) CheckIns() automationstore.CheckInStore {
	return checkInStore{db: s.db, now: s.now}
}
