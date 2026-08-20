package pluginspg

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

func (s Store) PluginKV() storage.PluginKVStore {
	return pluginKVStore{db: s.db, now: s.now}
}

func (s Store) ModuleStates() storage.ModuleStateStore {
	return moduleStateStore{db: s.db, now: s.now}
}

func (s Store) PluginOAuthGrants() storage.PluginOAuthGrantStore {
	return pluginOAuthGrantStore{db: s.db, now: s.now}
}
