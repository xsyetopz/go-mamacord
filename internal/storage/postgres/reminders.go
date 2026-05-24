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

type reminderStore struct {
	db  *sql.DB
	now func() time.Time
}

const (
	defaultReminderListLimit     = 25
	defaultReminderClaimLimit    = 25
	defaultReminderLeaseDuration = 30 * time.Second
)

func (s reminderStore) CreateReminder(ctx context.Context, r store.Reminder) error {
	if s.db == nil {
		return errors.New("db not configured")
	}
	if strings.TrimSpace(r.ID) == "" {
		return errors.New("reminder id is required")
	}
	if r.UserID == 0 {
		return errors.New("reminder user_id is required")
	}
	if strings.TrimSpace(r.Schedule) == "" {
		return errors.New("reminder schedule is required")
	}
	if strings.TrimSpace(r.Kind) == "" {
		return errors.New("reminder kind is required")
	}
	if r.NextRunAt.IsZero() {
		return errors.New("reminder next_run_at is required")
	}

	userID64, err := toInt64(r.UserID, "user_id")
	if err != nil {
		return err
	}
	guildAny, err := toAnyInt64Ptr(r.GuildID, "guild_id")
	if err != nil {
		return err
	}
	channelAny, err := toAnyInt64Ptr(r.ChannelID, "channel_id")
	if err != nil {
		return err
	}

	now := s.nowUTC()

	const query = `
INSERT INTO reminders(
	id, user_id, schedule, kind, note, delivery, guild_id, channel_id,
	enabled, next_run_at, last_run_at, failure_count,
	lease_until, lease_id, created_at, updated_at
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, NULL, NULL, $13, $14)`

	var lastRunAny any = sql.NullInt64{}
	if r.LastRunAt != nil && !r.LastRunAt.IsZero() {
		lastRunAny = r.LastRunAt.UTC().Unix()
	}

	if _, err := s.db.ExecContext(
		ctx,
		query,
		strings.TrimSpace(r.ID),
		userID64,
		strings.TrimSpace(r.Schedule),
		strings.TrimSpace(r.Kind),
		strings.TrimSpace(r.Note),
		string(r.Delivery),
		guildAny,
		channelAny,
		r.Enabled,
		r.NextRunAt.UTC().Unix(),
		lastRunAny,
		r.FailureCount,
		now,
		now,
	); err != nil {
		return fmt.Errorf("create reminder: %w", err)
	}
	return nil
}

func (s reminderStore) ListReminders(ctx context.Context, userID uint64, limit int) ([]store.Reminder, error) {
	if s.db == nil {
		return nil, errors.New("db not configured")
	}
	if limit <= 0 {
		limit = defaultReminderListLimit
	}

	userID64, err := toInt64(userID, "user_id")
	if err != nil {
		return nil, err
	}

	const query = `
SELECT id, schedule, kind, note, delivery, guild_id, channel_id,
	enabled, next_run_at, last_run_at, failure_count,
	created_at, updated_at
FROM reminders
WHERE user_id = $1
ORDER BY created_at DESC
LIMIT $2`

	rows, err := s.db.QueryContext(ctx, query, userID64, limit)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	defer rows.Close()

	out := []store.Reminder{}
	for rows.Next() {
		item, err := scanReminder(rows, userID)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list reminders iterate: %w", err)
	}
	return out, nil
}

func (s reminderStore) DeleteReminder(ctx context.Context, userID uint64, reminderID string) (bool, error) {
	if s.db == nil {
		return false, errors.New("db not configured")
	}

	reminderID = strings.TrimSpace(reminderID)
	if reminderID == "" {
		return false, nil
	}

	userID64, err := toInt64(userID, "user_id")
	if err != nil {
		return false, err
	}

	res, err := s.db.ExecContext(ctx, `DELETE FROM reminders WHERE id = $1 AND user_id = $2`, reminderID, userID64)
	if err != nil {
		return false, fmt.Errorf("delete reminder: %w", err)
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (s reminderStore) ClaimDueReminders(
	ctx context.Context,
	now time.Time,
	leaseID string,
	leaseDuration time.Duration,
	limit int,
) ([]store.Reminder, error) {
	if s.db == nil {
		return nil, errors.New("db not configured")
	}
	leaseID = strings.TrimSpace(leaseID)
	if leaseID == "" {
		return nil, errors.New("leaseID is required")
	}
	if leaseDuration <= 0 {
		leaseDuration = defaultReminderLeaseDuration
	}
	if limit <= 0 {
		limit = defaultReminderClaimLimit
	}

	now = now.UTC()
	nowUnix := now.Unix()
	leaseUntilUnix := now.Add(leaseDuration).Unix()
	updatedAtUnix := s.nowUTC()

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("claim due reminders: begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	candidates, err := selectDueReminderCandidates(ctx, tx, nowUnix, limit)
	if err != nil {
		return nil, err
	}
	claimed, err := claimReminderCandidates(ctx, tx, candidates, leaseUntilUnix, leaseID, updatedAtUnix, nowUnix)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("claim due reminders: commit: %w", err)
	}
	return claimed, nil
}

func selectDueReminderCandidates(ctx context.Context, tx *sql.Tx, nowUnix int64, limit int) ([]store.Reminder, error) {
	const query = `
SELECT id, user_id, schedule, kind, note, delivery, guild_id, channel_id,
	enabled, next_run_at, last_run_at, failure_count,
	created_at, updated_at
FROM reminders
WHERE enabled = $1
	AND next_run_at <= $2
	AND (lease_until IS NULL OR lease_until < $3)
ORDER BY next_run_at ASC
LIMIT $4`

	rows, err := tx.QueryContext(ctx, query, true, nowUnix, nowUnix, limit)
	if err != nil {
		return nil, fmt.Errorf("claim due reminders: select: %w", err)
	}
	defer rows.Close()

	candidates := []store.Reminder{}
	for rows.Next() {
		var (
			id           string
			userID64     int64
			schedule     string
			kind         string
			note         string
			delivery     string
			guildID      sql.NullInt64
			channelID    sql.NullInt64
			enabled      bool
			nextRunAt    int64
			lastRunAt    sql.NullInt64
			failureCount int
			createdAt    int64
			updatedAt    int64
		)
		if err := rows.Scan(
			&id, &userID64, &schedule, &kind, &note, &delivery, &guildID, &channelID,
			&enabled, &nextRunAt, &lastRunAt, &failureCount, &createdAt, &updatedAt,
		); err != nil {
			return nil, fmt.Errorf("claim due reminders: scan: %w", err)
		}

		userIDU64, err := toUint64(userID64, "user_id")
		if err != nil {
			return nil, err
		}

		var guildPtr *uint64
		guildIDU64, hasGuild, err := nullInt64ToUint64(guildID, "guild_id")
		if err != nil {
			return nil, err
		}
		if hasGuild {
			v := guildIDU64
			guildPtr = &v
		}

		var channelPtr *uint64
		channelIDU64, hasChannel, err := nullInt64ToUint64(channelID, "channel_id")
		if err != nil {
			return nil, err
		}
		if hasChannel {
			v := channelIDU64
			channelPtr = &v
		}

		candidates = append(candidates, store.Reminder{
			ID:           strings.TrimSpace(id),
			UserID:       userIDU64,
			Schedule:     strings.TrimSpace(schedule),
			Kind:         strings.TrimSpace(kind),
			Note:         strings.TrimSpace(note),
			Delivery:     store.ReminderDelivery(strings.TrimSpace(delivery)),
			GuildID:      guildPtr,
			ChannelID:    channelPtr,
			Enabled:      enabled,
			NextRunAt:    time.Unix(nextRunAt, 0).UTC(),
			LastRunAt:    nullInt64ToTimePtr(lastRunAt),
			FailureCount: failureCount,
			CreatedAt:    time.Unix(createdAt, 0).UTC(),
			UpdatedAt:    time.Unix(updatedAt, 0).UTC(),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("claim due reminders: iterate: %w", err)
	}

	return candidates, nil
}

func claimReminderCandidates(
	ctx context.Context,
	tx *sql.Tx,
	candidates []store.Reminder,
	leaseUntilUnix int64,
	leaseID string,
	updatedAtUnix int64,
	nowUnix int64,
) ([]store.Reminder, error) {
	const query = `
UPDATE reminders
SET lease_until = $1, lease_id = $2, updated_at = $3
WHERE id = $4
	AND enabled = $5
	AND next_run_at <= $6
	AND (lease_until IS NULL OR lease_until < $7)`

	claimed := make([]store.Reminder, 0, len(candidates))
	for _, reminder := range candidates {
		res, err := tx.ExecContext(
			ctx,
			query,
			leaseUntilUnix,
			leaseID,
			updatedAtUnix,
			reminder.ID,
			true,
			nowUnix,
			nowUnix,
		)
		if err != nil {
			return nil, fmt.Errorf("claim due reminders: update: %w", err)
		}
		affected, _ := res.RowsAffected()
		if affected == 1 {
			claimed = append(claimed, reminder)
		}
	}
	return claimed, nil
}

func (s reminderStore) FinishReminderRun(
	ctx context.Context,
	reminderID string,
	leaseID string,
	lastRunAt time.Time,
	nextRunAt time.Time,
	failureCount int,
	enabled bool,
) error {
	if s.db == nil {
		return errors.New("db not configured")
	}

	reminderID = strings.TrimSpace(reminderID)
	leaseID = strings.TrimSpace(leaseID)
	if reminderID == "" || leaseID == "" {
		return errors.New("reminderID and leaseID are required")
	}

	const query = `
UPDATE reminders
SET lease_until = NULL,
	lease_id = NULL,
	last_run_at = $1,
	next_run_at = $2,
	failure_count = $3,
	enabled = $4,
	updated_at = $5
WHERE id = $6 AND lease_id = $7`

	res, err := s.db.ExecContext(
		ctx,
		query,
		lastRunAt.UTC().Unix(),
		nextRunAt.UTC().Unix(),
		failureCount,
		enabled,
		s.nowUTC(),
		reminderID,
		leaseID,
	)
	if err != nil {
		return fmt.Errorf("finish reminder run: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		return errors.New("finish reminder run: reminder not leased by this worker")
	}
	return nil
}

func (s reminderStore) nowUTC() int64 {
	if s.now == nil {
		return time.Now().UTC().Unix()
	}
	return s.now().UTC().Unix()
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanReminder(r rowScanner, userID uint64) (store.Reminder, error) {
	var (
		id           string
		schedule     string
		kind         string
		note         string
		delivery     string
		guildID      sql.NullInt64
		channelID    sql.NullInt64
		enabled      bool
		nextRunAt    int64
		lastRunAt    sql.NullInt64
		failureCount int
		createdAt    int64
		updatedAt    int64
	)
	if err := r.Scan(
		&id, &schedule, &kind, &note, &delivery, &guildID, &channelID,
		&enabled, &nextRunAt, &lastRunAt, &failureCount, &createdAt, &updatedAt,
	); err != nil {
		return store.Reminder{}, fmt.Errorf("scan reminder: %w", err)
	}

	var guildPtr *uint64
	if guildID.Valid {
		v, err := toUint64(guildID.Int64, "guild_id")
		if err != nil {
			return store.Reminder{}, err
		}
		guildPtr = &v
	}

	var channelPtr *uint64
	if channelID.Valid {
		v, err := toUint64(channelID.Int64, "channel_id")
		if err != nil {
			return store.Reminder{}, err
		}
		channelPtr = &v
	}

	var lastPtr *time.Time
	if lastRunAt.Valid {
		t := time.Unix(lastRunAt.Int64, 0).UTC()
		lastPtr = &t
	}

	return store.Reminder{
		ID:           strings.TrimSpace(id),
		UserID:       userID,
		Schedule:     strings.TrimSpace(schedule),
		Kind:         strings.TrimSpace(kind),
		Note:         strings.TrimSpace(note),
		Delivery:     store.ReminderDelivery(strings.TrimSpace(delivery)),
		GuildID:      guildPtr,
		ChannelID:    channelPtr,
		Enabled:      enabled,
		NextRunAt:    time.Unix(nextRunAt, 0).UTC(),
		LastRunAt:    lastPtr,
		FailureCount: failureCount,
		CreatedAt:    time.Unix(createdAt, 0).UTC(),
		UpdatedAt:    time.Unix(updatedAt, 0).UTC(),
	}, nil
}

func nullInt64ToUint64(v sql.NullInt64, field string) (uint64, bool, error) {
	if !v.Valid {
		return 0, false, nil
	}
	out, err := toUint64(v.Int64, field)
	if err != nil {
		return 0, false, err
	}
	return out, true, nil
}

func nullInt64ToTimePtr(v sql.NullInt64) *time.Time {
	if !v.Valid {
		return nil
	}
	t := time.Unix(v.Int64, 0).UTC()
	return &t
}
