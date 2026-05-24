package postgresstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type userStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s userStore) UpsertUserSeen(ctx context.Context, u store.UserSeen) error {
	now := s.now().UTC()
	createdAt := u.CreatedAt
	if createdAt.IsZero() {
		createdAt = now
	}
	firstSeenAt := u.FirstSeenAt
	if firstSeenAt.IsZero() {
		firstSeenAt = now
	}
	lastSeenAt := u.LastSeenAt
	if lastSeenAt.IsZero() {
		lastSeenAt = firstSeenAt
	}

	const query = `
INSERT INTO users(user_id, created_at, is_bot, is_system, first_seen_at, last_seen_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT(user_id) DO UPDATE SET
	is_bot = excluded.is_bot,
	is_system = excluded.is_system,
	first_seen_at = CASE
		WHEN excluded.first_seen_at < users.first_seen_at THEN excluded.first_seen_at
		ELSE users.first_seen_at
	END,
	last_seen_at = CASE
		WHEN excluded.last_seen_at > users.last_seen_at THEN excluded.last_seen_at
		ELSE users.last_seen_at
	END`

	userIDDB, err := toInt64(u.UserID, "user_id")
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(
		ctx,
		query,
		userIDDB,
		createdAt.Unix(),
		u.IsBot,
		u.IsSystem,
		firstSeenAt.Unix(),
		lastSeenAt.Unix(),
	); err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}

func (s userStore) TouchUserSeen(ctx context.Context, userID uint64, seenAt time.Time) error {
	if seenAt.IsZero() {
		seenAt = s.now().UTC()
	}
	const query = `
UPDATE users
SET last_seen_at = CASE
	WHEN $1 > last_seen_at THEN $1
	ELSE last_seen_at
END
WHERE user_id = $2`

	userIDDB, err := toInt64(userID, "user_id")
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, query, seenAt.Unix(), userIDDB); err != nil {
		return fmt.Errorf("touch user: %w", err)
	}
	return nil
}
