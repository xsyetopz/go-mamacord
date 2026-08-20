package pluginspg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
	"github.com/xsyetopz/go-mamacord/internal/storage/postgres/internal/sqlvalue"
	"strings"
	"time"
)

type pluginKVStore struct {
	db  *sql.DB
	now func() time.Time
}

func (s pluginKVStore) GetPluginKV(ctx context.Context, guildID uint64, pluginID, key string) (string, bool, error) {
	pluginID = strings.TrimSpace(pluginID)
	key = strings.TrimSpace(key)
	if pluginID == "" || key == "" {
		return "", false, nil
	}

	const query = `
SELECT value_json
FROM plugin_kv
WHERE guild_id = $1 AND plugin_id = $2 AND key = $3`

	guildIDDB, err := sqlvalue.Uint64ToInt64(guildID, "guild_id")
	if err != nil {
		return "", false, err
	}

	var value string
	if err := s.db.QueryRowContext(ctx, query, guildIDDB, pluginID, key).Scan(&value); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("get plugin kv: %w", err)
	}
	return value, true, nil
}

func (s pluginKVStore) PutPluginKV(ctx context.Context, guildID uint64, pluginID, key, valueJSON string) error {
	pluginID = strings.TrimSpace(pluginID)
	key = strings.TrimSpace(key)
	if pluginID == "" || key == "" {
		return errors.New("plugin_id and key are required")
	}

	const query = `
INSERT INTO plugin_kv(guild_id, plugin_id, key, value_json, updated_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT(guild_id, plugin_id, key)
DO UPDATE SET value_json = excluded.value_json, updated_at = excluded.updated_at, version = plugin_kv.version + 1`

	guildIDDB, err := sqlvalue.Uint64ToInt64(guildID, "guild_id")
	if err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, query, guildIDDB, pluginID, key, valueJSON, s.now().UTC().Unix()); err != nil {
		return fmt.Errorf("put plugin kv: %w", err)
	}
	return nil
}

func (s pluginKVStore) DeletePluginKV(ctx context.Context, guildID uint64, pluginID, key string) error {
	pluginID = strings.TrimSpace(pluginID)
	key = strings.TrimSpace(key)
	if pluginID == "" || key == "" {
		return errors.New("plugin_id and key are required")
	}

	guildIDDB, err := sqlvalue.Uint64ToInt64(guildID, "guild_id")
	if err != nil {
		return err
	}

	const query = `DELETE FROM plugin_kv WHERE guild_id = $1 AND plugin_id = $2 AND key = $3`
	if _, err := s.db.ExecContext(ctx, query, guildIDDB, pluginID, key); err != nil {
		return fmt.Errorf("delete plugin kv: %w", err)
	}
	return nil
}

func (s pluginKVStore) GetPluginKVVersioned(ctx context.Context, guildID uint64, pluginID, key string) (storage.PluginKVValue, bool, error) {
	pluginID = strings.TrimSpace(pluginID)
	key = strings.TrimSpace(key)
	if pluginID == "" || key == "" {
		return storage.PluginKVValue{}, false, nil
	}
	guildIDDB, err := sqlvalue.Uint64ToInt64(guildID, "guild_id")
	if err != nil {
		return storage.PluginKVValue{}, false, err
	}
	var value storage.PluginKVValue
	var version int64
	err = s.db.QueryRowContext(ctx, `SELECT value_json, version FROM plugin_kv WHERE guild_id = $1 AND plugin_id = $2 AND key = $3`, guildIDDB, pluginID, key).Scan(&value.ValueJSON, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return storage.PluginKVValue{}, false, nil
	}
	if err != nil {
		return storage.PluginKVValue{}, false, fmt.Errorf("get versioned plugin kv: %w", err)
	}
	if version <= 0 {
		return storage.PluginKVValue{}, false, errors.New("plugin kv version is invalid")
	}
	value.Version = uint64(version)
	return value, true, nil
}
func (s pluginKVStore) CompareAndSwapPluginKV(ctx context.Context, guildID uint64, pluginID, key, valueJSON string, expectedVersion uint64) (uint64, bool, error) {
	pluginID = strings.TrimSpace(pluginID)
	key = strings.TrimSpace(key)
	if pluginID == "" || key == "" {
		return 0, false, errors.New("plugin_id and key are required")
	}
	guildIDDB, err := sqlvalue.Uint64ToInt64(guildID, "guild_id")
	if err != nil {
		return 0, false, err
	}
	now := s.now().UTC().Unix()
	var version int64
	if expectedVersion == 0 {
		err = s.db.QueryRowContext(ctx, `INSERT INTO plugin_kv(guild_id,plugin_id,key,value_json,updated_at,version) VALUES($1,$2,$3,$4,$5,1) ON CONFLICT(guild_id,plugin_id,key) DO NOTHING RETURNING version`, guildIDDB, pluginID, key, valueJSON, now).Scan(&version)
	} else {
		expected, convertErr := sqlvalue.Uint64ToInt64(expectedVersion, "expected_version")
		if convertErr != nil {
			return 0, false, convertErr
		}
		err = s.db.QueryRowContext(ctx, `UPDATE plugin_kv SET value_json=$4,updated_at=$5,version=version+1 WHERE guild_id=$1 AND plugin_id=$2 AND key=$3 AND version=$6 RETURNING version`, guildIDDB, pluginID, key, valueJSON, now, expected).Scan(&version)
	}
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("compare and swap plugin kv: %w", err)
	}
	return uint64(version), true, nil
}
func (s pluginKVStore) DeletePluginKVVersion(ctx context.Context, guildID uint64, pluginID, key string, expectedVersion uint64) (bool, error) {
	pluginID = strings.TrimSpace(pluginID)
	key = strings.TrimSpace(key)
	if pluginID == "" || key == "" {
		return false, errors.New("plugin_id and key are required")
	}
	guildIDDB, err := sqlvalue.Uint64ToInt64(guildID, "guild_id")
	if err != nil {
		return false, err
	}
	expected, err := sqlvalue.Uint64ToInt64(expectedVersion, "expected_version")
	if err != nil {
		return false, err
	}
	result, err := s.db.ExecContext(ctx, `DELETE FROM plugin_kv WHERE guild_id=$1 AND plugin_id=$2 AND key=$3 AND version=$4`, guildIDDB, pluginID, key, expected)
	if err != nil {
		return false, fmt.Errorf("delete versioned plugin kv: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("delete versioned plugin kv rows: %w", err)
	}
	return rows == 1, nil
}
