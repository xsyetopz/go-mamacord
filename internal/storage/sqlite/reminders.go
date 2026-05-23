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
	enabled := 0
	if r.Enabled {
		enabled = 1
	}

	const query = `
INSERT INTO reminders(
	id, user_id, schedule, kind, note, delivery, guild_id, channel_id,
	enabled, next_run_at, last_run_at, failure_count,
	lease_until, lease_id, created_at, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, NULL, ?, ?)`

	var lastRunAny any
	if r.LastRunAt != nil && !r.LastRunAt.IsZero() {
		lastRunAny = r.LastRunAt.UTC().Unix()
	} else {
		lastRunAny = sql.NullInt64{}
	}

	if _, execErr := s.db.ExecContext(
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
		enabled,
		r.NextRunAt.UTC().Unix(),
		lastRunAny,
		r.FailureCount,
		now,
		now,
	); execErr != nil {
		return fmt.Errorf("create reminder: %w", execErr)
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
WHERE user_id = ?
ORDER BY created_at DESC
LIMIT ?`

	rows, err := s.db.QueryContext(ctx, query, userID64, limit)
	if err != nil {
		return nil, fmt.Errorf("list reminders: %w", err)
	}
	defer rows.Close()

	out := []store.Reminder{}
	for rows.Next() {
		r, scanErr := scanReminder(rows, userID)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, r)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("list reminders iterate: %w", rowsErr)
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

	res, execErr := s.db.ExecContext(
		ctx,
		"DELETE FROM reminders WHERE id = ? AND user_id = ?",
		reminderID,
		userID64,
	)
	if execErr != nil {
		return false, fmt.Errorf("delete reminder: %w", execErr)
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

	if commitErr := tx.Commit(); commitErr != nil {
		return nil, fmt.Errorf("claim due reminders: commit: %w", commitErr)
	}

	return claimed, nil
}

func selectDueReminderCandidates(ctx context.Context, tx *sql.Tx, nowUnix int64, limit int) ([]store.Reminder, error) {
	const selectQuery = `
SELECT id, user_id, schedule, kind, note, delivery, guild_id, channel_id,
	enabled, next_run_at, last_run_at, failure_count,
	created_at, updated_at
FROM reminders
WHERE enabled = 1
	AND next_run_at <= ?
	AND (lease_until IS NULL OR lease_until < ?)
ORDER BY next_run_at ASC
LIMIT ?`

	rows, err := tx.QueryContext(ctx, selectQuery, nowUnix, nowUnix, limit)
	if err != nil {
		return nil, fmt.Errorf("claim due reminders: select: %w", err)
	}
	defer rows.Close()

	candidates := []store.Reminder{}
	for rows.Next() {
		var id string
		var userID64 int64
		var schedule, kind, note, delivery string
		var guildID, channelID sql.NullInt64
		var enabled int
		var nextRunAt int64
		var lastRunAt sql.NullInt64
		var failureCount int
		var createdAt int64
		var updatedAt int64

		if scanErr := rows.Scan(
			&id, &userID64, &schedule, &kind, &note, &delivery, &guildID, &channelID,
			&enabled, &nextRunAt, &lastRunAt, &failureCount,
			&createdAt, &updatedAt,
		); scanErr != nil {
			return nil, fmt.Errorf("claim due reminders: scan: %w", scanErr)
		}

		userU64, userConvErr := toUint64(userID64, "user_id")
		if userConvErr != nil {
			return nil, userConvErr
		}

		var guildPtr *uint64
		guildIDU64, hasGuild, guildConvErr := nullInt64ToUint64(guildID, "guild_id")
		if guildConvErr != nil {
			return nil, guildConvErr
		}
		if hasGuild {
			v := guildIDU64
			guildPtr = &v
		}

		var channelPtr *uint64
		channelIDU64, hasChannel, channelConvErr := nullInt64ToUint64(channelID, "channel_id")
		if channelConvErr != nil {
			return nil, channelConvErr
		}
		if hasChannel {
			v := channelIDU64
			channelPtr = &v
		}
		lastPtr := nullInt64ToTimePtr(lastRunAt)

		candidates = append(candidates, store.Reminder{
			ID:           strings.TrimSpace(id),
			UserID:       userU64,
			Schedule:     strings.TrimSpace(schedule),
			Kind:         strings.TrimSpace(kind),
			Note:         strings.TrimSpace(note),
			Delivery:     store.ReminderDelivery(strings.TrimSpace(delivery)),
			GuildID:      guildPtr,
			ChannelID:    channelPtr,
			Enabled:      enabled == 1,
			NextRunAt:    time.Unix(nextRunAt, 0).UTC(),
			LastRunAt:    lastPtr,
			FailureCount: failureCount,
			CreatedAt:    time.Unix(createdAt, 0).UTC(),
			UpdatedAt:    time.Unix(updatedAt, 0).UTC(),
		})
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("claim due reminders: iterate: %w", rowsErr)
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
	claimed := make([]store.Reminder, 0, len(candidates))
	const updateQuery = `
UPDATE reminders
SET lease_until = ?, lease_id = ?, updated_at = ?
WHERE id = ?
	AND enabled = 1
	AND next_run_at <= ?
	AND (lease_until IS NULL OR lease_until < ?)`

	for _, r := range candidates {
		res, execErr := tx.ExecContext(
			ctx,
			updateQuery,
			leaseUntilUnix,
			leaseID,
			updatedAtUnix,
			r.ID,
			nowUnix,
			nowUnix,
		)
		if execErr != nil {
			return nil, fmt.Errorf("claim due reminders: update: %w", execErr)
		}
		affected, _ := res.RowsAffected()
		if affected == 1 {
			claimed = append(claimed, r)
		}
	}

	return claimed, nil
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

	lastUnix := lastRunAt.UTC().Unix()
	nextUnix := nextRunAt.UTC().Unix()
	updatedAtUnix := s.nowUTC()
	enabledInt := 0
	if enabled {
		enabledInt = 1
	}

	const query = `
UPDATE reminders
SET lease_until = NULL,
	lease_id = NULL,
	last_run_at = ?,
	next_run_at = ?,
	failure_count = ?,
	enabled = ?,
	updated_at = ?
WHERE id = ? AND lease_id = ?`

	res, err := s.db.ExecContext(
		ctx,
		query,
		lastUnix,
		nextUnix,
		failureCount,
		enabledInt,
		updatedAtUnix,
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
	var id string
	var schedule, kind, note, delivery string
	var guildID, channelID sql.NullInt64
	var enabled int
	var nextRunAt int64
	var lastRunAt sql.NullInt64
	var failureCount int
	var createdAt int64
	var updatedAt int64

	if scanErr := r.Scan(
		&id, &schedule, &kind, &note, &delivery, &guildID, &channelID,
		&enabled, &nextRunAt, &lastRunAt, &failureCount, &createdAt, &updatedAt,
	); scanErr != nil {
		return store.Reminder{}, fmt.Errorf("scan reminder: %w", scanErr)
	}

	var guildPtr *uint64
	if guildID.Valid {
		v, convErr := toUint64(guildID.Int64, "guild_id")
		if convErr != nil {
			return store.Reminder{}, convErr
		}
		guildPtr = &v
	}
	var channelPtr *uint64
	if channelID.Valid {
		v, convErr := toUint64(channelID.Int64, "channel_id")
		if convErr != nil {
			return store.Reminder{}, convErr
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
		Enabled:      enabled == 1,
		NextRunAt:    time.Unix(nextRunAt, 0).UTC(),
		LastRunAt:    lastPtr,
		FailureCount: failureCount,
		CreatedAt:    time.Unix(createdAt, 0).UTC(),
		UpdatedAt:    time.Unix(updatedAt, 0).UTC(),
	}, nil
}
