package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type checkInStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s checkInStore) CreateCheckIn(ctx context.Context, c store.CheckIn) error {
	if s.db == nil {
		return errors.New("db not configured")
	}
	if strings.TrimSpace(c.ID) == "" {
		return errors.New("checkin id is required")
	}
	if c.UserID == 0 {
		return errors.New("checkin user_id is required")
	}
	if c.Mood < 1 || c.Mood > 5 {
		return errors.New("checkin mood must be 1..5")
	}

	userID64, err := toInt64(c.UserID, "user_id")
	if err != nil {
		return err
	}

	createdAt := c.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.nowTime().UTC()
	}

	const query = `
INSERT INTO checkins(id, user_id, mood, created_at)
VALUES (?, ?, ?, ?)`
	if _, execErr := s.db.ExecContext(
		ctx,
		query,
		strings.TrimSpace(c.ID),
		userID64,
		c.Mood,
		createdAt.UTC().Unix(),
	); execErr != nil {
		return fmt.Errorf("create checkin: %w", execErr)
	}
	return nil
}

func (s checkInStore) ListCheckIns(ctx context.Context, userID uint64, limit int) ([]store.CheckIn, error) {
	if s.db == nil {
		return nil, errors.New("db not configured")
	}
	if limit <= 0 {
		limit = 25
	}

	userID64, err := toInt64(userID, "user_id")
	if err != nil {
		return nil, err
	}

	const query = `
SELECT id, mood, created_at
FROM checkins
WHERE user_id = ?
ORDER BY created_at DESC
LIMIT ?`

	rows, err := s.db.QueryContext(ctx, query, userID64, limit)
	if err != nil {
		return nil, fmt.Errorf("list checkins: %w", err)
	}
	defer rows.Close()

	out := []store.CheckIn{}
	for rows.Next() {
		var id string
		var mood int
		var createdAt int64
		if scanErr := rows.Scan(&id, &mood, &createdAt); scanErr != nil {
			return nil, fmt.Errorf("scan checkin: %w", scanErr)
		}
		out = append(out, store.CheckIn{
			ID:        strings.TrimSpace(id),
			UserID:    userID,
			Mood:      mood,
			CreatedAt: time.Unix(createdAt, 0).UTC(),
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("list checkins iterate: %w", rowsErr)
	}
	return out, nil
}

func (s checkInStore) nowTime() time.Time {
	if s.now == nil {
		return time.Now()
	}
	return s.now()
}
