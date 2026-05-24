package postgresstore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type restrictionStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s restrictionStore) GetRestriction(
	ctx context.Context,
	targetType store.TargetType,
	targetID uint64,
) (store.Restriction, bool, error) {
	const query = `
SELECT reason, created_by, created_at
FROM restrictions
WHERE target_type = $1 AND target_id = $2`

	targetIDDB, err := toInt64(targetID, "target_id")
	if err != nil {
		return store.Restriction{}, false, err
	}

	var (
		reason    string
		createdBy int64
		createdAt int64
	)
	if err := s.db.QueryRowContext(ctx, query, string(targetType), targetIDDB).Scan(&reason, &createdBy, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.Restriction{}, false, nil
		}
		return store.Restriction{}, false, fmt.Errorf("get restriction: %w", err)
	}

	createdByU64, err := toUint64(createdBy, "created_by")
	if err != nil {
		return store.Restriction{}, false, err
	}

	return store.Restriction{
		TargetType: targetType,
		TargetID:   targetID,
		Reason:     reason,
		CreatedBy:  createdByU64,
		CreatedAt:  time.Unix(createdAt, 0).UTC(),
	}, true, nil
}

func (s restrictionStore) PutRestriction(ctx context.Context, r store.Restriction) error {
	const query = `
INSERT INTO restrictions(target_type, target_id, reason, created_by, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT(target_type, target_id) DO UPDATE SET
	reason = excluded.reason,
	created_by = excluded.created_by,
	created_at = excluded.created_at`

	createdAt := r.CreatedAt
	if createdAt.IsZero() {
		createdAt = s.now().UTC()
	}

	targetIDDB, err := toInt64(r.TargetID, "target_id")
	if err != nil {
		return err
	}
	createdByDB, err := toInt64(r.CreatedBy, "created_by")
	if err != nil {
		return err
	}

	if _, err := s.db.ExecContext(
		ctx,
		query,
		string(r.TargetType),
		targetIDDB,
		r.Reason,
		createdByDB,
		createdAt.Unix(),
	); err != nil {
		return fmt.Errorf("put restriction: %w", err)
	}
	return nil
}

func (s restrictionStore) DeleteRestriction(ctx context.Context, targetType store.TargetType, targetID uint64) error {
	targetIDDB, err := toInt64(targetID, "target_id")
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(
		ctx,
		`DELETE FROM restrictions WHERE target_type = $1 AND target_id = $2`,
		string(targetType),
		targetIDDB,
	); err != nil {
		return fmt.Errorf("delete restriction: %w", err)
	}
	return nil
}
