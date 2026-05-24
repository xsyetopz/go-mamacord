package guildconfig

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
)

const KVKey = "__guild_config"

type PluginConfig struct {
	Enabled                  bool            `json:"enabled"`
	Commands                 map[string]bool `json:"commands"`
	WarningLimit             int             `json:"warning_limit,omitempty"`
	TimeoutThreshold         int             `json:"timeout_threshold,omitempty"`
	TimeoutMinutes           int             `json:"timeout_minutes,omitempty"`
	AllowChannelReminders    bool            `json:"allow_channel_reminders,omitempty"`
	DefaultReminderChannelID uint64          `json:"default_reminder_channel_id,omitempty"`
}

var pluginCommands = map[string][]string{
	"fun":        {"8ball", "flip", "hug", "pat", "poke", "roll", "shrug"},
	"info":       {"about", "lookup"},
	"manager":    {"slowmode", "nick", "roles", "purge", "emojis", "stickers"},
	"moderation": {"warn", "unwarn"},
	"wellness":   {"timezone", "checkin", "remind"},
}

func KnownPlugins() []string {
	out := make([]string, 0, len(pluginCommands))
	for pluginID := range pluginCommands {
		out = append(out, pluginID)
	}
	sort.Strings(out)
	return out
}

func Commands(pluginID string) []string {
	commands := pluginCommands[pluginID]
	if len(commands) == 0 {
		return nil
	}
	out := make([]string, len(commands))
	copy(out, commands)
	return out
}

func Default(pluginID string) PluginConfig {
	cfg := PluginConfig{
		Enabled:  true,
		Commands: map[string]bool{},
	}
	for _, command := range pluginCommands[pluginID] {
		cfg.Commands[command] = true
	}

	switch pluginID {
	case "moderation":
		cfg.WarningLimit = 3
		cfg.TimeoutThreshold = 3
		cfg.TimeoutMinutes = 10
	case "wellness":
		cfg.AllowChannelReminders = true
	}
	return cfg
}

func Load(ctx context.Context, store commandruntime.Store, guildID uint64, pluginID string) (PluginConfig, error) {
	cfg := Default(pluginID)
	if guildID == 0 || pluginID == "" {
		return cfg, nil
	}
	if store == nil || store.PluginKV() == nil {
		return cfg, errors.New("plugin kv store unavailable")
	}

	raw, ok, err := store.PluginKV().GetPluginKV(ctx, guildID, pluginID, KVKey)
	if err != nil {
		return PluginConfig{}, err
	}
	if !ok || raw == "" {
		return cfg, nil
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return cfg, nil
	}

	if v, ok := payload["enabled"]; ok {
		var enabled bool
		if json.Unmarshal(v, &enabled) == nil {
			cfg.Enabled = enabled
		}
	}
	if v, ok := payload["commands"]; ok {
		var commands map[string]bool
		if json.Unmarshal(v, &commands) == nil {
			for _, command := range pluginCommands[pluginID] {
				if enabled, has := commands[command]; has {
					cfg.Commands[command] = enabled
				}
			}
		}
	}

	switch pluginID {
	case "moderation":
		cfg.WarningLimit = loadInt(payload["warning_limit"], cfg.WarningLimit)
		cfg.TimeoutThreshold = loadInt(payload["timeout_threshold"], cfg.TimeoutThreshold)
		cfg.TimeoutMinutes = loadInt(payload["timeout_minutes"], cfg.TimeoutMinutes)
	case "wellness":
		cfg.AllowChannelReminders = loadBool(payload["allow_channel_reminders"], cfg.AllowChannelReminders)
		cfg.DefaultReminderChannelID = loadUint64(payload["default_reminder_channel_id"], cfg.DefaultReminderChannelID)
	}

	return Normalize(pluginID, cfg), nil
}

func Save(ctx context.Context, store commandruntime.Store, guildID uint64, pluginID string, cfg PluginConfig) (PluginConfig, error) {
	if guildID == 0 || pluginID == "" {
		return PluginConfig{}, errors.New("invalid guild plugin config target")
	}
	if store == nil || store.PluginKV() == nil {
		return PluginConfig{}, errors.New("plugin kv store unavailable")
	}

	cfg = Normalize(pluginID, cfg)
	body, err := json.Marshal(cfg)
	if err != nil {
		return PluginConfig{}, err
	}
	if err := store.PluginKV().PutPluginKV(ctx, guildID, pluginID, KVKey, string(body)); err != nil {
		return PluginConfig{}, err
	}
	return cfg, nil
}

func PluginEnabled(ctx context.Context, store commandruntime.Store, guildID uint64, pluginID string) (bool, error) {
	cfg, err := Load(ctx, store, guildID, pluginID)
	if err != nil {
		return false, err
	}
	return cfg.Enabled, nil
}

func CommandEnabled(ctx context.Context, store commandruntime.Store, guildID uint64, pluginID, commandName string) (bool, error) {
	cfg, err := Load(ctx, store, guildID, pluginID)
	if err != nil {
		return false, err
	}
	if !cfg.Enabled {
		return false, nil
	}
	enabled, ok := cfg.Commands[commandName]
	if !ok {
		return true, nil
	}
	return enabled, nil
}

func Normalize(pluginID string, cfg PluginConfig) PluginConfig {
	normalized := Default(pluginID)
	normalized.Enabled = cfg.Enabled
	if cfg.Commands != nil {
		for _, command := range pluginCommands[pluginID] {
			if enabled, ok := cfg.Commands[command]; ok {
				normalized.Commands[command] = enabled
			}
		}
	}

	switch pluginID {
	case "moderation":
		normalized.WarningLimit = clamp(cfg.WarningLimit, 1, 20, normalized.WarningLimit)
		normalized.TimeoutThreshold = clamp(cfg.TimeoutThreshold, 1, 20, normalized.TimeoutThreshold)
		normalized.TimeoutMinutes = clamp(cfg.TimeoutMinutes, 1, 10080, normalized.TimeoutMinutes)
	case "wellness":
		normalized.AllowChannelReminders = cfg.AllowChannelReminders
		normalized.DefaultReminderChannelID = cfg.DefaultReminderChannelID
	}

	return normalized
}

func loadInt(raw json.RawMessage, fallback int) int {
	if len(raw) == 0 {
		return fallback
	}
	var value int
	if err := json.Unmarshal(raw, &value); err != nil {
		return fallback
	}
	return value
}

func loadUint64(raw json.RawMessage, fallback uint64) uint64 {
	if len(raw) == 0 {
		return fallback
	}
	var value uint64
	if err := json.Unmarshal(raw, &value); err != nil {
		return fallback
	}
	return value
}

func loadBool(raw json.RawMessage, fallback bool) bool {
	if len(raw) == 0 {
		return fallback
	}
	var value bool
	if err := json.Unmarshal(raw, &value); err != nil {
		return fallback
	}
	return value
}

func clamp(value, minValue, maxValue, fallback int) int {
	if value == 0 {
		return fallback
	}
	if value < minValue {
		return minValue
	}
	if value > maxValue {
		return maxValue
	}
	return value
}
