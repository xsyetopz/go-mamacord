package postgresstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type userSettingsStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s userSettingsStore) GetUserSettings(ctx context.Context, userID uint64) (store.UserSettings, bool, error) {
	if s.db == nil {
		return store.UserSettings{}, false, errors.New("db not configured")
	}

	userID64, err := toInt64(userID, "user_id")
	if err != nil {
		return store.UserSettings{}, false, err
	}

	const query = `
SELECT timezone, dm_channel_id, created_at, updated_at
FROM user_settings
WHERE user_id = $1`

	var (
		timezone  string
		dmChannel sql.NullInt64
		createdAt int64
		updatedAt int64
	)
	if err := s.db.QueryRowContext(ctx, query, userID64).Scan(&timezone, &dmChannel, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.UserSettings{}, false, nil
		}
		return store.UserSettings{}, false, fmt.Errorf("get user_settings: %w", err)
	}

	var dmPtr *uint64
	if dmChannel.Valid {
		v, err := toUint64(dmChannel.Int64, "dm_channel_id")
		if err != nil {
			return store.UserSettings{}, false, err
		}
		dmPtr = &v
	}

	return store.UserSettings{
		UserID:      userID,
		Timezone:    strings.TrimSpace(timezone),
		DMChannelID: dmPtr,
		CreatedAt:   time.Unix(createdAt, 0).UTC(),
		UpdatedAt:   time.Unix(updatedAt, 0).UTC(),
	}, true, nil
}

func (s userSettingsStore) UpsertUserTimezone(ctx context.Context, userID uint64, timezone string) error {
	if s.db == nil {
		return errors.New("db not configured")
	}

	userID64, err := toInt64(userID, "user_id")
	if err != nil {
		return err
	}

	now := s.nowUTC()
	tz := strings.TrimSpace(timezone)

	const query = `
INSERT INTO user_settings(user_id, timezone, created_at, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT(user_id)
DO UPDATE SET timezone = excluded.timezone, updated_at = excluded.updated_at`

	if _, err := s.db.ExecContext(ctx, query, userID64, tz, now, now); err != nil {
		return fmt.Errorf("upsert timezone: %w", err)
	}
	return nil
}

func (s userSettingsStore) ClearUserTimezone(ctx context.Context, userID uint64) error {
	return s.UpsertUserTimezone(ctx, userID, "")
}

func (s userSettingsStore) UpsertUserDMChannelID(ctx context.Context, userID uint64, dmChannelID uint64) error {
	if s.db == nil {
		return errors.New("db not configured")
	}

	userID64, err := toInt64(userID, "user_id")
	if err != nil {
		return err
	}
	dmID64, err := toInt64(dmChannelID, "dm_channel_id")
	if err != nil {
		return err
	}

	now := s.nowUTC()

	const query = `
INSERT INTO user_settings(user_id, dm_channel_id, created_at, updated_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT(user_id)
DO UPDATE SET dm_channel_id = excluded.dm_channel_id, updated_at = excluded.updated_at`

	if _, err := s.db.ExecContext(ctx, query, userID64, dmID64, now, now); err != nil {
		return fmt.Errorf("upsert dm_channel_id: %w", err)
	}
	return nil
}

func (s userSettingsStore) nowUTC() int64 {
	if s.now == nil {
		return time.Now().UTC().Unix()
	}
	return s.now().UTC().Unix()
}
