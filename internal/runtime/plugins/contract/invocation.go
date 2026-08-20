// Package contract defines Mamacord-owned plugin declarations, invocations, and effects.
// It deliberately contains no scripting-engine, Discord-library, storage, or transport types.
package contract

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

type RouteID string
type GenerationID string

type CommandKind string

const (
	CommandSlash      CommandKind = "slash"
	CommandUser       CommandKind = "user"
	CommandMessage    CommandKind = "message"
	CommandGroup      CommandKind = "group"
	CommandSubcommand CommandKind = "subcommand"
)

type OptionKind string

const (
	OptionString      OptionKind = "string"
	OptionBoolean     OptionKind = "bool"
	OptionInteger     OptionKind = "int"
	OptionNumber      OptionKind = "float"
	OptionUser        OptionKind = "user"
	OptionChannel     OptionKind = "channel"
	OptionRole        OptionKind = "role"
	OptionMentionable OptionKind = "mentionable"
	OptionAttachment  OptionKind = "attachment"
)

type ResponseState string

const (
	ResponseNone           ResponseState = ""
	ResponseUnacknowledged ResponseState = "unacknowledged"
	ResponseDeferredCreate ResponseState = "deferred_create"
	ResponseDeferredUpdate ResponseState = "deferred_update"
	ResponseResponded      ResponseState = "responded"
)

type ModalOrigin string

const (
	ModalOriginNone      ModalOrigin = ""
	ModalOriginCommand   ModalOrigin = "command"
	ModalOriginComponent ModalOrigin = "component"
)

type ComponentKind string

const (
	ComponentButton            ComponentKind = "button"
	ComponentStringSelect      ComponentKind = "string_select"
	ComponentUserSelect        ComponentKind = "user_select"
	ComponentRoleSelect        ComponentKind = "role_select"
	ComponentMentionableSelect ComponentKind = "mentionable_select"
	ComponentChannelSelect     ComponentKind = "channel_select"
)

func validComponentKind(kind ComponentKind) bool {
	switch kind {
	case ComponentButton, ComponentStringSelect, ComponentUserSelect, ComponentRoleSelect, ComponentMentionableSelect, ComponentChannelSelect:
		return true
	default:
		return false
	}
}

type InvocationKind string

const (
	InvocationCommand      InvocationKind = "command"
	InvocationAutocomplete InvocationKind = "autocomplete"
	InvocationComponent    InvocationKind = "component"
	InvocationModal        InvocationKind = "modal"
	InvocationEvent        InvocationKind = "event"
	InvocationTask         InvocationKind = "task"
	InvocationCheck        InvocationKind = "check"
)

type UserRef struct {
	ID        string
	Username  string
	Name      string
	AvatarURL string
	Bot       bool
	System    bool
}

type RuntimeRef struct {
	Version        string
	Description    string
	Repository     string
	MascotImageURL string
}

type GuildRef struct {
	ID   string
	Name string
}

type ChannelRef struct {
	ID             string
	GuildID        string
	Name           string
	Kind           ChannelKind
	ParentID       string
	Mention        string
	PermissionBits uint64
	CreatedAt      int64
}

type RoleIdentity struct {
	ID      string
	GuildID string
	Name    string
}

type RoleAuthority struct {
	Position    int
	Permissions []MemberPermission
}

type RolePresentation struct {
	Mention     string
	Color       int
	Hoist       bool
	Mentionable bool
}

type RoleManagement struct {
	Managed        bool
	PermissionBits uint64
}

type RoleRef struct {
	RoleIdentity
	RoleAuthority
	RolePresentation
	RoleManagement
	CreatedAt int64
}

type MemberRef struct {
	GuildID     string
	User        UserRef
	DisplayName string
	RoleIDs     []string
	Permissions []MemberPermission
}

type MentionableKind string

const (
	MentionableUser MentionableKind = "user"
	MentionableRole MentionableKind = "role"
)

type MentionableRef struct {
	Kind MentionableKind
	User *UserRef
	Role *RoleRef
}

type AttachmentRef struct {
	ID          string
	Filename    string
	ContentType string
	URL         string
	Size        int64
	Width       int
	Height      int
}

type MessageRef struct {
	ID        string
	GuildID   string
	ChannelID string
	Author    UserRef
	Content   string
}

type ScalarOptionValue struct {
	String  string
	Boolean bool
	Integer int64
	Number  float64
}

type ReferenceOptionValue struct {
	User        *UserRef
	Channel     *ChannelRef
	Role        *RoleRef
	Mentionable *MentionableRef
	Attachment  *AttachmentRef
}

type OptionValue struct {
	Name string
	Kind OptionKind
	ScalarOptionValue
	ReferenceOptionValue
}

func (v OptionValue) Validate() error {
	if v.Name != strings.TrimSpace(v.Name) || !slashNamePattern.MatchString(v.Name) {
		return errors.New("option value name is not canonical")
	}
	if !validOptionKind(v.Kind) {
		return fmt.Errorf("unsupported option value kind %q", v.Kind)
	}
	if err := requireOnlyMatchingReference(v); err != nil {
		return err
	}

	switch v.Kind {
	case OptionString:
		if !utf8.ValidString(v.String) || utf8.RuneCountInString(v.String) > 6000 {
			return errors.New("string option must be valid UTF-8 and at most 6000 characters")
		}
		if v.Boolean || v.Integer != 0 || v.Number != 0 {
			return fmt.Errorf("string option %q contains inactive scalar payload", v.Name)
		}
	case OptionBoolean:
		if v.String != "" || v.Integer != 0 || v.Number != 0 {
			return fmt.Errorf("boolean option %q contains inactive scalar payload", v.Name)
		}
	case OptionInteger:
		if v.String != "" || v.Boolean || v.Number != 0 {
			return fmt.Errorf("integer option %q contains inactive scalar payload", v.Name)
		}
	case OptionNumber:
		if math.IsNaN(v.Number) || math.IsInf(v.Number, 0) {
			return errors.New("option number must be finite")
		}
		if v.String != "" || v.Boolean || v.Integer != 0 {
			return fmt.Errorf("number option %q contains inactive scalar payload", v.Name)
		}
	default:
		if v.String != "" || v.Boolean || v.Integer != 0 || v.Number != 0 {
			return fmt.Errorf("reference option %q contains inactive scalar payload", v.Name)
		}
	}
	return validateOptionReference(v)
}

func requireOnlyMatchingReference(v OptionValue) error {
	references := 0
	if v.User != nil {
		references++
	}
	if v.Channel != nil {
		references++
	}
	if v.Role != nil {
		references++
	}
	if v.Mentionable != nil {
		references++
	}
	if v.Attachment != nil {
		references++
	}
	expectsReference := v.Kind == OptionUser || v.Kind == OptionChannel || v.Kind == OptionRole || v.Kind == OptionMentionable || v.Kind == OptionAttachment
	if expectsReference && references != 1 {
		return fmt.Errorf("option %q must contain exactly one %s reference", v.Name, v.Kind)
	}
	if !expectsReference && references != 0 {
		return fmt.Errorf("scalar option %q cannot contain a reference", v.Name)
	}
	if v.Kind == OptionUser && v.User == nil || v.Kind == OptionChannel && v.Channel == nil || v.Kind == OptionRole && v.Role == nil || v.Kind == OptionMentionable && v.Mentionable == nil || v.Kind == OptionAttachment && v.Attachment == nil {
		return fmt.Errorf("option %q contains the wrong reference kind", v.Name)
	}
	return nil
}

func validateOptionReference(v OptionValue) error {
	switch v.Kind {
	case OptionUser:
		if strings.TrimSpace(v.User.ID) == "" {
			return fmt.Errorf("user option %q has no user id", v.Name)
		}
	case OptionChannel:
		if strings.TrimSpace(v.Channel.ID) == "" {
			return fmt.Errorf("channel option %q has no channel id", v.Name)
		}
		if !validChannelKind(v.Channel.Kind) {
			return fmt.Errorf("channel option %q has unsupported channel kind %q", v.Name, v.Channel.Kind)
		}
	case OptionRole:
		if err := validateRoleRef(*v.Role); err != nil {
			return fmt.Errorf("role option %q: %w", v.Name, err)
		}
	case OptionMentionable:
		if err := v.Mentionable.Validate(); err != nil {
			return fmt.Errorf("mentionable option %q: %w", v.Name, err)
		}
	case OptionAttachment:
		if strings.TrimSpace(v.Attachment.ID) == "" || strings.TrimSpace(v.Attachment.Filename) == "" {
			return fmt.Errorf("attachment option %q requires id and filename", v.Name)
		}
		if v.Attachment.Size < 0 {
			return fmt.Errorf("attachment option %q has negative size", v.Name)
		}
	}
	return nil
}

func (mentionable MentionableRef) Validate() error {
	switch mentionable.Kind {
	case MentionableUser:
		if mentionable.User == nil || mentionable.Role != nil || strings.TrimSpace(mentionable.User.ID) == "" {
			return errors.New("user mentionable requires only a user")
		}
	case MentionableRole:
		if mentionable.Role == nil || mentionable.User != nil {
			return errors.New("role mentionable requires only a role")
		}
		if err := validateRoleRef(*mentionable.Role); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported mentionable kind %q", mentionable.Kind)
	}
	return nil
}

func validateRoleRef(role RoleRef) error {
	if strings.TrimSpace(role.ID) == "" {
		return errors.New("role id is required")
	}
	seen := make(map[MemberPermission]struct{}, len(role.Permissions))
	for _, permission := range role.Permissions {
		if !validMemberPermission(permission) {
			return fmt.Errorf("unsupported role permission %q", permission)
		}
		if _, exists := seen[permission]; exists {
			return fmt.Errorf("duplicate role permission %q", permission)
		}
		seen[permission] = struct{}{}
	}
	return nil
}

type NamedString struct {
	Name  string
	Value string
}

type CommandInput struct {
	Kind          CommandKind
	Path          []string
	Options       []OptionValue
	TargetUser    *UserRef
	TargetMember  *MemberRef
	TargetMessage *MessageRef
}

type AutocompleteInput struct {
	Path    []string
	Option  string
	Focused OptionValue
	Options []OptionValue
}

type ComponentInput struct {
	ID     string
	Kind   ComponentKind
	Values []OptionValue
}

type ModalInput struct {
	ID     string
	Fields []NamedString
}

type EventInput struct {
	Name string
	Data Value
}

type TaskInput struct {
	ID string
}
type CheckInput struct {
	ID string
}

type StateEntry struct {
	Key     string
	Value   Value
	Version uint64
}

type InvocationIdentity struct {
	PluginID   string
	Generation GenerationID
	Route      RouteID
	Kind       InvocationKind
}

type InvocationActorContext struct {
	Guild   *GuildRef
	Channel *ChannelRef
	Author  *UserRef
	BotUser *UserRef
	Member  *MemberRef
	Locale  string
}

type InvocationExecutionContext struct {
	NowUnix    int64
	RandomSeed uint64
	Runtime    RuntimeRef
	State      []StateEntry
	IsOwner    bool
}

type InvocationInteractionContext struct {
	ResponseState ResponseState
	ModalOrigin   ModalOrigin
}

type InvocationInput struct {
	Command      *CommandInput
	Autocomplete *AutocompleteInput
	Component    *ComponentInput
	Modal        *ModalInput
	Event        *EventInput
	Task         *TaskInput
	Check        *CheckInput
}

type Invocation struct {
	InvocationIdentity
	InvocationActorContext
	InvocationExecutionContext
	InvocationInteractionContext
	InvocationInput
}

func (i Invocation) DeepClone() Invocation {
	out := i
	if i.State != nil {
		out.State = make([]StateEntry, len(i.State))
		for index, entry := range i.State {
			out.State[index] = StateEntry{Key: entry.Key, Value: entry.Value.Clone(), Version: entry.Version}
		}
	}
	if i.Guild != nil {
		value := *i.Guild
		out.Guild = &value
	}
	if i.Channel != nil {
		value := *i.Channel
		out.Channel = &value
	}
	if i.Author != nil {
		value := *i.Author
		out.Author = &value
	}
	if i.BotUser != nil {
		value := *i.BotUser
		out.BotUser = &value
	}
	if i.Member != nil {
		value := *i.Member
		value.RoleIDs = append([]string(nil), i.Member.RoleIDs...)
		value.Permissions = append([]MemberPermission(nil), i.Member.Permissions...)
		out.Member = &value
	}
	if i.Command != nil {
		value := *i.Command
		value.Path = append([]string(nil), i.Command.Path...)
		value.Options = cloneOptionValues(i.Command.Options)
		if i.Command.TargetUser != nil {
			user := *i.Command.TargetUser
			value.TargetUser = &user
		}
		if i.Command.TargetMember != nil {
			member := *i.Command.TargetMember
			member.RoleIDs = append([]string(nil), i.Command.TargetMember.RoleIDs...)
			member.Permissions = append([]MemberPermission(nil), i.Command.TargetMember.Permissions...)
			value.TargetMember = &member
		}
		if i.Command.TargetMessage != nil {
			message := *i.Command.TargetMessage
			value.TargetMessage = &message
		}
		out.Command = &value
	}
	if i.Autocomplete != nil {
		value := *i.Autocomplete
		value.Path = append([]string(nil), i.Autocomplete.Path...)
		value.Focused = cloneOptionValue(i.Autocomplete.Focused)
		value.Options = cloneOptionValues(i.Autocomplete.Options)
		out.Autocomplete = &value
	}
	if i.Component != nil {
		value := *i.Component
		value.Values = cloneOptionValues(i.Component.Values)
		out.Component = &value
	}
	if i.Modal != nil {
		value := *i.Modal
		value.Fields = append([]NamedString(nil), i.Modal.Fields...)
		out.Modal = &value
	}
	if i.Event != nil {
		value := *i.Event
		value.Data = i.Event.Data.Clone()
		out.Event = &value
	}
	if i.Task != nil {
		value := *i.Task
		out.Task = &value
	}
	if i.Check != nil {
		value := *i.Check
		out.Check = &value
	}
	return out
}

func cloneOptionValues(values []OptionValue) []OptionValue {
	if len(values) == 0 {
		return nil
	}
	out := make([]OptionValue, len(values))
	for index, value := range values {
		out[index] = cloneOptionValue(value)
	}
	return out
}

func cloneOptionValue(value OptionValue) OptionValue {
	out := value
	if value.User != nil {
		ref := *value.User
		out.User = &ref
	}
	if value.Channel != nil {
		ref := *value.Channel
		out.Channel = &ref
	}
	if value.Role != nil {
		ref := *value.Role
		ref.Permissions = append([]MemberPermission(nil), value.Role.Permissions...)
		out.Role = &ref
	}
	if value.Mentionable != nil {
		ref := *value.Mentionable
		if value.Mentionable.User != nil {
			user := *value.Mentionable.User
			ref.User = &user
		}
		if value.Mentionable.Role != nil {
			role := *value.Mentionable.Role
			role.Permissions = append([]MemberPermission(nil), value.Mentionable.Role.Permissions...)
			ref.Role = &role
		}
		out.Mentionable = &ref
	}
	if value.Attachment != nil {
		ref := *value.Attachment
		out.Attachment = &ref
	}
	return out
}

func (i Invocation) Validate() error {
	if strings.TrimSpace(i.PluginID) == "" {
		return errors.New("plugin id is required")
	}
	if strings.TrimSpace(string(i.Generation)) == "" {
		return errors.New("generation id is required")
	}
	route := string(i.Route)
	if route != strings.TrimSpace(route) || !routeIDPattern.MatchString(route) {
		return errors.New("route id is not canonical")
	}
	if err := i.validateContext(); err != nil {
		return err
	}
	if err := i.validateState(); err != nil {
		return err
	}

	inputs := 0
	if i.Command != nil {
		inputs++
	}
	if i.Autocomplete != nil {
		inputs++
	}
	if i.Component != nil {
		inputs++
	}
	if i.Modal != nil {
		inputs++
	}
	if i.Event != nil {
		inputs++
	}
	if i.Task != nil {
		inputs++
	}
	if i.Check != nil {
		inputs++
	}
	if inputs != 1 {
		return errors.New("invocation must contain exactly one route input")
	}
	matches := i.Kind == InvocationCommand && i.Command != nil || i.Kind == InvocationAutocomplete && i.Autocomplete != nil || i.Kind == InvocationComponent && i.Component != nil || i.Kind == InvocationModal && i.Modal != nil || i.Kind == InvocationEvent && i.Event != nil || i.Kind == InvocationTask && i.Task != nil || i.Kind == InvocationCheck && i.Check != nil
	if !matches {
		return fmt.Errorf("invocation kind %q does not match its route input", i.Kind)
	}

	interaction := i.Kind == InvocationCommand || i.Kind == InvocationAutocomplete || i.Kind == InvocationComponent || i.Kind == InvocationModal
	if interaction {
		switch i.ResponseState {
		case ResponseUnacknowledged, ResponseDeferredCreate, ResponseDeferredUpdate, ResponseResponded:
		default:
			return fmt.Errorf("interaction has invalid response state %q", i.ResponseState)
		}
	} else if i.ResponseState != ResponseNone {
		return errors.New("automation invocation cannot have interaction response state")
	}
	switch i.Kind {
	case InvocationCommand:
		if i.ResponseState == ResponseDeferredUpdate {
			return errors.New("command cannot have deferred-update state")
		}
	case InvocationAutocomplete:
		if i.ResponseState != ResponseUnacknowledged {
			return errors.New("autocomplete must be unacknowledged")
		}
	case InvocationModal:
		if i.ResponseState == ResponseDeferredUpdate && i.ModalOrigin != ModalOriginComponent {
			return errors.New("command-origin modal cannot have deferred-update state")
		}
	}
	if i.Kind == InvocationModal {
		if i.ModalOrigin != ModalOriginCommand && i.ModalOrigin != ModalOriginComponent {
			return errors.New("modal invocation requires command or component origin")
		}
	} else if i.ModalOrigin != ModalOriginNone {
		return errors.New("modal origin is only valid for modal invocations")
	}

	if i.Command != nil {
		if err := validatePath(i.Command.Path); err != nil {
			return fmt.Errorf("command: %w", err)
		}
		switch i.Command.Kind {
		case CommandSlash:
			if i.Command.TargetUser != nil || i.Command.TargetMember != nil || i.Command.TargetMessage != nil {
				return errors.New("slash command cannot contain a context-command target")
			}
			if err := validateOptionValues(i.Command.Options); err != nil {
				return err
			}
		case CommandUser:
			if i.Command.TargetUser == nil || strings.TrimSpace(i.Command.TargetUser.ID) == "" {
				return errors.New("user command requires target user")
			}
			if i.Command.TargetMessage != nil || len(i.Command.Options) != 0 {
				return errors.New("user command cannot contain message target or options")
			}
			if i.Command.TargetMember != nil && i.Command.TargetMember.User.ID != i.Command.TargetUser.ID {
				return errors.New("target member does not match target user")
			}
		case CommandMessage:
			if i.Command.TargetMessage == nil || strings.TrimSpace(i.Command.TargetMessage.ID) == "" {
				return errors.New("message command requires target message")
			}
			if i.Command.TargetUser != nil || i.Command.TargetMember != nil || len(i.Command.Options) != 0 {
				return errors.New("message command cannot contain user target or options")
			}
		default:
			return fmt.Errorf("invocation has unsupported command kind %q", i.Command.Kind)
		}
	}
	if i.Autocomplete != nil {
		if err := validatePath(i.Autocomplete.Path); err != nil {
			return fmt.Errorf("autocomplete: %w", err)
		}
		if strings.TrimSpace(i.Autocomplete.Option) == "" {
			return errors.New("autocomplete option is required")
		}
		if err := i.Autocomplete.Focused.Validate(); err != nil {
			return fmt.Errorf("focused option: %w", err)
		}
		if i.Autocomplete.Focused.Name != i.Autocomplete.Option {
			return errors.New("focused option name does not match autocomplete option")
		}
		if err := validateOptionValues(i.Autocomplete.Options); err != nil {
			return err
		}
	}
	if i.Component != nil {
		if !handlerIDPattern.MatchString(i.Component.ID) {
			return errors.New("component id is not canonical")
		}
		if err := validateComponentInput(*i.Component); err != nil {
			return err
		}
	}
	if i.Modal != nil {
		if !handlerIDPattern.MatchString(i.Modal.ID) {
			return errors.New("modal id is not canonical")
		}
		if len(i.Modal.Fields) == 0 || len(i.Modal.Fields) > 5 {
			return errors.New("modal input must contain 1 to 5 fields")
		}
		seen := make(map[string]struct{}, len(i.Modal.Fields))
		for _, field := range i.Modal.Fields {
			if !handlerIDPattern.MatchString(field.Name) {
				return errors.New("modal field name is not canonical")
			}
			if !utf8.ValidString(field.Value) || utf8.RuneCountInString(field.Value) > 4000 {
				return fmt.Errorf("modal field %q value must be valid UTF-8 and at most 4000 characters", field.Name)
			}
			if _, exists := seen[field.Name]; exists {
				return fmt.Errorf("duplicate modal field %q", field.Name)
			}
			seen[field.Name] = struct{}{}
		}
	}
	if i.Event != nil {
		if strings.TrimSpace(i.Event.Name) == "" {
			return errors.New("event name is required")
		}
		size, err := i.Event.Data.EncodedSize()
		if err != nil {
			return fmt.Errorf("event data: %w", err)
		}
		if size > MaxInvocationValueBytes {
			return fmt.Errorf("event data exceeds %d bytes", MaxInvocationValueBytes)
		}
	}
	if i.Task != nil && strings.TrimSpace(i.Task.ID) == "" {
		return errors.New("task id is required")
	}
	return nil
}

func (i Invocation) validateState() error {
	if len(i.State) == 0 {
		return nil
	}
	if i.Guild == nil {
		return errors.New("state input requires guild context")
	}
	if len(i.State) > MaxValueItems {
		return fmt.Errorf("state input exceeds %d entries", MaxValueItems)
	}
	seen := make(map[string]struct{}, len(i.State))
	total := 0
	for _, entry := range i.State {
		if !kvKeyPattern.MatchString(entry.Key) {
			return fmt.Errorf("state key %q is not canonical", entry.Key)
		}
		if _, exists := seen[entry.Key]; exists {
			return fmt.Errorf("duplicate state key %q", entry.Key)
		}
		seen[entry.Key] = struct{}{}
		size, err := entry.Value.EncodedSize()
		if err != nil {
			return fmt.Errorf("state key %q: %w", entry.Key, err)
		}
		if size > MaxStateValueBytes {
			return fmt.Errorf("state key %q exceeds %d bytes", entry.Key, MaxStateValueBytes)
		}
		total += size
		if total > MaxInvocationValueBytes {
			return fmt.Errorf("state input exceeds %d aggregate bytes", MaxInvocationValueBytes)
		}
	}
	return nil
}

func (i Invocation) validateContext() error {
	if i.Guild != nil && strings.TrimSpace(i.Guild.ID) == "" {
		return errors.New("guild id is required")
	}
	if i.Channel != nil {
		if strings.TrimSpace(i.Channel.ID) == "" {
			return errors.New("channel id is required")
		}
		if !validChannelKind(i.Channel.Kind) {
			return fmt.Errorf("unsupported channel kind %q", i.Channel.Kind)
		}
		if i.Guild != nil && i.Channel.GuildID != "" && i.Channel.GuildID != i.Guild.ID {
			return errors.New("channel guild does not match invocation guild")
		}
	}
	if i.Author != nil && strings.TrimSpace(i.Author.ID) == "" {
		return errors.New("author id is required")
	}
	if i.Member != nil {
		if i.Guild == nil {
			return errors.New("member context requires guild")
		}
		if i.Member.GuildID != i.Guild.ID {
			return errors.New("member guild does not match invocation guild")
		}
		if i.Author == nil || i.Member.User.ID != i.Author.ID {
			return errors.New("member user does not match invocation author")
		}
		seenRoles := make(map[string]struct{}, len(i.Member.RoleIDs))
		for _, roleID := range i.Member.RoleIDs {
			if strings.TrimSpace(roleID) == "" {
				return errors.New("member role id is required")
			}
			if _, exists := seenRoles[roleID]; exists {
				return fmt.Errorf("member repeats role %q", roleID)
			}
			seenRoles[roleID] = struct{}{}
		}
		seenPermissions := make(map[MemberPermission]struct{}, len(i.Member.Permissions))
		for _, permission := range i.Member.Permissions {
			if !validMemberPermission(permission) {
				return fmt.Errorf("member has unsupported permission %q", permission)
			}
			if _, exists := seenPermissions[permission]; exists {
				return fmt.Errorf("member repeats permission %q", permission)
			}
			seenPermissions[permission] = struct{}{}
		}
	}
	if i.Kind == InvocationCommand || i.Kind == InvocationAutocomplete || i.Kind == InvocationComponent || i.Kind == InvocationModal || i.Kind == InvocationCheck {
		if i.Author == nil {
			return errors.New("interactive invocation requires author")
		}
	}
	return nil
}

func validateComponentInput(input ComponentInput) error {
	if len(input.Values) > 25 {
		return errors.New("component input exceeds 25 selected values")
	}
	if input.Kind == ComponentButton {
		if len(input.Values) != 0 {
			return errors.New("button input cannot contain selected values")
		}
		return nil
	}
	var expected OptionKind
	switch input.Kind {
	case ComponentStringSelect:
		expected = OptionString
	case ComponentUserSelect:
		expected = OptionUser
	case ComponentRoleSelect:
		expected = OptionRole
	case ComponentMentionableSelect:
		expected = OptionMentionable
	case ComponentChannelSelect:
		expected = OptionChannel
	default:
		return fmt.Errorf("unsupported component kind %q", input.Kind)
	}
	if err := validateOptionValues(input.Values); err != nil {
		return fmt.Errorf("component values: %w", err)
	}
	for _, value := range input.Values {
		if value.Kind != expected {
			return fmt.Errorf("component value %q has kind %q, want %q", value.Name, value.Kind, expected)
		}
	}
	return nil
}

func validatePath(parts []string) error {
	if len(parts) == 0 || len(parts) > 3 {
		return errors.New("command path must contain one to three names")
	}
	for _, part := range parts {
		if strings.TrimSpace(part) == "" {
			return errors.New("command path contains an empty name")
		}
	}
	return nil
}

func validateOptionValues(values []OptionValue) error {
	if len(values) > 25 {
		return errors.New("invocation exceeds 25 option values")
	}
	seen := make(map[string]struct{}, len(values))
	for index, value := range values {
		if err := value.Validate(); err != nil {
			return fmt.Errorf("option %d: %w", index+1, err)
		}
		key := value.Name
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate option value %q", value.Name)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func validOptionKind(kind OptionKind) bool {
	switch kind {
	case OptionString, OptionBoolean, OptionInteger, OptionNumber, OptionUser, OptionChannel,
		OptionRole, OptionMentionable, OptionAttachment:
		return true
	default:
		return false
	}
}
