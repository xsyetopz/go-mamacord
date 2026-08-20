package starlark

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

type generationLowerer struct {
	id     contract.GenerationID
	routes map[contract.RouteID]starlarkgo.Callable
}

func (lowerer *generationLowerer) lower(cogs []*apiValue) (contract.Definition, error) {
	definition := contract.Definition{Cogs: make([]contract.CogDefinition, len(cogs))}
	for index, value := range cogs {
		declaration, ok := value.data.(cogDeclaration)
		if !ok {
			return contract.Definition{}, errors.New("invalid cog declaration")
		}
		cog := contract.CogDefinition{Name: declaration.name}
		var err error
		if cog.Checks, err = lowerer.lowerChecks(declaration.checks); err != nil {
			return contract.Definition{}, fmt.Errorf("cog %q checks: %w", declaration.name, err)
		}
		if cog.Commands, err = lowerer.lowerCommands(declaration.commands, nil); err != nil {
			return contract.Definition{}, fmt.Errorf("cog %q commands: %w", declaration.name, err)
		}
		if cog.Listeners, err = lowerer.lowerListeners(declaration.listeners); err != nil {
			return contract.Definition{}, fmt.Errorf("cog %q listeners: %w", declaration.name, err)
		}
		if cog.Tasks, err = lowerer.lowerTasks(declaration.tasks); err != nil {
			return contract.Definition{}, fmt.Errorf("cog %q tasks: %w", declaration.name, err)
		}
		if cog.Components, err = lowerer.lowerComponents(declaration.components); err != nil {
			return contract.Definition{}, fmt.Errorf("cog %q components: %w", declaration.name, err)
		}
		if cog.Modals, err = lowerer.lowerModals(declaration.modals); err != nil {
			return contract.Definition{}, fmt.Errorf("cog %q modals: %w", declaration.name, err)
		}
		definition.Cogs[index] = cog
	}
	return definition, nil
}

func (lowerer *generationLowerer) lowerCommands(values []*apiValue, parent []string) ([]contract.CommandDefinition, error) {
	out := make([]contract.CommandDefinition, len(values))
	for index, value := range values {
		declaration := value.data.(commandDeclaration)
		kind, err := commandKind(declaration.kind)
		if err != nil {
			return nil, err
		}
		if declaration.kind == "group" && len(parent) == 0 {
			kind = contract.CommandSlash
		}
		command := contract.CommandDefinition{Kind: kind, Name: declaration.name, Description: declaration.description, DescriptionID: declaration.descriptionID, Ephemeral: declaration.ephemeral}
		if command.Defer, err = deferMode(declaration.deferMode); err != nil {
			return nil, fmt.Errorf("command %q: %w", declaration.name, err)
		}
		if command.DefaultMemberPermissions, err = memberPermissions(declaration.permissions); err != nil {
			return nil, fmt.Errorf("command %q: %w", declaration.name, err)
		}
		if command.Checks, err = lowerer.lowerChecks(declaration.checks); err != nil {
			return nil, fmt.Errorf("command %q checks: %w", declaration.name, err)
		}
		path := append(append([]string(nil), parent...), declaration.name)
		if len(declaration.children) != 0 {
			if command.Children, err = lowerer.lowerCommands(declaration.children, path); err != nil {
				return nil, err
			}
		} else {
			route, err := commandRoute(declaration, path)
			if err != nil {
				return nil, err
			}
			command.Route = route
			if err := lowerer.addRoute(route, declaration.handler, "command "+strings.Join(path, " ")); err != nil {
				return nil, err
			}
			if command.Options, err = lowerer.lowerOptions(declaration.options, route); err != nil {
				return nil, fmt.Errorf("command %q options: %w", declaration.name, err)
			}
		}
		out[index] = command
	}
	return out, nil
}

func commandKind(value string) (contract.CommandKind, error) {
	switch value {
	case "slash":
		return contract.CommandSlash, nil
	case "user":
		return contract.CommandUser, nil
	case "message":
		return contract.CommandMessage, nil
	case "group":
		return contract.CommandGroup, nil
	case "subcommand":
		return contract.CommandSubcommand, nil
	default:
		return "", fmt.Errorf("unsupported command kind %q", value)
	}
}
func commandRoute(declaration commandDeclaration, path []string) (contract.RouteID, error) {
	if declaration.id != "" {
		if !apiIDPattern.MatchString(declaration.id) {
			return "", fmt.Errorf("command id %q is not canonical", declaration.id)
		}
		return contract.RouteID("command:" + declaration.id), nil
	}
	if declaration.kind == "user" || declaration.kind == "message" {
		return "", fmt.Errorf("%s command %q requires an id", declaration.kind, declaration.name)
	}
	parts := make([]string, len(path))
	for index, part := range path {
		if !apiCommandNamePattern.MatchString(part) {
			return "", fmt.Errorf("command path part %q is not canonical", part)
		}
		parts[index] = part
	}
	return contract.RouteID("command:slash:" + strings.Join(parts, ".")), nil
}

func (lowerer *generationLowerer) lowerOptions(values []*apiValue, commandRoute contract.RouteID) ([]contract.OptionDefinition, error) {
	out := make([]contract.OptionDefinition, len(values))
	for index, value := range values {
		declaration := value.data.(optionDeclaration)
		kind, err := optionKind(declaration.kind)
		if err != nil {
			return nil, err
		}
		option := contract.OptionDefinition{Name: declaration.name, Kind: kind, Description: declaration.description, DescriptionID: declaration.descriptionID, Required: declaration.required, MinInteger: clonePointer(declaration.minInteger), MaxInteger: clonePointer(declaration.maxInteger), MinNumber: clonePointer(declaration.minNumber), MaxNumber: clonePointer(declaration.maxNumber), MinLength: clonePointer(declaration.minLength), MaxLength: clonePointer(declaration.maxLength)}
		if option.ChannelKinds, err = channelKinds(declaration.channelKinds); err != nil {
			return nil, err
		}
		if option.Choices, err = lowerChoices(declaration.choices); err != nil {
			return nil, err
		}
		if declaration.autocomplete != nil {
			suffix := strings.TrimPrefix(string(commandRoute), "command:")
			route := contract.RouteID("autocomplete:" + suffix + ":" + declaration.name)
			option.Autocomplete = route
			if err := lowerer.addRoute(route, declaration.autocomplete, "autocomplete "+declaration.name); err != nil {
				return nil, err
			}
		}
		out[index] = option
	}
	return out, nil
}

func optionKind(value string) (contract.OptionKind, error) {
	switch value {
	case "string":
		return contract.OptionString, nil
	case "integer":
		return contract.OptionInteger, nil
	case "number":
		return contract.OptionNumber, nil
	case "boolean":
		return contract.OptionBoolean, nil
	case "user":
		return contract.OptionUser, nil
	case "channel":
		return contract.OptionChannel, nil
	case "role":
		return contract.OptionRole, nil
	case "mentionable":
		return contract.OptionMentionable, nil
	case "attachment":
		return contract.OptionAttachment, nil
	default:
		return "", fmt.Errorf("unsupported option kind %q", value)
	}
}
func lowerChoices(values []*apiValue) ([]contract.ChoiceDefinition, error) {
	out := make([]contract.ChoiceDefinition, len(values))
	for index, value := range values {
		declaration := value.data.(choiceDeclaration)
		choice := contract.ChoiceDefinition{Name: declaration.name}
		switch typed := declaration.value.(type) {
		case starlarkgo.String:
			choice.Value = contract.ChoiceValue{Kind: contract.ChoiceString, String: string(typed)}
		case starlarkgo.Int:
			integer, ok := typed.Int64()
			if !ok {
				return nil, fmt.Errorf("choice %q integer is outside int64 range", declaration.name)
			}
			choice.Value = contract.ChoiceValue{Kind: contract.ChoiceInteger, Integer: integer}
		case starlarkgo.Float:
			number := float64(typed)
			if math.IsNaN(number) || math.IsInf(number, 0) {
				return nil, fmt.Errorf("choice %q number must be finite", declaration.name)
			}
			choice.Value = contract.ChoiceValue{Kind: contract.ChoiceNumber, Number: number}
		default:
			return nil, fmt.Errorf("choice %q has unsupported value", declaration.name)
		}
		out[index] = choice
	}
	return out, nil
}

func (lowerer *generationLowerer) lowerChecks(values []*apiValue) ([]contract.CheckDefinition, error) {
	out := make([]contract.CheckDefinition, len(values))
	for index, value := range values {
		declaration := value.data.(checkDeclaration)
		check := contract.CheckDefinition{}
		switch declaration.kind {
		case "guild_only":
			check.Kind = contract.CheckGuildOnly
		case "owner_only":
			check.Kind = contract.CheckOwnerOnly
		case "has_permissions":
			check.Kind = contract.CheckHasPermissions
			permissions, err := memberPermissions(declaration.permissions)
			if err != nil {
				return nil, err
			}
			check.Permissions = permissions
		case "custom":
			if !apiIDPattern.MatchString(declaration.id) {
				return nil, fmt.Errorf("check id %q is not canonical", declaration.id)
			}
			check.Kind = contract.CheckCustom
			check.Route = contract.RouteID("check:" + declaration.id)
			if err := lowerer.addRoute(check.Route, declaration.handler, "check "+declaration.id); err != nil {
				return nil, err
			}
		default:
			return nil, fmt.Errorf("unsupported check kind %q", declaration.kind)
		}
		out[index] = check
	}
	return out, nil
}

func (lowerer *generationLowerer) lowerComponents(values []*apiValue) ([]contract.ComponentDefinition, error) {
	out := make([]contract.ComponentDefinition, len(values))
	for index, value := range values {
		declaration := value.data.(componentDeclaration)
		if !apiIDPattern.MatchString(declaration.id) {
			return nil, fmt.Errorf("component id %q is not canonical", declaration.id)
		}
		component := contract.ComponentDefinition{ID: declaration.id, Route: contract.RouteID("component:" + declaration.id)}
		var err error
		if component.Defer, err = deferMode(declaration.deferMode); err != nil {
			return nil, err
		}
		if component.Kinds, err = componentKinds(declaration.kinds); err != nil {
			return nil, err
		}
		if component.Checks, err = lowerer.lowerChecks(declaration.checks); err != nil {
			return nil, err
		}
		if err := lowerer.addRoute(component.Route, declaration.handler, "component "+declaration.id); err != nil {
			return nil, err
		}
		out[index] = component
	}
	return out, nil
}
func (lowerer *generationLowerer) lowerModals(values []*apiValue) ([]contract.ModalDefinition, error) {
	out := make([]contract.ModalDefinition, len(values))
	for index, value := range values {
		declaration := value.data.(modalDeclaration)
		if !apiIDPattern.MatchString(declaration.id) {
			return nil, fmt.Errorf("modal id %q is not canonical", declaration.id)
		}
		modal := contract.ModalDefinition{ID: declaration.id, Route: contract.RouteID("modal:" + declaration.id)}
		var err error
		if modal.Defer, err = deferMode(declaration.deferMode); err != nil {
			return nil, err
		}
		if modal.Checks, err = lowerer.lowerChecks(declaration.checks); err != nil {
			return nil, err
		}
		for _, field := range declaration.fields {
			item := field.data.(modalFieldDeclaration)
			modal.Fields = append(modal.Fields, contract.ModalFieldDefinition{ID: item.id, Required: item.required})
		}
		if err := lowerer.addRoute(modal.Route, declaration.handler, "modal "+declaration.id); err != nil {
			return nil, err
		}
		out[index] = modal
	}
	return out, nil
}
func (lowerer *generationLowerer) lowerListeners(values []*apiValue) ([]contract.ListenerDefinition, error) {
	out := make([]contract.ListenerDefinition, len(values))
	for index, value := range values {
		declaration := value.data.(listenerDeclaration)
		if !apiIDPattern.MatchString(declaration.id) {
			return nil, fmt.Errorf("listener id %q is not canonical", declaration.id)
		}
		item := contract.ListenerDefinition{ID: declaration.id, Event: declaration.event, Route: contract.RouteID("listener:" + declaration.id)}
		var err error
		if item.Checks, err = lowerer.lowerChecks(declaration.checks); err != nil {
			return nil, err
		}
		if err := lowerer.addRoute(item.Route, declaration.handler, "listener "+declaration.id); err != nil {
			return nil, err
		}
		out[index] = item
	}
	return out, nil
}
func (lowerer *generationLowerer) lowerTasks(values []*apiValue) ([]contract.TaskDefinition, error) {
	out := make([]contract.TaskDefinition, len(values))
	for index, value := range values {
		declaration := value.data.(taskDeclaration)
		if !apiIDPattern.MatchString(declaration.id) {
			return nil, fmt.Errorf("task id %q is not canonical", declaration.id)
		}
		item := contract.TaskDefinition{ID: declaration.id, Schedule: declaration.schedule, Route: contract.RouteID("task:" + declaration.id)}
		var err error
		if item.Checks, err = lowerer.lowerChecks(declaration.checks); err != nil {
			return nil, err
		}
		if err := lowerer.addRoute(item.Route, declaration.handler, "task "+declaration.id); err != nil {
			return nil, err
		}
		out[index] = item
	}
	return out, nil
}

func (lowerer *generationLowerer) addRoute(route contract.RouteID, callable starlarkgo.Callable, owner string) error {
	if err := validateFunction(callable, owner); err != nil {
		return err
	}
	if previous, exists := lowerer.routes[route]; exists {
		if previous == callable {
			return nil
		}
		return fmt.Errorf("route %q has multiple handlers", route)
	}
	lowerer.routes[route] = callable
	return nil
}

func deferMode(value string) (contract.DeferMode, error) {
	switch value {
	case "":
		return contract.DeferNone, nil
	case "create":
		return contract.DeferCreate, nil
	case "update":
		return contract.DeferUpdate, nil
	default:
		return "", fmt.Errorf("unsupported defer mode %q", value)
	}
}
func memberPermissions(values []string) ([]contract.MemberPermission, error) {
	out := make([]contract.MemberPermission, len(values))
	for index, value := range values {
		out[index] = contract.MemberPermission(value)
	}
	return out, nil
}
func channelKinds(values []string) ([]contract.ChannelKind, error) {
	out := make([]contract.ChannelKind, len(values))
	for i, v := range values {
		out[i] = contract.ChannelKind(v)
	}
	return out, nil
}
func componentKinds(values []string) ([]contract.ComponentKind, error) {
	out := make([]contract.ComponentKind, len(values))
	for i, v := range values {
		out[i] = contract.ComponentKind(v)
	}
	return out, nil
}
func clonePointer[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
