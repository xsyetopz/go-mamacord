package accountspg

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

func (s Store) AdminSessions() storage.AdminSessionStore {
	return adminSessionStore{db: s.db, now: s.now}
}

func (s Store) Users() storage.UserStore {
	return userStore{db: s.db, now: s.now}
}

func (s Store) Guilds() storage.GuildStore {
	return guildStore{db: s.db, now: s.now}
}

func (s Store) GuildMembers() storage.GuildMemberStore {
	return guildMemberStore{db: s.db, now: s.now}
}

func (s Store) UserSettings() storage.UserSettingsStore {
	return userSettingsStore{db: s.db, now: s.now}
}

func (s Store) DiscordOAuthTokens() storage.DiscordOAuthTokenStore {
	return discordOAuthTokenStore{db: s.db, now: s.now}
}
