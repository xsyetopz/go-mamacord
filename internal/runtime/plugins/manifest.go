package pluginhost

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/permissions"
)

type Manifest struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Version string `json:"version"`

	Permissions permissions.Permissions `json:"permissions"`

	Commands []Command `json:"commands"`

	// Events declares which gateway events this plugin wants to receive.
	// The host still enforces permissions/allow-lists.
	Events []string `json:"events,omitempty"`

	// Jobs declares scheduled plugin triggers.
	Jobs []Job `json:"jobs,omitempty"`
}

const (
	CommandTypeSlash   = "slash"
	CommandTypeUser    = "user"
	CommandTypeMessage = "message"
)

type Command struct {
	Type        string `json:"type,omitempty"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// DescriptionID is an optional i18n key used for command description localization.
	DescriptionID            string   `json:"description_id,omitempty"`
	Ephemeral                bool     `json:"ephemeral"`
	DefaultMemberPermissions []string `json:"default_member_permissions,omitempty"`

	Options []CommandOption `json:"options"`

	Subcommands []Subcommand   `json:"subcommands,omitempty"`
	Groups      []CommandGroup `json:"groups,omitempty"`
}

type CommandOption struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	// DescriptionID is an optional i18n key used for option description localization.
	DescriptionID string `json:"description_id,omitempty"`
	Required      bool   `json:"required"`
	Autocomplete  string `json:"autocomplete,omitempty"`

	Choices []OptionChoice `json:"choices,omitempty"`

	MinValue *float64 `json:"min_value,omitempty"`
	MaxValue *float64 `json:"max_value,omitempty"`

	MinLength *int `json:"min_length,omitempty"`
	MaxLength *int `json:"max_length,omitempty"`

	ChannelTypes []int `json:"channel_types,omitempty"`
}

type Subcommand struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DescriptionID string `json:"description_id,omitempty"`

	Ephemeral *bool `json:"ephemeral,omitempty"`

	Options []CommandOption `json:"options"`
}

type CommandGroup struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DescriptionID string `json:"description_id,omitempty"`

	Subcommands []Subcommand `json:"subcommands"`
}

type OptionChoice struct {
	Name  string `json:"name"`
	Value any    `json:"value"`
}

type Job struct {
	ID       string `json:"id"`
	Schedule string `json:"schedule"`
}

func ReadManifest(path string) (Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	return ParseManifest(b)
}

func ParseManifest(b []byte) (Manifest, error) {
	var m Manifest
	if unmarshalErr := json.Unmarshal(b, &m); unmarshalErr != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", unmarshalErr)
	}

	if m.ID == "" {
		return Manifest{}, errors.New("manifest missing id")
	}
	for i := range m.Commands {
		m.Commands[i].Type = NormalizeCommandType(m.Commands[i].Type)
	}

	return m, nil
}

func NormalizeCommandType(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", CommandTypeSlash:
		return CommandTypeSlash
	case CommandTypeUser:
		return CommandTypeUser
	case CommandTypeMessage:
		return CommandTypeMessage
	default:
		return strings.ToLower(strings.TrimSpace(raw))
	}
}
