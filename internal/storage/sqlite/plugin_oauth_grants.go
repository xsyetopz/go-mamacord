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

type pluginOAuthGrantStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s pluginOAuthGrantStore) GetPluginOAuthGrant(ctx context.Context, userID uint64, pluginID string) (store.PluginOAuthGrant, bool, error) {
	if s.db == nil {
		return store.PluginOAuthGrant{}, false, errors.New("db unavailable")
	}
	if userID == 0 {
		return store.PluginOAuthGrant{}, false, errors.New("invalid user id")
	}
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return store.PluginOAuthGrant{}, false, errors.New("plugin id is required")
	}

	row := s.db.QueryRowContext(
		ctx,
		`SELECT user_id, plugin_id, scope, created_at, updated_at
		 FROM plugin_oauth_grants
		 WHERE user_id = ? AND plugin_id = ?`,
		userID,
		pluginID,
	)
	var grant store.PluginOAuthGrant
	var createdAt int64
	var updatedAt int64
	if err := row.Scan(&grant.UserID, &grant.PluginID, &grant.Scope, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return store.PluginOAuthGrant{}, false, nil
		}
		return store.PluginOAuthGrant{}, false, err
	}
	grant.CreatedAt = time.Unix(createdAt, 0).UTC()
	grant.UpdatedAt = time.Unix(updatedAt, 0).UTC()
	return grant, true, nil
}

func (s pluginOAuthGrantStore) ListPluginOAuthGrants(ctx context.Context, userID uint64) ([]store.PluginOAuthGrant, error) {
	if s.db == nil {
		return nil, errors.New("db unavailable")
	}
	if userID == 0 {
		return nil, errors.New("invalid user id")
	}

	rows, err := s.db.QueryContext(
		ctx,
		`SELECT user_id, plugin_id, scope, created_at, updated_at
		 FROM plugin_oauth_grants
		 WHERE user_id = ?
		 ORDER BY plugin_id ASC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []store.PluginOAuthGrant{}
	for rows.Next() {
		var grant store.PluginOAuthGrant
		var createdAt int64
		var updatedAt int64
		if err := rows.Scan(&grant.UserID, &grant.PluginID, &grant.Scope, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		grant.CreatedAt = time.Unix(createdAt, 0).UTC()
		grant.UpdatedAt = time.Unix(updatedAt, 0).UTC()
		out = append(out, grant)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s pluginOAuthGrantStore) PutPluginOAuthGrant(ctx context.Context, grant store.PluginOAuthGrant) error {
	if s.db == nil {
		return errors.New("db unavailable")
	}
	if grant.UserID == 0 {
		return errors.New("invalid user id")
	}
	grant.PluginID = strings.TrimSpace(grant.PluginID)
	if grant.PluginID == "" {
		return errors.New("plugin id is required")
	}
	grant.Scope = strings.TrimSpace(grant.Scope)
	if grant.Scope == "" {
		return errors.New("scope is required")
	}

	now := time.Now().UTC()
	if s.now != nil {
		now = s.now().UTC()
	}

	createdAt := grant.CreatedAt.UTC().Unix()
	if createdAt <= 0 {
		createdAt = now.Unix()
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO plugin_oauth_grants(user_id, plugin_id, scope, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(user_id, plugin_id) DO UPDATE SET
		   scope=excluded.scope,
		   updated_at=excluded.updated_at`,
		grant.UserID,
		grant.PluginID,
		grant.Scope,
		createdAt,
		now.Unix(),
	)
	if err != nil {
		return fmt.Errorf("put plugin oauth grant: %w", err)
	}
	return nil
}

func (s pluginOAuthGrantStore) DeletePluginOAuthGrant(ctx context.Context, userID uint64, pluginID string) error {
	if s.db == nil {
		return errors.New("db unavailable")
	}
	if userID == 0 {
		return errors.New("invalid user id")
	}
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return errors.New("plugin id is required")
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM plugin_oauth_grants WHERE user_id = ? AND plugin_id = ?", userID, pluginID); err != nil {
		return fmt.Errorf("delete plugin oauth grant: %w", err)
	}
	return nil
}

func (s pluginOAuthGrantStore) CountPluginOAuthGrants(ctx context.Context, userID uint64) (int, error) {
	if s.db == nil {
		return 0, errors.New("db unavailable")
	}
	if userID == 0 {
		return 0, errors.New("invalid user id")
	}
	row := s.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM plugin_oauth_grants WHERE user_id = ?", userID)
	var count int
	if err := row.Scan(&count); err != nil {
		return 0, err
	}
	return count, nil
}
