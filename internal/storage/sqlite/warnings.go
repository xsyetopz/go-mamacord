package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type warningStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s warningStore) CountWarnings(ctx context.Context, guildID, userID uint64) (int, error) {
	const query = `SELECT COUNT(1) FROM warnings WHERE guild_id = ? AND user_id = ?`

	guildIDDB, err := toInt64(guildID, "guild_id")
	if err != nil {
		return 0, err
	}
	userIDDB, err := toInt64(userID, "user_id")
	if err != nil {
		return 0, err
	}

	var count int
	err = s.db.QueryRowContext(ctx, query, guildIDDB, userIDDB).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count warnings: %w", err)
	}
	return count, nil
}

func (s warningStore) ListWarnings(ctx context.Context, guildID, userID uint64, limit int) ([]store.Warning, error) {
	if limit <= 0 {
		limit = 25
	}

	const query = `
SELECT id, guild_id, user_id, moderator_id, reason, created_at
FROM warnings
WHERE guild_id = ? AND user_id = ?
ORDER BY created_at DESC
LIMIT ?`

	guildIDDB, err := toInt64(guildID, "guild_id")
	if err != nil {
		return nil, err
	}
	userIDDB, err := toInt64(userID, "user_id")
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, query, guildIDDB, userIDDB, limit)
	if err != nil {
		return nil, fmt.Errorf("list warnings: %w", err)
	}
	defer rows.Close()

	var out []store.Warning
	for rows.Next() {
		var w store.Warning
		var guildIDDBRow int64
		var userIDDBRow int64
		var modIDDBRow int64
		var createdAt int64

		if scanErr := rows.Scan(
			&w.ID,
			&guildIDDBRow,
			&userIDDBRow,
			&modIDDBRow,
			&w.Reason,
			&createdAt,
		); scanErr != nil {
			return nil, fmt.Errorf("scan warning: %w", scanErr)
		}

		guildIDU64, convErr := toUint64(guildIDDBRow, "guild_id")
		if convErr != nil {
			return nil, convErr
		}
		userIDU64, convErr := toUint64(userIDDBRow, "user_id")
		if convErr != nil {
			return nil, convErr
		}
		moderatorIDU64, convErr := toUint64(modIDDBRow, "moderator_id")
		if convErr != nil {
			return nil, convErr
		}

		w.GuildID = guildIDU64
		w.UserID = userIDU64
		w.ModeratorID = moderatorIDU64
		w.CreatedAt = time.Unix(createdAt, 0).UTC()
		out = append(out, w)
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("iterate warnings: %w", rowsErr)
	}

	return out, nil
}

func (s warningStore) CreateWarning(ctx context.Context, w store.Warning) error {
	const query = `
INSERT INTO warnings(id, guild_id, user_id, moderator_id, reason, created_at)
VALUES (?, ?, ?, ?, ?, ?)`

	createdAt := w.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.now()
	}

	guildIDDB, err := toInt64(w.GuildID, "guild_id")
	if err != nil {
		return err
	}
	userIDDB, err := toInt64(w.UserID, "user_id")
	if err != nil {
		return err
	}
	moderatorIDDB, err := toInt64(w.ModeratorID, "moderator_id")
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(
		ctx,
		query,
		w.ID,
		guildIDDB,
		userIDDB,
		moderatorIDDB,
		w.Reason,
		createdAt.Unix(),
	)
	if err != nil {
		return fmt.Errorf("create warning: %w", err)
	}
	return nil
}

func (s warningStore) DeleteWarning(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, "DELETE FROM warnings WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete warning: %w", err)
	}
	return nil
}
