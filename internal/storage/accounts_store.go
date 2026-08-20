package storage

import (
	"context"
	"time"
)

type AdminSession struct {
	ID          string
	UserID      uint64
	Username    string
	Name        string
	AvatarURL   string
	CSRFToken   string
	AccessToken string
	IsOwner     bool
	ExpiresAt   int64
}

type AdminSessionStore interface {
	GetAdminSession(ctx context.Context, id string) (AdminSession, bool, error)
	PutAdminSession(ctx context.Context, session AdminSession) error
	DeleteAdminSession(ctx context.Context, id string) error
	DeleteExpiredAdminSessions(ctx context.Context, nowUnix int64) (int64, error)
}

type UserSeen struct {
	UserID      uint64
	CreatedAt   time.Time
	IsBot       bool
	IsSystem    bool
	FirstSeenAt time.Time
	LastSeenAt  time.Time
}

type UserStore interface {
	UpsertUserSeen(ctx context.Context, user UserSeen) error
	TouchUserSeen(ctx context.Context, userID uint64, seenAt time.Time) error
}

type GuildSeen struct {
	GuildID   uint64
	OwnerID   uint64
	CreatedAt time.Time
	JoinedAt  time.Time
	LeftAt    *time.Time
	Name      string
	UpdatedAt time.Time
}

type GuildStore interface {
	UpsertGuildSeen(ctx context.Context, guild GuildSeen) error
	MarkGuildLeft(ctx context.Context, guildID uint64, leftAt time.Time) error
	UpdateGuildOwner(ctx context.Context, guildID uint64, ownerID uint64, updatedAt time.Time) error
}

type GuildMemberStore interface {
	MarkMemberJoined(ctx context.Context, guildID, userID uint64, joinedAt time.Time) error
	MarkMemberLeft(ctx context.Context, guildID, userID uint64, leftAt time.Time) error
}

type UserSettings struct {
	UserID      uint64
	Timezone    string
	DMChannelID *uint64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type UserSettingsStore interface {
	GetUserSettings(ctx context.Context, userID uint64) (UserSettings, bool, error)
	UpsertUserTimezone(ctx context.Context, userID uint64, timezone string) error
	ClearUserTimezone(ctx context.Context, userID uint64) error
	UpsertUserDMChannelID(ctx context.Context, userID uint64, dmChannelID uint64) error
}

type DiscordOAuthToken struct {
	UserID          uint64
	AccessTokenEnc  string
	RefreshTokenEnc string
	Scope           string
	ExpiresAt       time.Time
	UpdatedAt       time.Time
}

type DiscordOAuthTokenStore interface {
	GetDiscordOAuthToken(ctx context.Context, userID uint64) (DiscordOAuthToken, bool, error)
	PutDiscordOAuthToken(ctx context.Context, token DiscordOAuthToken) error
	DeleteDiscordOAuthToken(ctx context.Context, userID uint64) error
}
