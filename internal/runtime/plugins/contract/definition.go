package contract

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

type MemberPermission string

const (
	PermissionAdministrator     MemberPermission = "administrator"
	PermissionManageGuild       MemberPermission = "manage_guild"
	PermissionManageRoles       MemberPermission = "manage_roles"
	PermissionManageExpressions MemberPermission = "manage_expressions"
	PermissionCreateExpressions MemberPermission = "create_expressions"
	PermissionManageMessages    MemberPermission = "manage_messages"
	PermissionManageNicknames   MemberPermission = "manage_nicknames"
	PermissionManageChannels    MemberPermission = "manage_channels"
	PermissionKickMembers       MemberPermission = "kick_members"
	PermissionBanMembers        MemberPermission = "ban_members"
	PermissionModerateMembers   MemberPermission = "moderate_members"
)

func validMemberPermission(permission MemberPermission) bool {
	switch permission {
	case PermissionAdministrator, PermissionManageGuild, PermissionManageRoles,
		PermissionManageExpressions, PermissionCreateExpressions, PermissionManageMessages,
		PermissionManageNicknames, PermissionManageChannels, PermissionKickMembers,
		PermissionBanMembers, PermissionModerateMembers:
		return true
	default:
		return false
	}
}

type ChannelKind string

const (
	ChannelText         ChannelKind = "text"
	ChannelVoice        ChannelKind = "voice"
	ChannelCategory     ChannelKind = "category"
	ChannelAnnouncement ChannelKind = "announcement"
	ChannelStage        ChannelKind = "stage"
	ChannelForum        ChannelKind = "forum"
	ChannelMedia        ChannelKind = "media"
)

func validChannelKind(kind ChannelKind) bool {
	switch kind {
	case ChannelText, ChannelVoice, ChannelCategory, ChannelAnnouncement, ChannelStage, ChannelForum, ChannelMedia:
		return true
	default:
		return false
	}
}

type DeferMode string

const (
	DeferNone   DeferMode = ""
	DeferCreate DeferMode = "create"
	DeferUpdate DeferMode = "update"
)

func validDeferMode(mode DeferMode) bool {
	return mode == DeferNone || mode == DeferCreate || mode == DeferUpdate
}

type Definition struct {
	Cogs []CogDefinition
}

type CogDefinition struct {
	Name        string
	Description string
	Checks      []CheckDefinition
	Commands    []CommandDefinition
	Listeners   []ListenerDefinition
	Tasks       []TaskDefinition
	Components  []ComponentDefinition
	Modals      []ModalDefinition
}

type CommandDescription struct {
	Description   string
	DescriptionID string
}

type CommandResponse struct {
	Ephemeral bool
	Defer     DeferMode
}

type CommandDefinition struct {
	Kind  CommandKind
	Route RouteID
	Name  string
	CommandDescription
	CommandResponse
	DefaultMemberPermissions []MemberPermission
	Options                  []OptionDefinition
	Children                 []CommandDefinition
	Checks                   []CheckDefinition
}

type OptionDescription struct {
	Description   string
	DescriptionID string
}

type OptionSelection struct {
	Choices      []ChoiceDefinition
	Autocomplete RouteID
}

type IntegerOptionBounds struct {
	MinInteger *int64
	MaxInteger *int64
}

type NumberOptionBounds struct {
	MinNumber *float64
	MaxNumber *float64
}

type StringOptionBounds struct {
	MinLength *int
	MaxLength *int
}

type OptionDefinition struct {
	Name string
	Kind OptionKind
	OptionDescription
	Required bool
	OptionSelection
	IntegerOptionBounds
	NumberOptionBounds
	StringOptionBounds
	ChannelKinds []ChannelKind
}

type ChoiceKind string

const (
	ChoiceString  ChoiceKind = "string"
	ChoiceInteger ChoiceKind = "int"
	ChoiceNumber  ChoiceKind = "float"
)

type ChoiceValue struct {
	Kind    ChoiceKind
	String  string
	Integer int64
	Number  float64
}

type ChoiceDefinition struct {
	Name  string
	Value ChoiceValue
}

type CheckKind string

const (
	CheckGuildOnly      CheckKind = "guild_only"
	CheckOwnerOnly      CheckKind = "owner_only"
	CheckHasPermissions CheckKind = "has_permissions"
	CheckCustom         CheckKind = "custom"
)

type CheckDefinition struct {
	Kind        CheckKind
	Permissions []MemberPermission
	Route       RouteID
}

type ListenerDefinition struct {
	ID     string
	Event  string
	Route  RouteID
	Checks []CheckDefinition
}

type TaskDefinition struct {
	ID       string
	Schedule string
	Route    RouteID
	Checks   []CheckDefinition
}

type ComponentDefinition struct {
	ID     string
	Route  RouteID
	Defer  DeferMode
	Kinds  []ComponentKind
	Checks []CheckDefinition
}

type ModalFieldDefinition struct {
	ID       string
	Required bool
}

type ModalDefinition struct {
	ID     string
	Route  RouteID
	Defer  DeferMode
	Fields []ModalFieldDefinition
	Checks []CheckDefinition
}

func (d Definition) DeepClone() Definition {
	out := Definition{Cogs: make([]CogDefinition, len(d.Cogs))}
	for index := range d.Cogs {
		out.Cogs[index] = cloneCog(d.Cogs[index])
	}
	return out
}

func cloneCog(cog CogDefinition) CogDefinition {
	out := cog
	out.Checks = cloneChecks(cog.Checks)
	out.Commands = cloneCommands(cog.Commands)
	out.Listeners = cloneListeners(cog.Listeners)
	out.Tasks = cloneTasks(cog.Tasks)
	out.Components = cloneComponents(cog.Components)
	out.Modals = cloneModals(cog.Modals)
	return out
}

func cloneCommands(commands []CommandDefinition) []CommandDefinition {
	if len(commands) == 0 {
		return nil
	}
	out := make([]CommandDefinition, len(commands))
	for index, command := range commands {
		out[index] = command
		out[index].DefaultMemberPermissions = append([]MemberPermission(nil), command.DefaultMemberPermissions...)
		out[index].Options = cloneOptions(command.Options)
		out[index].Children = cloneCommands(command.Children)
		out[index].Checks = cloneChecks(command.Checks)
	}
	return out
}

func cloneOptions(options []OptionDefinition) []OptionDefinition {
	if len(options) == 0 {
		return nil
	}
	out := make([]OptionDefinition, len(options))
	for index, option := range options {
		out[index] = option
		out[index].Choices = append([]ChoiceDefinition(nil), option.Choices...)
		out[index].ChannelKinds = append([]ChannelKind(nil), option.ChannelKinds...)
		if option.MinInteger != nil {
			value := *option.MinInteger
			out[index].MinInteger = &value
		}
		if option.MaxInteger != nil {
			value := *option.MaxInteger
			out[index].MaxInteger = &value
		}
		if option.MinNumber != nil {
			value := *option.MinNumber
			out[index].MinNumber = &value
		}
		if option.MaxNumber != nil {
			value := *option.MaxNumber
			out[index].MaxNumber = &value
		}
		if option.MinLength != nil {
			value := *option.MinLength
			out[index].MinLength = &value
		}
		if option.MaxLength != nil {
			value := *option.MaxLength
			out[index].MaxLength = &value
		}
	}
	return out
}

func cloneChecks(checks []CheckDefinition) []CheckDefinition {
	if len(checks) == 0 {
		return nil
	}
	out := make([]CheckDefinition, len(checks))
	for index, check := range checks {
		out[index] = check
		out[index].Permissions = append([]MemberPermission(nil), check.Permissions...)
	}
	return out
}

func cloneListeners(values []ListenerDefinition) []ListenerDefinition {
	if len(values) == 0 {
		return nil
	}
	out := make([]ListenerDefinition, len(values))
	for index, value := range values {
		out[index] = value
		out[index].Checks = cloneChecks(value.Checks)
	}
	return out
}
func cloneTasks(values []TaskDefinition) []TaskDefinition {
	if len(values) == 0 {
		return nil
	}
	out := make([]TaskDefinition, len(values))
	for index, value := range values {
		out[index] = value
		out[index].Checks = cloneChecks(value.Checks)
	}
	return out
}
func cloneComponents(values []ComponentDefinition) []ComponentDefinition {
	if len(values) == 0 {
		return nil
	}
	out := make([]ComponentDefinition, len(values))
	for index, value := range values {
		out[index] = value
		out[index].Kinds = append([]ComponentKind(nil), value.Kinds...)
		out[index].Checks = cloneChecks(value.Checks)
	}
	return out
}
func cloneModals(values []ModalDefinition) []ModalDefinition {
	if len(values) == 0 {
		return nil
	}
	out := make([]ModalDefinition, len(values))
	for index, value := range values {
		out[index] = value
		out[index].Fields = append([]ModalFieldDefinition(nil), value.Fields...)
		out[index].Checks = cloneChecks(value.Checks)
	}
	return out
}

var (
	slashNamePattern = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
	routeIDPattern   = regexp.MustCompile(`^[a-z][a-z0-9]*(?::[a-z0-9][a-z0-9_.-]*)+$`)
	handlerIDPattern = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
	kvKeyPattern     = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/-]{0,127}$`)
)

func (d Definition) Validate() error {
	if len(d.Cogs) == 0 {
		return errors.New("definition must contain at least one cog")
	}
	cogNames := make(map[string]struct{}, len(d.Cogs))
	routes := make(map[RouteID]string)
	commandPaths := make(map[string]struct{})
	commandCounts := make(map[CommandKind]int)
	components := make(map[string]struct{})
	modals := make(map[string]struct{})
	listeners := make(map[string]struct{})
	tasks := make(map[string]struct{})

	for cogIndex, cog := range d.Cogs {
		name := strings.TrimSpace(cog.Name)
		if name == "" {
			return fmt.Errorf("cog %d: name is required", cogIndex+1)
		}
		if utf8.RuneCountInString(name) > 100 {
			return fmt.Errorf("cog %q: name exceeds 100 characters", name)
		}
		nameKey := strings.ToLower(name)
		if _, exists := cogNames[nameKey]; exists {
			return fmt.Errorf("duplicate cog name %q", name)
		}
		cogNames[nameKey] = struct{}{}
		if err := validateChecks(cog.Checks, routes, "cog "+name); err != nil {
			return err
		}

		for _, command := range cog.Commands {
			commandCounts[command.Kind]++
			limit := 100
			if command.Kind == CommandUser || command.Kind == CommandMessage {
				limit = 5
			}
			if commandCounts[command.Kind] > limit {
				return fmt.Errorf("definition exceeds %d top-level %s commands", limit, command.Kind)
			}
			if command.Kind != CommandSlash && command.Kind != CommandUser && command.Kind != CommandMessage {
				return fmt.Errorf("cog %q: top-level command %q has illegal kind %q", name, command.Name, command.Kind)
			}
			if err := validateCommand(command, command.Kind, nil, routes, commandPaths); err != nil {
				return fmt.Errorf("cog %q: %w", name, err)
			}
		}
		for _, listener := range cog.Listeners {
			if err := validateNamedRoute("listener", listener.ID, listener.Route, listeners, routes); err != nil {
				return fmt.Errorf("cog %q: %w", name, err)
			}
			if strings.TrimSpace(listener.Event) == "" {
				return fmt.Errorf("cog %q: listener %q event is required", name, listener.ID)
			}
			if err := validateChecks(listener.Checks, routes, "listener "+listener.ID); err != nil {
				return err
			}
		}
		for _, task := range cog.Tasks {
			if err := validateNamedRoute("task", task.ID, task.Route, tasks, routes); err != nil {
				return fmt.Errorf("cog %q: %w", name, err)
			}
			if strings.TrimSpace(task.Schedule) == "" {
				return fmt.Errorf("cog %q: task %q schedule is required", name, task.ID)
			}
			if err := validateChecks(task.Checks, routes, "task "+task.ID); err != nil {
				return err
			}
		}
		for _, component := range cog.Components {
			if len(component.Kinds) == 0 {
				return fmt.Errorf("cog %q: component %q requires at least one kind", name, component.ID)
			}
			seenKinds := make(map[ComponentKind]struct{}, len(component.Kinds))
			for _, kind := range component.Kinds {
				if !validComponentKind(kind) {
					return fmt.Errorf("cog %q: component %q has invalid kind %q", name, component.ID, kind)
				}
				if _, exists := seenKinds[kind]; exists {
					return fmt.Errorf("cog %q: component %q repeats kind %q", name, component.ID, kind)
				}
				seenKinds[kind] = struct{}{}
			}
			if !validDeferMode(component.Defer) {
				return fmt.Errorf("cog %q: component %q has invalid defer mode %q", name, component.ID, component.Defer)
			}
			if err := validateNamedRoute("component", component.ID, component.Route, components, routes); err != nil {
				return fmt.Errorf("cog %q: %w", name, err)
			}
			if err := validateChecks(component.Checks, routes, "component "+component.ID); err != nil {
				return err
			}
		}
		for _, modal := range cog.Modals {
			if len(modal.Fields) == 0 || len(modal.Fields) > 5 {
				return fmt.Errorf("cog %q: modal %q requires 1 to 5 fields", name, modal.ID)
			}
			seenFields := make(map[string]struct{}, len(modal.Fields))
			for _, field := range modal.Fields {
				if !handlerIDPattern.MatchString(field.ID) {
					return fmt.Errorf("cog %q: modal %q has invalid field %q", name, modal.ID, field.ID)
				}
				if _, exists := seenFields[field.ID]; exists {
					return fmt.Errorf("cog %q: modal %q repeats field %q", name, modal.ID, field.ID)
				}
				seenFields[field.ID] = struct{}{}
			}
			if !validDeferMode(modal.Defer) {
				return fmt.Errorf("cog %q: modal %q has invalid defer mode %q", name, modal.ID, modal.Defer)
			}
			if err := validateNamedRoute("modal", modal.ID, modal.Route, modals, routes); err != nil {
				return fmt.Errorf("cog %q: %w", name, err)
			}
			if err := validateChecks(modal.Checks, routes, "modal "+modal.ID); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateCommand(command CommandDefinition, namespace CommandKind, parent []string, routes map[RouteID]string, paths map[string]struct{}) error {
	name := strings.TrimSpace(command.Name)
	if command.Name != name {
		return fmt.Errorf("command name %q has surrounding whitespace", command.Name)
	}
	if !slashNamePattern.MatchString(name) && command.Kind != CommandUser && command.Kind != CommandMessage {
		return fmt.Errorf("command name %q must match %s", name, slashNamePattern.String())
	}
	if (command.Kind == CommandUser || command.Kind == CommandMessage) && (name == "" || utf8.RuneCountInString(name) > 32) {
		return fmt.Errorf("context command name %q must contain 1 to 32 characters", name)
	}
	if command.Kind != CommandUser && command.Kind != CommandMessage {
		if strings.TrimSpace(command.Description) == "" {
			return fmt.Errorf("command %q requires a base description", name)
		}
		if utf8.RuneCountInString(command.Description) > 100 {
			return fmt.Errorf("command %q description exceeds 100 characters", name)
		}
	}
	if (command.Kind == CommandUser || command.Kind == CommandMessage) && (command.Description != "" || command.DescriptionID != "") {
		return fmt.Errorf("context command %q cannot define a description", name)
	}
	if len(parent) != 0 && len(command.DefaultMemberPermissions) != 0 {
		return fmt.Errorf("child command %q cannot define default member permissions", name)
	}
	if len(command.DefaultMemberPermissions) > 0 {
		seenPermissions := make(map[MemberPermission]struct{}, len(command.DefaultMemberPermissions))
		for _, permission := range command.DefaultMemberPermissions {
			permission = MemberPermission(strings.TrimSpace(string(permission)))
			if permission == "" {
				return fmt.Errorf("command %q has an empty default permission", name)
			}
			if !validMemberPermission(permission) {
				return fmt.Errorf("command %q has unsupported default permission %q", name, permission)
			}
			if _, exists := seenPermissions[permission]; exists {
				return fmt.Errorf("command %q repeats default permission %q", name, permission)
			}
			seenPermissions[permission] = struct{}{}
		}
	}

	pathParts := append(append([]string(nil), parent...), strings.ToLower(name))
	displayPath := strings.Join(pathParts, " ")
	pathKey := string(namespace) + ":" + displayPath
	if _, exists := paths[pathKey]; exists {
		return fmt.Errorf("duplicate command path %q", displayPath)
	}
	paths[pathKey] = struct{}{}

	container := len(command.Children) > 0 || command.Kind == CommandGroup
	if container {
		if command.Route != "" {
			return fmt.Errorf("command container %q cannot have a route", displayPath)
		}
		if len(command.Options) > 0 {
			return fmt.Errorf("command container %q cannot have options", displayPath)
		}
		if command.Ephemeral || command.Defer != DeferNone {
			return fmt.Errorf("command container %q cannot define leaf response metadata", displayPath)
		}
	} else if err := addRoute(routes, command.Route, "command "+displayPath); err != nil {
		return err
	}
	if !container && (!validDeferMode(command.Defer) || command.Defer == DeferUpdate) {
		return fmt.Errorf("command %q has invalid defer mode %q", displayPath, command.Defer)
	}
	if len(command.Children) > 25 {
		return fmt.Errorf("command %q exceeds 25 children", displayPath)
	}
	if command.Kind == CommandUser || command.Kind == CommandMessage {
		if len(command.Children) > 0 || len(command.Options) > 0 {
			return fmt.Errorf("context command %q cannot have children or options", displayPath)
		}
	}
	if command.Kind == CommandGroup && len(command.Children) == 0 {
		return fmt.Errorf("group %q must contain subcommands", displayPath)
	}
	if command.Kind == CommandSubcommand && len(command.Children) > 0 {
		return fmt.Errorf("subcommand %q cannot contain children", displayPath)
	}
	if err := validateOptions(command.Options, routes, "command "+displayPath); err != nil {
		return err
	}
	if err := validateChecks(command.Checks, routes, "command "+displayPath); err != nil {
		return err
	}

	for _, child := range command.Children {
		switch command.Kind {
		case CommandSlash:
			if child.Kind != CommandSubcommand && child.Kind != CommandGroup {
				return fmt.Errorf("slash command %q may contain only subcommands or groups", displayPath)
			}
		case CommandGroup:
			if child.Kind != CommandSubcommand {
				return fmt.Errorf("group %q may contain only subcommands", displayPath)
			}
		default:
			return fmt.Errorf("command %q cannot contain children", displayPath)
		}
		if err := validateCommand(child, namespace, pathParts, routes, paths); err != nil {
			return err
		}
	}
	return nil
}

func validateOptions(options []OptionDefinition, routes map[RouteID]string, owner string) error {
	if len(options) > 25 {
		return fmt.Errorf("%s exceeds 25 options", owner)
	}
	seen := make(map[string]struct{}, len(options))
	optionalSeen := false
	for _, option := range options {
		name := strings.TrimSpace(option.Name)
		if option.Name != name {
			return fmt.Errorf("%s option name %q has surrounding whitespace", owner, option.Name)
		}
		if !slashNamePattern.MatchString(name) {
			return fmt.Errorf("%s option %q has invalid name", owner, name)
		}
		if !validOptionKind(option.Kind) {
			return fmt.Errorf("%s option %q has unsupported kind %q", owner, name, option.Kind)
		}
		if strings.TrimSpace(option.Description) == "" {
			return fmt.Errorf("%s option %q requires a base description", owner, name)
		}
		if utf8.RuneCountInString(option.Description) > 100 {
			return fmt.Errorf("%s option %q description exceeds 100 characters", owner, name)
		}
		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("%s has duplicate option %q", owner, name)
		}
		seen[key] = struct{}{}
		if !option.Required {
			optionalSeen = true
		} else if optionalSeen {
			return fmt.Errorf("%s required option %q follows an optional option", owner, name)
		}

		if (option.MinInteger != nil || option.MaxInteger != nil) && option.Kind != OptionInteger {
			return fmt.Errorf("%s option %q has integer limits for noninteger kind", owner, name)
		}
		const maxSafeInteger int64 = 1<<53 - 1
		if option.MinInteger != nil && (*option.MinInteger < -maxSafeInteger || *option.MinInteger > maxSafeInteger) {
			return fmt.Errorf("%s option %q minimum is outside Discord safe integer range", owner, name)
		}
		if option.MaxInteger != nil && (*option.MaxInteger < -maxSafeInteger || *option.MaxInteger > maxSafeInteger) {
			return fmt.Errorf("%s option %q maximum is outside Discord safe integer range", owner, name)
		}
		if option.MinInteger != nil && option.MaxInteger != nil && *option.MinInteger > *option.MaxInteger {
			return fmt.Errorf("%s option %q integer minimum exceeds maximum", owner, name)
		}
		if (option.MinNumber != nil || option.MaxNumber != nil) && option.Kind != OptionNumber {
			return fmt.Errorf("%s option %q has number limits for nonnumber kind", owner, name)
		}
		if option.MinNumber != nil && (math.IsNaN(*option.MinNumber) || math.IsInf(*option.MinNumber, 0)) {
			return fmt.Errorf("%s option %q number minimum must be finite", owner, name)
		}
		if option.MaxNumber != nil && (math.IsNaN(*option.MaxNumber) || math.IsInf(*option.MaxNumber, 0)) {
			return fmt.Errorf("%s option %q number maximum must be finite", owner, name)
		}
		if option.MinNumber != nil && option.MaxNumber != nil && *option.MinNumber > *option.MaxNumber {
			return fmt.Errorf("%s option %q number minimum exceeds maximum", owner, name)
		}
		if (option.MinLength != nil || option.MaxLength != nil) && option.Kind != OptionString {
			return fmt.Errorf("%s option %q has length limits for nonstring kind", owner, name)
		}
		if option.MinLength != nil && *option.MinLength < 0 || option.MaxLength != nil && *option.MaxLength < 0 {
			return fmt.Errorf("%s option %q length cannot be negative", owner, name)
		}
		if option.MinLength != nil && option.MaxLength != nil && *option.MinLength > *option.MaxLength {
			return fmt.Errorf("%s option %q minimum length exceeds maximum", owner, name)
		}
		if len(option.ChannelKinds) > 0 && option.Kind != OptionChannel {
			return fmt.Errorf("%s option %q has channel kinds for nonchannel option", owner, name)
		}
		seenChannelKinds := make(map[ChannelKind]struct{}, len(option.ChannelKinds))
		for _, kind := range option.ChannelKinds {
			if !validChannelKind(kind) {
				return fmt.Errorf("%s option %q has unsupported channel kind %q", owner, name, kind)
			}
			if _, exists := seenChannelKinds[kind]; exists {
				return fmt.Errorf("%s option %q repeats channel kind %q", owner, name, kind)
			}
			seenChannelKinds[kind] = struct{}{}
		}
		if option.Autocomplete != "" {
			if option.Kind != OptionString && option.Kind != OptionInteger && option.Kind != OptionNumber {
				return fmt.Errorf("%s option %q kind cannot autocomplete", owner, name)
			}
			if len(option.Choices) > 0 {
				return fmt.Errorf("%s option %q cannot have choices and autocomplete", owner, name)
			}
			if err := addRouteReference(routes, option.Autocomplete, "autocomplete", fmt.Sprintf("%s option %s autocomplete", owner, name)); err != nil {
				return err
			}
		}
		if len(option.Choices) > 25 {
			return fmt.Errorf("%s option %q exceeds 25 choices", owner, name)
		}
		choiceNames := make(map[string]struct{}, len(option.Choices))
		for choiceIndex, choice := range option.Choices {
			choiceName := strings.TrimSpace(choice.Name)
			if choiceName == "" || utf8.RuneCountInString(choiceName) > 100 {
				return fmt.Errorf("%s option %q choice %d name must contain 1 to 100 characters", owner, name, choiceIndex+1)
			}
			if _, exists := choiceNames[choiceName]; exists {
				return fmt.Errorf("%s option %q has duplicate choice %q", owner, name, choiceName)
			}
			choiceNames[choiceName] = struct{}{}
			if err := choice.Value.Validate(); err != nil {
				return fmt.Errorf("%s option %q choice %q: %w", owner, name, choiceName, err)
			}
			if option.Kind == OptionString && choice.Value.Kind != ChoiceString || option.Kind == OptionInteger && choice.Value.Kind != ChoiceInteger || option.Kind == OptionNumber && choice.Value.Kind != ChoiceNumber {
				return fmt.Errorf("%s option %q choice %q kind does not match option", owner, name, choiceName)
			}
		}
	}
	return nil
}

func (v ChoiceValue) Validate() error {
	switch v.Kind {
	case ChoiceString:
		if !utf8.ValidString(v.String) || utf8.RuneCountInString(v.String) > 100 {
			return errors.New("string choice must be valid UTF-8 and at most 100 characters")
		}
		if v.Integer != 0 || v.Number != 0 {
			return errors.New("string choice contains inactive payload")
		}
	case ChoiceInteger:
		if v.String != "" || v.Number != 0 {
			return errors.New("integer choice contains inactive payload")
		}
	case ChoiceNumber:
		if math.IsNaN(v.Number) || math.IsInf(v.Number, 0) {
			return errors.New("number choice must be finite")
		}
		if v.String != "" || v.Integer != 0 {
			return errors.New("number choice contains inactive payload")
		}
	default:
		return fmt.Errorf("unsupported choice kind %q", v.Kind)
	}
	return nil
}

func validateChecks(checks []CheckDefinition, routes map[RouteID]string, owner string) error {
	for index, check := range checks {
		switch check.Kind {
		case CheckGuildOnly, CheckOwnerOnly:
			if len(check.Permissions) > 0 || check.Route != "" {
				return fmt.Errorf("%s check %d has unsupported arguments", owner, index+1)
			}
		case CheckHasPermissions:
			if len(check.Permissions) == 0 {
				return fmt.Errorf("%s permission check %d needs permissions", owner, index+1)
			}
			if check.Route != "" {
				return fmt.Errorf("%s permission check %d cannot have a route", owner, index+1)
			}
			seen := make(map[MemberPermission]struct{}, len(check.Permissions))
			for _, permission := range check.Permissions {
				if !validMemberPermission(permission) {
					return fmt.Errorf("%s permission check %d has unsupported permission %q", owner, index+1, permission)
				}
				if _, exists := seen[permission]; exists {
					return fmt.Errorf("%s permission check %d repeats permission %q", owner, index+1, permission)
				}
				seen[permission] = struct{}{}
			}
		case CheckCustom:
			if err := addRouteReference(routes, check.Route, "check", fmt.Sprintf("%s custom check %d", owner, index+1)); err != nil {
				return err
			}
		default:
			return fmt.Errorf("%s check %d has unsupported kind %q", owner, index+1, check.Kind)
		}
	}
	return nil
}

func validateNamedRoute(kind, id string, route RouteID, ids map[string]struct{}, routes map[RouteID]string) error {
	if !handlerIDPattern.MatchString(id) {
		return fmt.Errorf("%s id %q is not canonical", kind, id)
	}
	if _, exists := ids[id]; exists {
		return fmt.Errorf("duplicate %s id %q", kind, id)
	}
	ids[id] = struct{}{}
	return addRoute(routes, route, kind+" "+id)
}

func addRouteReference(routes map[RouteID]string, route RouteID, class, owner string) error {
	raw := string(route)
	if raw != strings.TrimSpace(raw) || !routeIDPattern.MatchString(raw) {
		return fmt.Errorf("%s route %q is not canonical", owner, route)
	}
	marker := "reference:" + class
	if previous, exists := routes[route]; exists {
		if previous == marker {
			return nil
		}
		return fmt.Errorf("duplicate route %q for %s and %s", route, previous, owner)
	}
	routes[route] = marker
	return nil
}

func addRoute(routes map[RouteID]string, route RouteID, owner string) error {
	raw := string(route)
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("%s route is required", owner)
	}
	if raw != strings.TrimSpace(raw) || !routeIDPattern.MatchString(raw) {
		return fmt.Errorf("%s route %q is not canonical", owner, route)
	}
	if previous, exists := routes[route]; exists {
		return fmt.Errorf("duplicate route %q used by %s and %s", route, previous, owner)
	}
	routes[route] = owner
	return nil
}
