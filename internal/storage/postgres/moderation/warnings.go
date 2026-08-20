package moderationpg

import (
	"context"
	"database/sql"
	"fmt"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
	"github.com/xsyetopz/go-mamacord/internal/storage/postgres/internal/sqlvalue"
	"time"
)

type warningStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s warningStore) CountWarnings(ctx context.Context, guildID, userID uint64) (int, error) {
	const query = `SELECT COUNT(1) FROM warnings WHERE guild_id = $1 AND user_id = $2`

	guildIDDB, err := sqlvalue.Uint64ToInt64(guildID, "guild_id")
	if err != nil {
		return 0, err
	}
	userIDDB, err := sqlvalue.Uint64ToInt64(userID, "user_id")
	if err != nil {
		return 0, err
	}

	var count int
	if err := s.db.QueryRowContext(ctx, query, guildIDDB, userIDDB).Scan(&count); err != nil {
		return 0, fmt.Errorf("count warnings: %w", err)
	}
	return count, nil
}

func (s warningStore) ListWarnings(ctx context.Context, guildID, userID uint64, limit int) ([]storage.Warning, error) {
	if limit <= 0 {
		limit = 25
	}

	const query = `
SELECT id, guild_id, user_id, moderator_id, reason, created_at
FROM warnings
WHERE guild_id = $1 AND user_id = $2
ORDER BY created_at DESC
LIMIT $3`

	guildIDDB, err := sqlvalue.Uint64ToInt64(guildID, "guild_id")
	if err != nil {
		return nil, err
	}
	userIDDB, err := sqlvalue.Uint64ToInt64(userID, "user_id")
	if err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, query, guildIDDB, userIDDB, limit)
	if err != nil {
		return nil, fmt.Errorf("list warnings: %w", err)
	}
	defer rows.Close()

	var out []storage.Warning
	for rows.Next() {
		var w storage.Warning
		var guildIDDBRow int64
		var userIDDBRow int64
		var moderatorIDDBRow int64
		var createdAt int64

		if err := rows.Scan(
			&w.ID,
			&guildIDDBRow,
			&userIDDBRow,
			&moderatorIDDBRow,
			&w.Reason,
			&createdAt,
		); err != nil {
			return nil, fmt.Errorf("scan warning: %w", err)
		}

		guildIDU64, err := sqlvalue.Int64ToUint64(guildIDDBRow, "guild_id")
		if err != nil {
			return nil, err
		}
		userIDU64, err := sqlvalue.Int64ToUint64(userIDDBRow, "user_id")
		if err != nil {
			return nil, err
		}
		moderatorIDU64, err := sqlvalue.Int64ToUint64(moderatorIDDBRow, "moderator_id")
		if err != nil {
			return nil, err
		}

		w.GuildID = guildIDU64
		w.UserID = userIDU64
		w.ModeratorID = moderatorIDU64
		w.CreatedAt = time.Unix(createdAt, 0).UTC()
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate warnings: %w", err)
	}
	return out, nil
}

func (s warningStore) CreateWarning(ctx context.Context, w storage.Warning) error {
	const query = `
INSERT INTO warnings(id, guild_id, user_id, moderator_id, reason, created_at)
VALUES ($1, $2, $3, $4, $5, $6)`

	createdAt := w.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.now().UTC()
	}

	guildIDDB, err := sqlvalue.Uint64ToInt64(w.GuildID, "guild_id")
	if err != nil {
		return err
	}
	userIDDB, err := sqlvalue.Uint64ToInt64(w.UserID, "user_id")
	if err != nil {
		return err
	}
	moderatorIDDB, err := sqlvalue.Uint64ToInt64(w.ModeratorID, "moderator_id")
	if err != nil {
		return err
	}

	if _, err := s.db.ExecContext(
		ctx,
		query,
		w.ID,
		guildIDDB,
		userIDDB,
		moderatorIDDB,
		w.Reason,
		createdAt.Unix(),
	); err != nil {
		return fmt.Errorf("create warning: %w", err)
	}
	return nil
}

func (s warningStore) DeleteWarning(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM warnings WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete warning: %w", err)
	}
	return nil
}
