package accountspg

import (
	"database/sql"
	"time"

	accountstore "github.com/xsyetopz/go-mamacord/internal/storage/accounts"
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

func (s Store) AdminSessions() accountstore.AdminSessionStore {
	return adminSessionStore{db: s.db, now: s.now}
}

func (s Store) Users() accountstore.UserStore {
	return userStore{db: s.db, now: s.now}
}

func (s Store) Guilds() accountstore.GuildStore {
	return guildStore{db: s.db, now: s.now}
}

func (s Store) GuildMembers() accountstore.GuildMemberStore {
	return guildMemberStore{db: s.db, now: s.now}
}

func (s Store) UserSettings() accountstore.UserSettingsStore {
	return userSettingsStore{db: s.db, now: s.now}
}

func (s Store) DiscordOAuthTokens() accountstore.DiscordOAuthTokenStore {
	return discordOAuthTokenStore{db: s.db, now: s.now}
}
