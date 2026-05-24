package postgresstore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type guildMemberStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s guildMemberStore) MarkMemberJoined(ctx context.Context, guildID, userID uint64, joinedAt time.Time) error {
	if joinedAt.IsZero() {
		joinedAt = s.now().UTC()
	}

	const query = `
INSERT INTO guild_members(guild_id, user_id, joined_at, left_at)
VALUES ($1, $2, $3, NULL)
ON CONFLICT(guild_id, user_id) DO UPDATE SET
	joined_at = excluded.joined_at,
	left_at = NULL`

	guildIDDB, err := toInt64(guildID, "guild_id")
	if err != nil {
		return err
	}
	userIDDB, err := toInt64(userID, "user_id")
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, query, guildIDDB, userIDDB, joinedAt.Unix()); err != nil {
		return fmt.Errorf("mark member joined: %w", err)
	}
	return nil
}

func (s guildMemberStore) MarkMemberLeft(ctx context.Context, guildID, userID uint64, leftAt time.Time) error {
	if leftAt.IsZero() {
		leftAt = s.now().UTC()
	}

	const query = `UPDATE guild_members SET left_at = $1 WHERE guild_id = $2 AND user_id = $3`

	guildIDDB, err := toInt64(guildID, "guild_id")
	if err != nil {
		return err
	}
	userIDDB, err := toInt64(userID, "user_id")
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, query, leftAt.Unix(), guildIDDB, userIDDB); err != nil {
		return fmt.Errorf("mark member left: %w", err)
	}
	return nil
}
