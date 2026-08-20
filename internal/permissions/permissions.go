package permissions

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const SchemaURL = "https://raw.githubusercontent.com/xsyetopz/go-mamacord/refs/heads/main/schemas/permissions.schema.json"

// Permissions are host-enforced capabilities.
//
// Security model: a plugin can only do what it both (1) requests in plugin.json
// and (2) is granted in the host permissions policy. Everything is denied by
// default.
type Permissions struct {
	Storage   StoragePermissions  `json:"storage"`
	Discord   DiscordPermissions  `json:"discord"`
	Network   NetworkPermissions  `json:"network"`
	Resources ResourcePermissions `json:"resources"`

	// Automation covers non-interaction triggers: scheduled jobs and gateway events.
	Automation AutomationPermissions `json:"automation"`
}

type StoragePermissions struct {
	KV           bool `json:"kv"`
	UserSettings bool `json:"user_settings"`
	CheckIns     bool `json:"checkins"`
	Reminders    bool `json:"reminders"`
	Warnings     bool `json:"warnings"`
	Audit        bool `json:"audit"`
}

type DiscordPermissions struct {
	Users     bool `json:"users"`
	Guilds    bool `json:"guilds"`
	Channels  bool `json:"channels"`
	Messages  bool `json:"messages"`
	Reactions bool `json:"reactions"`
	Members   bool `json:"members"`
	Roles     bool `json:"roles"`
	Threads   bool `json:"threads"`
	Invites   bool `json:"invites"`
	Webhooks  bool `json:"webhooks"`
	Emojis    bool `json:"emojis"`
	Stickers  bool `json:"stickers"`
}

type NetworkPermissions struct {
	HTTP bool `json:"http"`
}
type ResourcePermissions struct {
	Read bool `json:"read"`
}

type AutomationPermissions struct {
	// Jobs allows scheduled triggers declared by a plugin manifest.
	Jobs bool `json:"jobs"`

	Events AutomationEventPermissions `json:"events"`
}

type AutomationEventPermissions struct {
	// MemberJoinLeave covers member join/leave hooks.
	MemberJoinLeave bool `json:"member_join_leave"`

	// Moderation covers ban/unban hooks.
	Moderation bool `json:"moderation"`
}

// Policy is a Claude-style central permissions file: defaults + per-plugin overrides.
type Policy struct {
	Schema   string                 `json:"$schema"`
	Defaults Permissions            `json:"defaults"`
	Plugins  map[string]Permissions `json:"plugins"`
}

func LoadPolicyFile(path string) (Policy, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return Policy{}, nil
	}

	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Policy{}, nil
		}
		return Policy{}, fmt.Errorf("read permissions policy %q: %w", path, err)
	}

	var p Policy
	if unmarshalErr := json.Unmarshal(b, &p); unmarshalErr != nil {
		return Policy{}, fmt.Errorf("parse permissions policy %q: %w", path, unmarshalErr)
	}
	if p.Schema != SchemaURL {
		return Policy{}, fmt.Errorf("permissions policy $schema must be %q", SchemaURL)
	}
	if p.Plugins == nil {
		p.Plugins = map[string]Permissions{}
	}
	return p, nil
}

func WritePolicyFile(path string, policy Policy) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("permissions policy path is required")
	}
	policy.Schema = SchemaURL
	if policy.Plugins == nil {
		policy.Plugins = map[string]Permissions{}
	}

	bytes, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal permissions policy %q: %w", path, err)
	}
	bytes = append(bytes, '\n')

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create permissions dir for %q: %w", path, err)
	}
	if err := os.WriteFile(path, bytes, 0o644); err != nil {
		return fmt.Errorf("write permissions policy %q: %w", path, err)
	}
	return nil
}

func (p Policy) Granted(pluginID string) Permissions {
	g := p.Defaults
	if strings.TrimSpace(pluginID) == "" {
		return g
	}
	if override, ok := p.Plugins[pluginID]; ok {
		g = merge(g, override)
	}
	return g
}

// Effective returns the permissions a plugin actually gets: requested ∩ granted.
func Effective(requested, granted Permissions) Permissions {
	return Permissions{
		Storage: StoragePermissions{
			KV:           requested.Storage.KV && granted.Storage.KV,
			UserSettings: requested.Storage.UserSettings && granted.Storage.UserSettings,
			CheckIns:     requested.Storage.CheckIns && granted.Storage.CheckIns,
			Reminders:    requested.Storage.Reminders && granted.Storage.Reminders,
			Warnings:     requested.Storage.Warnings && granted.Storage.Warnings,
			Audit:        requested.Storage.Audit && granted.Storage.Audit,
		},
		Discord: DiscordPermissions{
			Users:     requested.Discord.Users && granted.Discord.Users,
			Guilds:    requested.Discord.Guilds && granted.Discord.Guilds,
			Channels:  requested.Discord.Channels && granted.Discord.Channels,
			Messages:  requested.Discord.Messages && granted.Discord.Messages,
			Reactions: requested.Discord.Reactions && granted.Discord.Reactions,
			Members:   requested.Discord.Members && granted.Discord.Members,
			Roles:     requested.Discord.Roles && granted.Discord.Roles,
			Threads:   requested.Discord.Threads && granted.Discord.Threads,
			Invites:   requested.Discord.Invites && granted.Discord.Invites,
			Webhooks:  requested.Discord.Webhooks && granted.Discord.Webhooks,
			Emojis:    requested.Discord.Emojis && granted.Discord.Emojis,
			Stickers:  requested.Discord.Stickers && granted.Discord.Stickers,
		},
		Network:   NetworkPermissions{HTTP: requested.Network.HTTP && granted.Network.HTTP},
		Resources: ResourcePermissions{Read: requested.Resources.Read && granted.Resources.Read},
		Automation: AutomationPermissions{
			Jobs: requested.Automation.Jobs && granted.Automation.Jobs,
			Events: AutomationEventPermissions{
				MemberJoinLeave: requested.Automation.Events.MemberJoinLeave &&
					granted.Automation.Events.MemberJoinLeave,
				Moderation: requested.Automation.Events.Moderation && granted.Automation.Events.Moderation,
			},
		},
	}
}

func merge(base, override Permissions) Permissions {
	// For v1, simple boolean OR for "grants" is sufficient.
	out := base
	out.Storage.KV = out.Storage.KV || override.Storage.KV
	out.Storage.UserSettings = out.Storage.UserSettings || override.Storage.UserSettings
	out.Storage.CheckIns = out.Storage.CheckIns || override.Storage.CheckIns
	out.Storage.Reminders = out.Storage.Reminders || override.Storage.Reminders
	out.Storage.Warnings = out.Storage.Warnings || override.Storage.Warnings
	out.Storage.Audit = out.Storage.Audit || override.Storage.Audit
	out.Discord.Users = out.Discord.Users || override.Discord.Users
	out.Discord.Guilds = out.Discord.Guilds || override.Discord.Guilds
	out.Discord.Channels = out.Discord.Channels || override.Discord.Channels
	out.Discord.Messages = out.Discord.Messages || override.Discord.Messages
	out.Discord.Reactions = out.Discord.Reactions || override.Discord.Reactions
	out.Discord.Members = out.Discord.Members || override.Discord.Members
	out.Discord.Roles = out.Discord.Roles || override.Discord.Roles
	out.Discord.Threads = out.Discord.Threads || override.Discord.Threads
	out.Discord.Invites = out.Discord.Invites || override.Discord.Invites
	out.Discord.Webhooks = out.Discord.Webhooks || override.Discord.Webhooks
	out.Discord.Emojis = out.Discord.Emojis || override.Discord.Emojis
	out.Discord.Stickers = out.Discord.Stickers || override.Discord.Stickers
	out.Network.HTTP = out.Network.HTTP || override.Network.HTTP
	out.Resources.Read = out.Resources.Read || override.Resources.Read
	out.Automation.Jobs = out.Automation.Jobs || override.Automation.Jobs
	out.Automation.Events.MemberJoinLeave = out.Automation.Events.MemberJoinLeave ||
		override.Automation.Events.MemberJoinLeave
	out.Automation.Events.Moderation = out.Automation.Events.Moderation || override.Automation.Events.Moderation
	return out
}

func (p Permissions) Validate() error {
	return nil
}
