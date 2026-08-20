package projection

import (
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	"strings"
)

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
	DescriptionID            string             `json:"description_id,omitempty"`
	Ephemeral                bool               `json:"ephemeral"`
	Defer                    contract.DeferMode `json:"-"`
	DefaultMemberPermissions []string           `json:"default_member_permissions,omitempty"`

	Options []CommandOption `json:"options"`

	Subcommands []Subcommand   `json:"subcommands,omitempty"`
	Groups      []CommandGroup `json:"groups,omitempty"`
}

type CommandOption struct {
	OptionPresentation
	OptionBounds
	ChannelTypes []int `json:"channel_types,omitempty"`
}

type OptionPresentation struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	// DescriptionID is an optional i18n key used for option description localization.
	DescriptionID string         `json:"description_id,omitempty"`
	Required      bool           `json:"required"`
	Autocomplete  string         `json:"autocomplete,omitempty"`
	Choices       []OptionChoice `json:"choices,omitempty"`
}

type OptionBounds struct {
	MinValue  *float64 `json:"min_value,omitempty"`
	MaxValue  *float64 `json:"max_value,omitempty"`
	MinLength *int     `json:"min_length,omitempty"`
	MaxLength *int     `json:"max_length,omitempty"`
}

type Subcommand struct {
	Name          string `json:"name"`
	Description   string `json:"description"`
	DescriptionID string `json:"description_id,omitempty"`

	Ephemeral *bool              `json:"ephemeral,omitempty"`
	Defer     contract.DeferMode `json:"-"`

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
