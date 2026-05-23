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

type moduleStateStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s moduleStateStore) GetModuleState(ctx context.Context, moduleID string) (store.ModuleState, bool, error) {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return store.ModuleState{}, false, nil
	}

	const query = `
SELECT module_id, enabled, updated_at, updated_by
FROM module_states
WHERE module_id = ?`

	var (
		id        string
		enabled   bool
		updatedAt int64
		updatedBy sql.NullInt64
	)

	err := s.db.QueryRowContext(ctx, query, moduleID).Scan(&id, &enabled, &updatedAt, &updatedBy)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.ModuleState{}, false, nil
		}
		return store.ModuleState{}, false, fmt.Errorf("get module state: %w", err)
	}

	return scanModuleState(id, enabled, updatedAt, updatedBy)
}

func (s moduleStateStore) ListModuleStates(ctx context.Context) ([]store.ModuleState, error) {
	const query = `
SELECT module_id, enabled, updated_at, updated_by
FROM module_states
ORDER BY module_id`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list module states: %w", err)
	}
	defer rows.Close()

	var out []store.ModuleState
	for rows.Next() {
		var (
			id        string
			enabled   bool
			updatedAt int64
			updatedBy sql.NullInt64
		)
		if scanErr := rows.Scan(&id, &enabled, &updatedAt, &updatedBy); scanErr != nil {
			return nil, fmt.Errorf("scan module state: %w", scanErr)
		}
		state, ok, convErr := scanModuleState(id, enabled, updatedAt, updatedBy)
		if convErr != nil {
			return nil, convErr
		}
		if ok {
			out = append(out, state)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate module states: %w", err)
	}

	return out, nil
}

func (s moduleStateStore) PutModuleState(ctx context.Context, state store.ModuleState) error {
	moduleID := strings.TrimSpace(state.ModuleID)
	if moduleID == "" {
		return errors.New("module_id is required")
	}

	updatedAt := state.UpdatedAt
	if updatedAt.IsZero() {
		updatedAt = s.now().UTC()
	}

	var updatedBy any
	if state.UpdatedBy != nil {
		id, err := toInt64(*state.UpdatedBy, "updated_by")
		if err != nil {
			return err
		}
		updatedBy = id
	}

	const query = `
INSERT INTO module_states(module_id, enabled, updated_at, updated_by)
VALUES (?, ?, ?, ?)
ON CONFLICT(module_id) DO UPDATE SET
	enabled = excluded.enabled,
	updated_at = excluded.updated_at,
	updated_by = excluded.updated_by`

	_, err := s.db.ExecContext(ctx, query, moduleID, state.Enabled, updatedAt.Unix(), updatedBy)
	if err != nil {
		return fmt.Errorf("put module state: %w", err)
	}
	return nil
}

func (s moduleStateStore) DeleteModuleState(ctx context.Context, moduleID string) error {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return errors.New("module_id is required")
	}

	_, err := s.db.ExecContext(ctx, "DELETE FROM module_states WHERE module_id = ?", moduleID)
	if err != nil {
		return fmt.Errorf("delete module state: %w", err)
	}
	return nil
}

func scanModuleState(
	moduleID string,
	enabled bool,
	updatedAt int64,
	updatedBy sql.NullInt64,
) (store.ModuleState, bool, error) {
	moduleID = strings.TrimSpace(moduleID)
	if moduleID == "" {
		return store.ModuleState{}, false, nil
	}

	var actor *uint64
	if updatedBy.Valid {
		id, err := toUint64(updatedBy.Int64, "updated_by")
		if err != nil {
			return store.ModuleState{}, false, err
		}
		actor = &id
	}

	return store.ModuleState{
		ModuleID:  moduleID,
		Enabled:   enabled,
		UpdatedAt: time.Unix(updatedAt, 0).UTC(),
		UpdatedBy: actor,
	}, true, nil
}
