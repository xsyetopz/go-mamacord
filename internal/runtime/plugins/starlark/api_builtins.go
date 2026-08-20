package starlark

import (
	"errors"
	"fmt"
	"math"

	starlarkgo "go.starlark.net/starlark"
)

const maxAuthorCollectionItems = 500

type builtinFunc = func(*starlarkgo.Thread, *starlarkgo.Builtin, starlarkgo.Tuple, []starlarkgo.Tuple) (starlarkgo.Value, error)

func AuthorAPI() starlarkgo.StringDict {
	api := starlarkgo.StringDict{
		"plugin":             starlarkgo.NewBuiltin("plugin", builtinPlugin),
		"cog":                starlarkgo.NewBuiltin("cog", builtinCog),
		"slash_command":      starlarkgo.NewBuiltin("slash_command", commandBuiltin("slash")),
		"user_command":       starlarkgo.NewBuiltin("user_command", contextCommandBuiltin("user")),
		"message_command":    starlarkgo.NewBuiltin("message_command", contextCommandBuiltin("message")),
		"group":              starlarkgo.NewBuiltin("group", groupBuiltin("group")),
		"subcommand":         starlarkgo.NewBuiltin("subcommand", commandBuiltin("subcommand")),
		"option":             starlarkgo.NewBuiltin("option", builtinOption),
		"string_option":      starlarkgo.NewBuiltin("string_option", typedOptionBuiltin("string")),
		"boolean_option":     starlarkgo.NewBuiltin("boolean_option", typedOptionBuiltin("boolean")),
		"integer_option":     starlarkgo.NewBuiltin("integer_option", typedOptionBuiltin("integer")),
		"number_option":      starlarkgo.NewBuiltin("number_option", typedOptionBuiltin("number")),
		"user_option":        starlarkgo.NewBuiltin("user_option", typedOptionBuiltin("user")),
		"channel_option":     starlarkgo.NewBuiltin("channel_option", typedOptionBuiltin("channel")),
		"role_option":        starlarkgo.NewBuiltin("role_option", typedOptionBuiltin("role")),
		"mentionable_option": starlarkgo.NewBuiltin("mentionable_option", typedOptionBuiltin("mentionable")),
		"attachment_option":  starlarkgo.NewBuiltin("attachment_option", typedOptionBuiltin("attachment")),
		"choice":             starlarkgo.NewBuiltin("choice", builtinChoice),
		"guild_only":         starlarkgo.NewBuiltin("guild_only", simpleCheckBuiltin("guild_only")),
		"owner_only":         starlarkgo.NewBuiltin("owner_only", simpleCheckBuiltin("owner_only")),
		"has_permissions":    starlarkgo.NewBuiltin("has_permissions", builtinHasPermissions),
		"custom_check":       starlarkgo.NewBuiltin("custom_check", builtinCustomCheck),
		"component":          starlarkgo.NewBuiltin("component", builtinComponent),
		"modal":              starlarkgo.NewBuiltin("modal", builtinModal),
		"modal_field":        starlarkgo.NewBuiltin("modal_field", builtinModalField),
		"listener":           starlarkgo.NewBuiltin("listener", builtinListener),
		"task":               starlarkgo.NewBuiltin("task", builtinTask),
	}
	for name, value := range effectAPI() {
		api[name] = value
	}
	for name, value := range domainEffectAPI() {
		api[name] = value
	}
	return api
}

func builtinPlugin(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var setup starlarkgo.Callable
	if err := starlarkgo.UnpackArgs("plugin", args, kwargs, "setup", &setup); err != nil {
		return nil, err
	}
	return &apiValue{kind: apiPlugin, data: pluginDeclaration{setup: setup}}, nil
}

func builtinCog(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var name string
	var checks, commands, listeners, tasks, components, modals starlarkgo.Value
	if err := starlarkgo.UnpackArgs("cog", args, kwargs, "name", &name, "checks?", &checks, "commands?", &commands, "listeners?", &listeners, "tasks?", &tasks, "components?", &components, "modals?", &modals); err != nil {
		return nil, err
	}
	values := cogDeclaration{name: name}
	var err error
	if values.checks, err = apiValueList(checks, apiCheck); err != nil {
		return nil, fmt.Errorf("cog checks: %w", err)
	}
	if values.commands, err = apiValueList(commands, apiCommand); err != nil {
		return nil, fmt.Errorf("cog commands: %w", err)
	}
	if values.listeners, err = apiValueList(listeners, apiListener); err != nil {
		return nil, fmt.Errorf("cog listeners: %w", err)
	}
	if values.tasks, err = apiValueList(tasks, apiTask); err != nil {
		return nil, fmt.Errorf("cog tasks: %w", err)
	}
	if values.components, err = apiValueList(components, apiComponent); err != nil {
		return nil, fmt.Errorf("cog components: %w", err)
	}
	if values.modals, err = apiValueList(modals, apiModal); err != nil {
		return nil, fmt.Errorf("cog modals: %w", err)
	}
	return &apiValue{kind: apiCog, data: values}, nil
}

func commandBuiltin(kind string) builtinFunc {
	return func(_ *starlarkgo.Thread, builtin *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		var name, description, descriptionID, id, deferMode string
		var handler starlarkgo.Callable
		var ephemeral bool
		var options, checks, permissions starlarkgo.Value
		if err := starlarkgo.UnpackArgs(builtin.Name(), args, kwargs, "name", &name, "description", &description, "handler", &handler, "id?", &id, "description_id?", &descriptionID, "ephemeral?", &ephemeral, "defer?", &deferMode, "permissions?", &permissions, "options?", &options, "checks?", &checks); err != nil {
			return nil, err
		}
		value := commandDeclaration{kind: kind, id: id, name: name, description: description, descriptionID: descriptionID, handler: handler, ephemeral: ephemeral, deferMode: deferMode}
		var err error
		if value.permissions, err = stringList(permissions); err != nil {
			return nil, fmt.Errorf("permissions: %w", err)
		}
		if value.options, err = apiValueList(options, apiOption); err != nil {
			return nil, fmt.Errorf("options: %w", err)
		}
		if value.checks, err = apiValueList(checks, apiCheck); err != nil {
			return nil, fmt.Errorf("checks: %w", err)
		}
		return &apiValue{kind: apiCommand, data: value}, nil
	}
}

func contextCommandBuiltin(kind string) builtinFunc {
	return func(_ *starlarkgo.Thread, builtin *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		var id, name, deferMode string
		var handler starlarkgo.Callable
		var ephemeral bool
		var checks, permissions starlarkgo.Value
		if err := starlarkgo.UnpackArgs(builtin.Name(), args, kwargs, "id", &id, "name", &name, "handler", &handler, "ephemeral?", &ephemeral, "defer?", &deferMode, "permissions?", &permissions, "checks?", &checks); err != nil {
			return nil, err
		}
		value := commandDeclaration{kind: kind, id: id, name: name, handler: handler, ephemeral: ephemeral, deferMode: deferMode}
		var err error
		if value.permissions, err = stringList(permissions); err != nil {
			return nil, fmt.Errorf("permissions: %w", err)
		}
		if value.checks, err = apiValueList(checks, apiCheck); err != nil {
			return nil, fmt.Errorf("checks: %w", err)
		}
		return &apiValue{kind: apiCommand, data: value}, nil
	}
}

func groupBuiltin(kind string) builtinFunc {
	return func(_ *starlarkgo.Thread, builtin *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		var name, description, descriptionID string
		var children, checks, permissions starlarkgo.Value
		if err := starlarkgo.UnpackArgs(builtin.Name(), args, kwargs, "name", &name, "description", &description, "children", &children, "description_id?", &descriptionID, "permissions?", &permissions, "checks?", &checks); err != nil {
			return nil, err
		}
		value := commandDeclaration{kind: kind, name: name, description: description, descriptionID: descriptionID}
		var err error
		if value.children, err = apiValueList(children, apiCommand); err != nil {
			return nil, fmt.Errorf("children: %w", err)
		}
		if value.checks, err = apiValueList(checks, apiCheck); err != nil {
			return nil, fmt.Errorf("checks: %w", err)
		}
		if value.permissions, err = stringList(permissions); err != nil {
			return nil, fmt.Errorf("permissions: %w", err)
		}
		return &apiValue{kind: apiCommand, data: value}, nil
	}
}

func typedOptionBuiltin(kind string) builtinFunc {
	return func(thread *starlarkgo.Thread, builtin *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		withKind := append(append([]starlarkgo.Tuple(nil), kwargs...), starlarkgo.Tuple{starlarkgo.String("kind"), starlarkgo.String(kind)})
		return builtinOption(thread, builtin, args, withKind)
	}
}

func builtinOption(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var kind, name, description, descriptionID string
	var required bool
	var choices, channelKinds starlarkgo.Value
	var minInteger, maxInteger, minNumber, maxNumber, minLength, maxLength starlarkgo.Value
	var autocomplete starlarkgo.Callable
	if err := starlarkgo.UnpackArgs("option", args, kwargs, "kind", &kind, "name", &name, "description", &description, "description_id?", &descriptionID, "required?", &required, "choices?", &choices, "min_integer?", &minInteger, "max_integer?", &maxInteger, "min_number?", &minNumber, "max_number?", &maxNumber, "min_length?", &minLength, "max_length?", &maxLength, "channel_kinds?", &channelKinds, "autocomplete?", &autocomplete); err != nil {
		return nil, err
	}
	value := optionDeclaration{kind: kind, name: name, description: description, descriptionID: descriptionID, required: required, autocomplete: autocomplete}
	var err error
	if value.choices, err = apiValueList(choices, apiChoice); err != nil {
		return nil, fmt.Errorf("choices: %w", err)
	}
	if value.channelKinds, err = stringList(channelKinds); err != nil {
		return nil, fmt.Errorf("channel_kinds: %w", err)
	}
	if value.minInteger, err = optionalInt64(minInteger); err != nil {
		return nil, fmt.Errorf("min_integer: %w", err)
	}
	if value.maxInteger, err = optionalInt64(maxInteger); err != nil {
		return nil, fmt.Errorf("max_integer: %w", err)
	}
	if value.minNumber, err = optionalFloat(minNumber); err != nil {
		return nil, fmt.Errorf("min_number: %w", err)
	}
	if value.maxNumber, err = optionalFloat(maxNumber); err != nil {
		return nil, fmt.Errorf("max_number: %w", err)
	}
	if value.minLength, err = optionalInt(minLength); err != nil {
		return nil, fmt.Errorf("min_length: %w", err)
	}
	if value.maxLength, err = optionalInt(maxLength); err != nil {
		return nil, fmt.Errorf("max_length: %w", err)
	}
	return &apiValue{kind: apiOption, data: value}, nil
}

func builtinChoice(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var name string
	var value starlarkgo.Value
	if err := starlarkgo.UnpackArgs("choice", args, kwargs, "name", &name, "value", &value); err != nil {
		return nil, err
	}
	switch value.(type) {
	case starlarkgo.String, starlarkgo.Int, starlarkgo.Float:
	default:
		return nil, fmt.Errorf("choice value must be string, int, or float, got %s", value.Type())
	}
	return &apiValue{kind: apiChoice, data: choiceDeclaration{name: name, value: value}}, nil
}

func simpleCheckBuiltin(kind string) builtinFunc {
	return func(_ *starlarkgo.Thread, builtin *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		if err := starlarkgo.UnpackArgs(builtin.Name(), args, kwargs); err != nil {
			return nil, err
		}
		return &apiValue{kind: apiCheck, data: checkDeclaration{kind: kind}}, nil
	}
}
func builtinHasPermissions(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var permissions starlarkgo.Value
	if err := starlarkgo.UnpackArgs("has_permissions", args, kwargs, "permissions", &permissions); err != nil {
		return nil, err
	}
	values, err := stringList(permissions)
	if err != nil {
		return nil, err
	}
	return &apiValue{kind: apiCheck, data: checkDeclaration{kind: "has_permissions", permissions: values}}, nil
}
func builtinCustomCheck(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var id string
	var handler starlarkgo.Callable
	if err := starlarkgo.UnpackArgs("custom_check", args, kwargs, "id", &id, "handler", &handler); err != nil {
		return nil, err
	}
	return &apiValue{kind: apiCheck, data: checkDeclaration{kind: "custom", id: id, handler: handler}}, nil
}

func builtinComponent(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var id, deferMode string
	var handler starlarkgo.Callable
	var kinds, checks starlarkgo.Value
	if err := starlarkgo.UnpackArgs("component", args, kwargs, "id", &id, "handler", &handler, "kinds", &kinds, "defer?", &deferMode, "checks?", &checks); err != nil {
		return nil, err
	}
	value := componentDeclaration{id: id, handler: handler, deferMode: deferMode}
	var err error
	if value.kinds, err = stringList(kinds); err != nil {
		return nil, fmt.Errorf("kinds: %w", err)
	}
	if value.checks, err = apiValueList(checks, apiCheck); err != nil {
		return nil, fmt.Errorf("checks: %w", err)
	}
	return &apiValue{kind: apiComponent, data: value}, nil
}
func builtinModalField(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var id string
	var required bool
	if err := starlarkgo.UnpackArgs("modal_field", args, kwargs, "id", &id, "required?", &required); err != nil {
		return nil, err
	}
	return &apiValue{kind: apiModalField, data: modalFieldDeclaration{id: id, required: required}}, nil
}
func builtinModal(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var id, deferMode string
	var handler starlarkgo.Callable
	var fields, checks starlarkgo.Value
	if err := starlarkgo.UnpackArgs("modal", args, kwargs, "id", &id, "handler", &handler, "fields", &fields, "defer?", &deferMode, "checks?", &checks); err != nil {
		return nil, err
	}
	value := modalDeclaration{id: id, handler: handler, deferMode: deferMode}
	var err error
	if value.fields, err = apiValueList(fields, apiModalField); err != nil {
		return nil, fmt.Errorf("fields: %w", err)
	}
	if value.checks, err = apiValueList(checks, apiCheck); err != nil {
		return nil, fmt.Errorf("checks: %w", err)
	}
	return &apiValue{kind: apiModal, data: value}, nil
}
func builtinListener(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var id, event string
	var handler starlarkgo.Callable
	var checks starlarkgo.Value
	if err := starlarkgo.UnpackArgs("listener", args, kwargs, "id", &id, "event", &event, "handler", &handler, "checks?", &checks); err != nil {
		return nil, err
	}
	value := listenerDeclaration{id: id, event: event, handler: handler}
	var err error
	if value.checks, err = apiValueList(checks, apiCheck); err != nil {
		return nil, err
	}
	return &apiValue{kind: apiListener, data: value}, nil
}
func builtinTask(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var id, schedule string
	var handler starlarkgo.Callable
	var checks starlarkgo.Value
	if err := starlarkgo.UnpackArgs("task", args, kwargs, "id", &id, "schedule", &schedule, "handler", &handler, "checks?", &checks); err != nil {
		return nil, err
	}
	value := taskDeclaration{id: id, schedule: schedule, handler: handler}
	var err error
	if value.checks, err = apiValueList(checks, apiCheck); err != nil {
		return nil, err
	}
	return &apiValue{kind: apiTask, data: value}, nil
}

func apiValueList(value starlarkgo.Value, kind apiValueKind) ([]*apiValue, error) {
	if value == nil || value == starlarkgo.None {
		return nil, nil
	}
	iterable, ok := value.(starlarkgo.Iterable)
	if !ok {
		return nil, fmt.Errorf("want iterable of mamacord.%s, got %s", kind, value.Type())
	}
	iterator := iterable.Iterate()
	defer iterator.Done()
	var item starlarkgo.Value
	var out []*apiValue
	for iterator.Next(&item) {
		declaration, ok := item.(*apiValue)
		if !ok || declaration == nil || declaration.kind != kind {
			return nil, fmt.Errorf("want mamacord.%s, got %s", kind, item.Type())
		}
		out = append(out, declaration)
		if len(out) > maxAuthorCollectionItems {
			return nil, fmt.Errorf("collection exceeds %d items", maxAuthorCollectionItems)
		}
	}
	return out, nil
}
func stringList(value starlarkgo.Value) ([]string, error) {
	if value == nil || value == starlarkgo.None {
		return nil, nil
	}
	iterable, ok := value.(starlarkgo.Iterable)
	if !ok {
		return nil, fmt.Errorf("want iterable of strings, got %s", value.Type())
	}
	iterator := iterable.Iterate()
	defer iterator.Done()
	var item starlarkgo.Value
	var out []string
	for iterator.Next(&item) {
		text, ok := starlarkgo.AsString(item)
		if !ok {
			return nil, fmt.Errorf("want string, got %s", item.Type())
		}
		out = append(out, text)
		if len(out) > maxAuthorCollectionItems {
			return nil, fmt.Errorf("collection exceeds %d items", maxAuthorCollectionItems)
		}
	}
	return out, nil
}
func optionalInt64(value starlarkgo.Value) (*int64, error) {
	if value == nil || value == starlarkgo.None {
		return nil, nil
	}
	var result int64
	if err := starlarkgo.AsInt(value, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
func optionalInt(value starlarkgo.Value) (*int, error) {
	if value == nil || value == starlarkgo.None {
		return nil, nil
	}
	var result int
	if err := starlarkgo.AsInt(value, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
func optionalFloat(value starlarkgo.Value) (*float64, error) {
	if value == nil || value == starlarkgo.None {
		return nil, nil
	}
	var result float64
	switch typed := value.(type) {
	case starlarkgo.Float:
		result = float64(typed)
	case starlarkgo.Int:
		integer, ok := typed.Int64()
		if !ok {
			return nil, errors.New("integer is outside int64 range")
		}
		result = float64(integer)
	default:
		return nil, fmt.Errorf("want number, got %s", value.Type())
	}
	if math.IsNaN(result) || math.IsInf(result, 0) {
		return nil, errors.New("number must be finite")
	}
	return &result, nil
}
