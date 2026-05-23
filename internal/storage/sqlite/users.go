package sqlitestore

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
	now := s.now()
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
VALUES (?, ?, ?, ?, ?, ?)
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

	_, err = s.db.ExecContext(
		ctx,
		query,
		userIDDB,
		createdAt.Unix(),
		boolToInt(u.IsBot),
		boolToInt(u.IsSystem),
		firstSeenAt.Unix(),
		lastSeenAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("upsert user: %w", err)
	}
	return nil
}

func (s userStore) TouchUserSeen(ctx context.Context, userID uint64, seenAt time.Time) error {
	if seenAt.IsZero() {
		seenAt = s.now()
	}
	const query = `
UPDATE users
SET last_seen_at = CASE
  WHEN ? > last_seen_at THEN ?
  ELSE last_seen_at
END
WHERE user_id = ?`

	userIDDB, err := toInt64(userID, "user_id")
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, query, seenAt.Unix(), seenAt.Unix(), userIDDB)
	if err != nil {
		return fmt.Errorf("touch user: %w", err)
	}
	return nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
