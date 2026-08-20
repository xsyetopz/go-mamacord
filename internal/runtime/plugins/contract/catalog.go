package contract

import (
	"errors"
	"fmt"
	"math"
	"slices"
	"strings"
	"unicode/utf8"
)

type RouteType string

const (
	RouteCommand      RouteType = "command"
	RouteAutocomplete RouteType = "autocomplete"
	RouteComponent    RouteType = "component"
	RouteModal        RouteType = "modal"
	RouteListener     RouteType = "listener"
	RouteTask         RouteType = "task"
	RouteCheck        RouteType = "check"
)

type RouteSignature struct {
	Type           RouteType
	Route          RouteID
	CommandKind    CommandKind
	Defer          DeferMode
	Path           []string
	Options        []OptionDefinition
	OptionName     string
	OptionKind     OptionKind
	ComponentID    string
	ComponentKinds []ComponentKind
	ModalID        string
	ModalFields    []ModalFieldDefinition
	Event          string
	TaskID         string
}

func (signature RouteSignature) DeepClone() RouteSignature {
	out := signature
	out.Path = append([]string(nil), signature.Path...)
	out.Options = cloneOptions(signature.Options)
	out.ComponentKinds = append([]ComponentKind(nil), signature.ComponentKinds...)
	out.ModalFields = append([]ModalFieldDefinition(nil), signature.ModalFields...)
	return out
}

type RouteCatalog struct {
	signatures map[RouteID][]RouteSignature
}

func (definition Definition) Compile() (*RouteCatalog, error) {
	if err := definition.Validate(); err != nil {
		return nil, err
	}
	catalog := &RouteCatalog{signatures: make(map[RouteID][]RouteSignature)}
	for _, cog := range definition.Cogs {
		for _, check := range cog.Checks {
			catalog.addCheck(check)
		}
		for _, command := range cog.Commands {
			if err := catalog.addCommand(command, nil, command.Kind); err != nil {
				return nil, err
			}
		}
		for _, listener := range cog.Listeners {
			catalog.add(RouteSignature{Type: RouteListener, Route: listener.Route, Event: listener.Event})
			for _, check := range listener.Checks {
				catalog.addCheck(check)
			}
		}
		for _, task := range cog.Tasks {
			catalog.add(RouteSignature{Type: RouteTask, Route: task.Route, TaskID: task.ID})
			for _, check := range task.Checks {
				catalog.addCheck(check)
			}
		}
		for _, component := range cog.Components {
			catalog.add(RouteSignature{Type: RouteComponent, Route: component.Route, ComponentID: component.ID, ComponentKinds: append([]ComponentKind(nil), component.Kinds...), Defer: component.Defer})
			for _, check := range component.Checks {
				catalog.addCheck(check)
			}
		}
		for _, modal := range cog.Modals {
			catalog.add(RouteSignature{Type: RouteModal, Route: modal.Route, ModalID: modal.ID, ModalFields: append([]ModalFieldDefinition(nil), modal.Fields...), Defer: modal.Defer})
			for _, check := range modal.Checks {
				catalog.addCheck(check)
			}
		}
	}
	return catalog, nil
}

func (catalog *RouteCatalog) addCommand(command CommandDefinition, parent []string, rootKind CommandKind) error {
	path := append(append([]string(nil), parent...), command.Name)
	for _, check := range command.Checks {
		catalog.addCheck(check)
	}
	if len(command.Children) != 0 {
		for _, child := range command.Children {
			if err := catalog.addCommand(child, path, rootKind); err != nil {
				return err
			}
		}
		return nil
	}
	catalog.add(RouteSignature{Type: RouteCommand, Route: command.Route, CommandKind: rootKind, Defer: command.Defer, Path: path, Options: cloneOptions(command.Options)})
	for _, option := range command.Options {
		if option.Autocomplete != "" {
			catalog.add(RouteSignature{Type: RouteAutocomplete, Route: option.Autocomplete, Path: append([]string(nil), path...), Options: cloneOptions(command.Options), OptionName: option.Name, OptionKind: option.Kind})
		}
	}
	return nil
}

func (catalog *RouteCatalog) addCheck(check CheckDefinition) {
	if check.Kind == CheckCustom {
		catalog.add(RouteSignature{Type: RouteCheck, Route: check.Route})
	}
}

func (catalog *RouteCatalog) add(signature RouteSignature) {
	if signature.Type == RouteCheck {
		for _, existing := range catalog.signatures[signature.Route] {
			if existing.Type == RouteCheck {
				return
			}
		}
	}
	catalog.signatures[signature.Route] = append(catalog.signatures[signature.Route], signature)
}

func (catalog *RouteCatalog) Resolve(invocation Invocation) (RouteSignature, error) {
	if catalog == nil {
		return RouteSignature{}, errors.New("route catalog is nil")
	}
	if err := invocation.Validate(); err != nil {
		return RouteSignature{}, err
	}
	candidates := catalog.signatures[invocation.Route]
	if len(candidates) == 0 {
		return RouteSignature{}, fmt.Errorf("route %q is not registered", invocation.Route)
	}
	var matched *RouteSignature
	var matchErr error
	for _, signature := range candidates {
		if err := signature.match(invocation); err != nil {
			matchErr = err
			continue
		}
		if matched != nil {
			return RouteSignature{}, fmt.Errorf("route %q matches multiple signatures", invocation.Route)
		}
		copy := signature.DeepClone()
		matched = &copy
	}
	if matched == nil {
		if matchErr != nil {
			return RouteSignature{}, fmt.Errorf("route %q signature mismatch: %w", invocation.Route, matchErr)
		}
		return RouteSignature{}, fmt.Errorf("route %q has no compatible signature", invocation.Route)
	}
	return *matched, nil
}

func (signature RouteSignature) match(invocation Invocation) error {
	switch signature.Type {
	case RouteCommand:
		if invocation.Kind != InvocationCommand {
			return fmt.Errorf("requires command invocation, got %s", invocation.Kind)
		}
		if invocation.Command.Kind != signature.CommandKind {
			return fmt.Errorf("command kind %q does not match %q", invocation.Command.Kind, signature.CommandKind)
		}
		if err := signature.matchDefer(invocation.ResponseState); err != nil {
			return err
		}
		if !slices.Equal(invocation.Command.Path, signature.Path) {
			return fmt.Errorf("command path %q does not match %q", strings.Join(invocation.Command.Path, " "), strings.Join(signature.Path, " "))
		}
		return validateRuntimeOptions(invocation.Command.Options, signature.Options, true, invocation.Guild)
	case RouteAutocomplete:
		if invocation.Kind != InvocationAutocomplete {
			return fmt.Errorf("requires autocomplete invocation, got %s", invocation.Kind)
		}
		if !slices.Equal(invocation.Autocomplete.Path, signature.Path) || invocation.Autocomplete.Option != signature.OptionName || invocation.Autocomplete.Focused.Kind != signature.OptionKind {
			return errors.New("autocomplete path, option, or kind does not match")
		}
		if err := validateFocusedOption(invocation.Autocomplete.Focused, findOption(signature.Options, signature.OptionName)); err != nil {
			return err
		}
		return validateRuntimeOptions(invocation.Autocomplete.Options, signature.Options, false, invocation.Guild)
	case RouteComponent:
		if invocation.Kind != InvocationComponent {
			return fmt.Errorf("requires component invocation, got %s", invocation.Kind)
		}
		if invocation.Component.ID != signature.ComponentID || !slices.Contains(signature.ComponentKinds, invocation.Component.Kind) {
			return errors.New("component id or kind does not match")
		}
		if err := signature.matchDefer(invocation.ResponseState); err != nil {
			return err
		}
		return nil
	case RouteModal:
		if invocation.Kind != InvocationModal {
			return fmt.Errorf("requires modal invocation, got %s", invocation.Kind)
		}
		if invocation.Modal.ID != signature.ModalID {
			return errors.New("modal id does not match")
		}
		if err := signature.matchDefer(invocation.ResponseState); err != nil {
			return err
		}
		return validateModalFields(invocation.Modal.Fields, signature.ModalFields)
	case RouteListener:
		if invocation.Kind != InvocationEvent || invocation.Event.Name != signature.Event {
			return errors.New("listener event does not match")
		}
		return nil
	case RouteTask:
		if invocation.Kind != InvocationTask || invocation.Task.ID != signature.TaskID {
			return errors.New("task id does not match")
		}
		return nil
	case RouteCheck:
		if invocation.Kind != InvocationCheck {
			return fmt.Errorf("requires check invocation, got %s", invocation.Kind)
		}
		return nil
	default:
		return fmt.Errorf("unsupported route signature type %q", signature.Type)
	}
}

func (signature RouteSignature) matchDefer(state ResponseState) error {
	expected := ResponseUnacknowledged
	if signature.Defer == DeferCreate {
		expected = ResponseDeferredCreate
	} else if signature.Defer == DeferUpdate {
		expected = ResponseDeferredUpdate
	}
	if state != expected {
		return fmt.Errorf("response state %q does not match defer mode %q", state, signature.Defer)
	}
	return nil
}

func findOption(options []OptionDefinition, name string) *OptionDefinition {
	for index := range options {
		if options[index].Name == name {
			return &options[index]
		}
	}
	return nil
}

func validateRuntimeOptions(values []OptionValue, definitions []OptionDefinition, requireRequired bool, guild *GuildRef) error {
	provided := make(map[string]OptionValue, len(values))
	for _, value := range values {
		if _, exists := provided[value.Name]; exists {
			return fmt.Errorf("duplicate option %q", value.Name)
		}
		provided[value.Name] = value
	}
	for _, definition := range definitions {
		value, exists := provided[definition.Name]
		if !exists {
			if requireRequired && definition.Required {
				return fmt.Errorf("required option %q is absent", definition.Name)
			}
			continue
		}
		if value.Kind != definition.Kind {
			return fmt.Errorf("option %q kind %q does not match %q", value.Name, value.Kind, definition.Kind)
		}
		if err := validateRuntimeOption(value, definition, guild); err != nil {
			return fmt.Errorf("option %q: %w", value.Name, err)
		}
		delete(provided, definition.Name)
	}
	for name := range provided {
		return fmt.Errorf("option %q is not declared", name)
	}
	return nil
}

func validateFocusedOption(value OptionValue, definition *OptionDefinition) error {
	if definition == nil {
		return errors.New("focused option is not declared")
	}
	if value.Kind != definition.Kind {
		return errors.New("focused option kind does not match")
	}
	if value.Kind == OptionNumber && (math.IsNaN(value.Number) || math.IsInf(value.Number, 0)) {
		return errors.New("focused number must be finite")
	}
	return nil
}

func validateRuntimeOption(value OptionValue, definition OptionDefinition, guild *GuildRef) error {
	switch value.Kind {
	case OptionString:
		length := utf8.RuneCountInString(value.String)
		if definition.MinLength != nil && length < *definition.MinLength || definition.MaxLength != nil && length > *definition.MaxLength {
			return errors.New("string length is outside declared bounds")
		}
	case OptionInteger:
		if definition.MinInteger != nil && value.Integer < *definition.MinInteger || definition.MaxInteger != nil && value.Integer > *definition.MaxInteger {
			return errors.New("integer is outside declared bounds")
		}
	case OptionNumber:
		if definition.MinNumber != nil && value.Number < *definition.MinNumber || definition.MaxNumber != nil && value.Number > *definition.MaxNumber {
			return errors.New("number is outside declared bounds")
		}
	case OptionChannel:
		if len(definition.ChannelKinds) != 0 && !slices.Contains(definition.ChannelKinds, value.Channel.Kind) {
			return errors.New("channel kind is not allowed")
		}
		if guild != nil && value.Channel.GuildID != "" && value.Channel.GuildID != guild.ID {
			return errors.New("channel belongs to another guild")
		}
	case OptionRole:
		if guild != nil && value.Role.GuildID != "" && value.Role.GuildID != guild.ID {
			return errors.New("role belongs to another guild")
		}
	case OptionMentionable:
		if value.Mentionable.Role != nil && guild != nil && value.Mentionable.Role.GuildID != "" && value.Mentionable.Role.GuildID != guild.ID {
			return errors.New("mentionable role belongs to another guild")
		}
	}
	if len(definition.Choices) != 0 {
		matched := false
		for _, choice := range definition.Choices {
			if choiceMatchesOption(choice.Value, value) {
				matched = true
				break
			}
		}
		if !matched {
			return errors.New("value is not a declared choice")
		}
	}
	return nil
}

func choiceMatchesOption(choice ChoiceValue, value OptionValue) bool {
	switch choice.Kind {
	case ChoiceString:
		return value.Kind == OptionString && value.String == choice.String
	case ChoiceInteger:
		return value.Kind == OptionInteger && value.Integer == choice.Integer
	case ChoiceNumber:
		return value.Kind == OptionNumber && value.Number == choice.Number
	default:
		return false
	}
}

func validateModalFields(values []NamedString, definitions []ModalFieldDefinition) error {
	provided := make(map[string]string, len(values))
	for _, value := range values {
		provided[value.Name] = value.Value
	}
	for _, definition := range definitions {
		value, exists := provided[definition.ID]
		if !exists && definition.Required {
			return fmt.Errorf("required modal field %q is absent", definition.ID)
		}
		if exists && definition.Required && value == "" {
			return fmt.Errorf("required modal field %q is empty", definition.ID)
		}
		delete(provided, definition.ID)
	}
	for name := range provided {
		return fmt.Errorf("modal field %q is not declared", name)
	}
	return nil
}
func (catalog *RouteCatalog) ValidateOutcome(invocation Invocation, outcome Outcome) error {
	if _, err := catalog.Resolve(invocation); err != nil {
		return err
	}
	if err := outcome.Validate(invocation); err != nil {
		return err
	}
	for index, operation := range outcome.Operations {
		switch value := operation.(type) {
		case *MessageOperation:
			if err := catalog.validateRows(value.Message.Components); err != nil {
				return fmt.Errorf("operation %d: %w", index+1, err)
			}
		case *UpdateOperation:
			if value.Patch.Components.Set {
				if err := catalog.validateRows(value.Patch.Components.Values); err != nil {
					return fmt.Errorf("operation %d: %w", index+1, err)
				}
			}
		case *EditResponseOperation:
			if value.Patch.Components.Set {
				if err := catalog.validateRows(value.Patch.Components.Values); err != nil {
					return fmt.Errorf("operation %d: %w", index+1, err)
				}
			}
		case *BestEffortOperation:
			if err := catalog.validateNestedOperation(value.Operation); err != nil {
				return fmt.Errorf("operation %d: %w", index+1, err)
			}
		case *GuardedOperation:
			if err := catalog.validateGuardedOperation(value); err != nil {
				return fmt.Errorf("operation %d: %w", index+1, err)
			}
		case *SendChannelOperation:
			if err := catalog.validateRows(value.Message.Components); err != nil {
				return fmt.Errorf("operation %d: %w", index+1, err)
			}
		case *SendDMOperation:
			if err := catalog.validateRows(value.Message.Components); err != nil {
				return fmt.Errorf("operation %d: %w", index+1, err)
			}
		case *ModalOperation:
			if err := catalog.validateModalView(value.Modal); err != nil {
				return fmt.Errorf("operation %d: %w", index+1, err)
			}
		}
	}
	return nil
}

func (catalog *RouteCatalog) ValidateCheckDecision(invocation Invocation, decision CheckDecision) error {
	signature, err := catalog.Resolve(invocation)
	if err != nil {
		return err
	}
	if signature.Type != RouteCheck {
		return errors.New("invocation does not resolve to a check route")
	}
	if err := decision.Validate(); err != nil {
		return err
	}
	if decision.Kind == CheckDeniedMessage {
		if decision.Denial == nil || !decision.Denial.Ephemeral {
			return errors.New("check denial message must be ephemeral")
		}
		if err := catalog.validateRows(decision.Denial.Message.Components); err != nil {
			return err
		}
	}
	return nil
}

func (catalog *RouteCatalog) validateRows(rows []ComponentRow) error {
	for rowIndex, row := range rows {
		for componentIndex, component := range row.Components {
			switch value := component.(type) {
			case *Button:
				if value.Style != ButtonLink && !catalog.componentSupports(value.Handler, ComponentButton) {
					return fmt.Errorf("row %d component %d references undeclared button handler %q", rowIndex+1, componentIndex+1, value.Handler)
				}
			case *Select:
				kind := componentKindForSelect(value.Kind)
				if !catalog.componentSupports(value.Handler, kind) {
					return fmt.Errorf("row %d component %d references undeclared %s handler %q", rowIndex+1, componentIndex+1, kind, value.Handler)
				}
			}
		}
	}
	return nil
}

func componentKindForSelect(kind SelectKind) ComponentKind {
	switch kind {
	case SelectString:
		return ComponentStringSelect
	case SelectUser:
		return ComponentUserSelect
	case SelectRole:
		return ComponentRoleSelect
	case SelectMentionable:
		return ComponentMentionableSelect
	case SelectChannel:
		return ComponentChannelSelect
	default:
		return ""
	}
}

func (catalog *RouteCatalog) componentSupports(id string, kind ComponentKind) bool {
	for _, signatures := range catalog.signatures {
		for _, signature := range signatures {
			if signature.Type == RouteComponent && signature.ComponentID == id && slices.Contains(signature.ComponentKinds, kind) {
				return true
			}
		}
	}
	return false
}

func (catalog *RouteCatalog) validateModalView(modal ModalView) error {
	for _, signatures := range catalog.signatures {
		for _, signature := range signatures {
			if signature.Type != RouteModal || signature.ModalID != modal.Handler {
				continue
			}
			if len(signature.ModalFields) != len(modal.Fields) {
				return fmt.Errorf("modal handler %q field count does not match declaration", modal.Handler)
			}
			definitions := make(map[string]ModalFieldDefinition, len(signature.ModalFields))
			for _, field := range signature.ModalFields {
				definitions[field.ID] = field
			}
			for _, field := range modal.Fields {
				definition, exists := definitions[field.ID]
				if !exists || definition.Required != field.Required {
					return fmt.Errorf("modal handler %q field %q does not match declaration", modal.Handler, field.ID)
				}
				delete(definitions, field.ID)
			}
			if len(definitions) != 0 {
				return fmt.Errorf("modal handler %q omits declared fields", modal.Handler)
			}
			return nil
		}
	}
	return fmt.Errorf("modal handler %q is not declared", modal.Handler)
}

func (catalog *RouteCatalog) validateGuardedOperation(operation *GuardedOperation) error {
	if operation == nil {
		return errors.New("guarded operation is nil")
	}
	for _, candidate := range []Operation{operation.Operation, operation.Failure} {
		switch value := candidate.(type) {
		case *SendChannelOperation:
			if err := catalog.validateRows(value.Message.Components); err != nil {
				return err
			}
		case *SendDMOperation:
			if err := catalog.validateRows(value.Message.Components); err != nil {
				return err
			}
		case *MessageOperation:
			if err := catalog.validateRows(value.Message.Components); err != nil {
				return err
			}
		case *EditResponseOperation:
			if value.Patch.Components.Set {
				if err := catalog.validateRows(value.Patch.Components.Values); err != nil {
					return err
				}
			}
		case *UpdateOperation:
			if value.Patch.Components.Set {
				if err := catalog.validateRows(value.Patch.Components.Values); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (catalog *RouteCatalog) validateNestedOperation(candidate Operation) error {
	switch value := candidate.(type) {
	case *SendChannelOperation:
		return catalog.validateRows(value.Message.Components)
	case *SendDMOperation:
		return catalog.validateRows(value.Message.Components)
	case *MessageOperation:
		return catalog.validateRows(value.Message.Components)
	case *EditResponseOperation:
		if value.Patch.Components.Set {
			return catalog.validateRows(value.Patch.Components.Values)
		}
	case *UpdateOperation:
		if value.Patch.Components.Set {
			return catalog.validateRows(value.Patch.Components.Values)
		}
	}
	return nil
}
