package pluginspg

import (
	"database/sql"
	"time"

	pluginstore "github.com/xsyetopz/go-mamacord/internal/storage/plugins"
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

func (s Store) PluginKV() pluginstore.PluginKVStore {
	return pluginKVStore{db: s.db, now: s.now}
}

func (s Store) ModuleStates() pluginstore.ModuleStateStore {
	return moduleStateStore{db: s.db, now: s.now}
}

func (s Store) PluginOAuthGrants() pluginstore.PluginOAuthGrantStore {
	return pluginOAuthGrantStore{db: s.db, now: s.now}
}
