package automationpg

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

func (s Store) Reminders() storage.ReminderStore {
	return reminderStore{db: s.db, now: s.now}
}

func (s Store) CheckIns() storage.CheckInStore {
	return checkInStore{db: s.db, now: s.now}
}
