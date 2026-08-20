package storage

import (
	"context"
	"time"
)

type PluginKVStore interface {
	GetPluginKV(ctx context.Context, guildID uint64, pluginID, key string) (valueJSON string, ok bool, err error)
	PutPluginKV(ctx context.Context, guildID uint64, pluginID, key, valueJSON string) error
	DeletePluginKV(ctx context.Context, guildID uint64, pluginID, key string) error
}

type PluginKVValue struct {
	ValueJSON string
	Version   uint64
}

type VersionedPluginKVStore interface {
	PluginKVStore
	GetPluginKVVersioned(ctx context.Context, guildID uint64, pluginID, key string) (PluginKVValue, bool, error)
	CompareAndSwapPluginKV(ctx context.Context, guildID uint64, pluginID, key, valueJSON string, expectedVersion uint64) (uint64, bool, error)
	DeletePluginKVVersion(ctx context.Context, guildID uint64, pluginID, key string, expectedVersion uint64) (bool, error)
}

type ModuleState struct {
	ModuleID  string
	Enabled   bool
	UpdatedAt time.Time
	UpdatedBy *uint64
}

type ModuleStateStore interface {
	GetModuleState(ctx context.Context, moduleID string) (ModuleState, bool, error)
	ListModuleStates(ctx context.Context) ([]ModuleState, error)
	PutModuleState(ctx context.Context, state ModuleState) error
	DeleteModuleState(ctx context.Context, moduleID string) error
}

type PluginOAuthGrant struct {
	UserID    uint64
	PluginID  string
	Scope     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type PluginOAuthGrantStore interface {
	GetPluginOAuthGrant(ctx context.Context, userID uint64, pluginID string) (PluginOAuthGrant, bool, error)
	ListPluginOAuthGrants(ctx context.Context, userID uint64) ([]PluginOAuthGrant, error)
	PutPluginOAuthGrant(ctx context.Context, grant PluginOAuthGrant) error
	DeletePluginOAuthGrant(ctx context.Context, userID uint64, pluginID string) error
	CountPluginOAuthGrants(ctx context.Context, userID uint64) (int, error)
}
