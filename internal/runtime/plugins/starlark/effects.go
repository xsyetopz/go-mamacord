package starlark

import (
	"errors"
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

func effectAPI() starlarkgo.StringDict {
	return starlarkgo.StringDict{
		"reply":                starlarkgo.NewBuiltin("reply", builtinReply),
		"update":               starlarkgo.NewBuiltin("update", builtinUpdate),
		"attempt":              starlarkgo.NewBuiltin("attempt", builtinAttempt),
		"best_effort":          starlarkgo.NewBuiltin("best_effort", builtinBestEffort),
		"button":               starlarkgo.NewBuiltin("button", builtinButton),
		"select":               starlarkgo.NewBuiltin("select", builtinSelect),
		"select_option":        starlarkgo.NewBuiltin("select_option", builtinSelectOption),
		"row":                  starlarkgo.NewBuiltin("row", builtinRow),
		"embed":                starlarkgo.NewBuiltin("embed", builtinEmbed),
		"embed_field":          starlarkgo.NewBuiltin("embed_field", builtinEmbedField),
		"embed_author":         starlarkgo.NewBuiltin("embed_author", builtinEmbedAuthor),
		"embed_footer":         starlarkgo.NewBuiltin("embed_footer", builtinEmbedFooter),
		"text_input":           starlarkgo.NewBuiltin("text_input", builtinTextInput),
		"modal_view":           starlarkgo.NewBuiltin("modal_view", builtinModalView),
		"show_modal":           starlarkgo.NewBuiltin("show_modal", builtinShowModal),
		"kv_put":               starlarkgo.NewBuiltin("kv_put", builtinKVPut),
		"kv_delete":            starlarkgo.NewBuiltin("kv_delete", builtinKVDelete),
		"autocomplete_choice":  starlarkgo.NewBuiltin("autocomplete_choice", builtinAutocompleteChoice),
		"autocomplete_choices": starlarkgo.NewBuiltin("autocomplete_choices", builtinAutocompleteChoices),
	}
}

func builtinBestEffort(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var operation *apiValue
	if err := starlarkgo.UnpackArgs("best_effort", args, kwargs, "effect", &operation); err != nil {
		return nil, err
	}
	if operation == nil || operation.kind != apiEffect {
		return nil, errors.New("best_effort requires a Mamacord effect")
	}
	return effectValue(effectBestEffort, operation.data.(effectDeclaration)), nil
}

func builtinAttempt(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var operation, failure *apiValue
	if err := starlarkgo.UnpackArgs("attempt", args, kwargs, "effect", &operation, "on_error", &failure); err != nil {
		return nil, err
	}
	if operation == nil || operation.kind != apiEffect {
		return nil, errors.New("attempt effect must be a Mamacord effect")
	}
	if failure == nil || failure.kind != apiEffect {
		return nil, errors.New("attempt on_error must be a Mamacord effect")
	}
	failureDeclaration := failure.data.(effectDeclaration)
	if failureDeclaration.kind != effectReply && failureDeclaration.kind != effectUpdate {
		return nil, errors.New("attempt on_error must be reply or update")
	}
	return effectValue(effectGuarded, guardedEffectDeclaration{operation: operation.data.(effectDeclaration), failure: failureDeclaration}), nil
}

func builtinReply(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var content string
	var ephemeral bool
	var components, embeds starlarkgo.Value
	if err := starlarkgo.UnpackArgs("reply", args, kwargs,
		"content?", &content,
		"ephemeral?", &ephemeral,
		"embeds?", &embeds,
		"components?", &components,
	); err != nil {
		return nil, err
	}
	rows, err := messageRows(components)
	if err != nil {
		return nil, err
	}
	embedValues, err := messageEmbeds(embeds)
	if err != nil {
		return nil, err
	}
	return effectValue(effectReply, replyDeclaration{message: contract.Message{Content: content, Embeds: embedValues, Components: rows}, ephemeral: ephemeral}), nil
}

func builtinUpdate(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var content, embeds, components starlarkgo.Value
	if err := starlarkgo.UnpackArgs("update", args, kwargs,
		"content?", &content,
		"embeds?", &embeds,
		"components?", &components,
	); err != nil {
		return nil, err
	}
	patch := contract.MessagePatch{}
	if content != nil && content != starlarkgo.None {
		text, ok := starlarkgo.AsString(content)
		if !ok {
			return nil, fmt.Errorf("content must be string, got %s", content.Type())
		}
		patch.Content = contract.OptionalString{Set: true, Value: text}
	}
	if embeds != nil && embeds != starlarkgo.None {
		values, err := messageEmbeds(embeds)
		if err != nil {
			return nil, err
		}
		patch.Embeds = contract.OptionalEmbeds{Set: true, Values: values}
	}
	if components != nil && components != starlarkgo.None {
		rows, err := messageRows(components)
		if err != nil {
			return nil, err
		}
		patch.Components = contract.OptionalComponentRows{Set: true, Values: rows}
	}
	return effectValue(effectUpdate, updateDeclaration{patch: patch}), nil
}

func builtinButton(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var handler, label, style, url string
	var disabled bool
	if err := starlarkgo.UnpackArgs("button", args, kwargs, "handler?", &handler, "label?", &label, "style?", &style, "url?", &url, "disabled?", &disabled); err != nil {
		return nil, err
	}
	if style == "" {
		style = "primary"
	}
	return &apiValue{kind: apiButton, data: &contract.Button{Handler: handler, Label: label, Style: contract.ButtonStyle(style), URL: url, Disabled: disabled}}, nil
}
func builtinSelectOption(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var label, value, description string
	var defaultValue bool
	if err := starlarkgo.UnpackArgs("select_option", args, kwargs,
		"label", &label,
		"value", &value,
		"description?", &description,
		"default?", &defaultValue,
	); err != nil {
		return nil, err
	}
	return &apiValue{kind: apiSelectOption, data: contract.SelectOption{Label: label, Value: value, Description: description, Default: defaultValue}}, nil
}

func builtinSelect(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var handler, kind, placeholder string
	var minValues, maxValues int
	var disabled bool
	var optionsValue, channelKindsValue starlarkgo.Value
	if err := starlarkgo.UnpackArgs("select", args, kwargs,
		"handler", &handler,
		"kind", &kind,
		"placeholder?", &placeholder,
		"min_values?", &minValues,
		"max_values?", &maxValues,
		"disabled?", &disabled,
		"options?", &optionsValue,
		"channel_kinds?", &channelKindsValue,
	); err != nil {
		return nil, err
	}
	optionValues, err := apiValueList(optionsValue, apiSelectOption)
	if err != nil {
		return nil, fmt.Errorf("options: %w", err)
	}
	channelKinds, err := stringList(channelKindsValue)
	if err != nil {
		return nil, fmt.Errorf("channel_kinds: %w", err)
	}
	selectMenu := &contract.Select{
		Handler:      handler,
		Kind:         contract.SelectKind(kind),
		Placeholder:  placeholder,
		MinValues:    minValues,
		MaxValues:    maxValues,
		Disabled:     disabled,
		Options:      make([]contract.SelectOption, len(optionValues)),
		ChannelKinds: make([]contract.ChannelKind, len(channelKinds)),
	}
	for index, value := range optionValues {
		selectMenu.Options[index] = value.data.(contract.SelectOption)
	}
	for index, value := range channelKinds {
		selectMenu.ChannelKinds[index] = contract.ChannelKind(value)
	}
	return &apiValue{kind: apiSelect, data: selectMenu}, nil
}

func builtinRow(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var components starlarkgo.Value
	if err := starlarkgo.UnpackArgs("row", args, kwargs, "components", &components); err != nil {
		return nil, err
	}
	if components == nil || components == starlarkgo.None {
		return nil, errors.New("row components are required")
	}
	iterable, ok := components.(starlarkgo.Iterable)
	if !ok {
		return nil, fmt.Errorf("row components must be iterable, got %s", components.Type())
	}
	iterator := iterable.Iterate()
	defer iterator.Done()
	row := contract.ComponentRow{}
	var item starlarkgo.Value
	for iterator.Next(&item) {
		value, ok := item.(*apiValue)
		if !ok || value == nil {
			return nil, fmt.Errorf("row component must be a button or select, got %s", item.Type())
		}
		switch value.kind {
		case apiButton:
			button := *(value.data.(*contract.Button))
			row.Components = append(row.Components, &button)
		case apiSelect:
			source := value.data.(*contract.Select)
			selectMenu := *source
			selectMenu.Options = append([]contract.SelectOption(nil), source.Options...)
			selectMenu.ChannelKinds = append([]contract.ChannelKind(nil), source.ChannelKinds...)
			row.Components = append(row.Components, &selectMenu)
		default:
			return nil, fmt.Errorf("row component must be a button or select, got %s", item.Type())
		}
		if len(row.Components) > 5 {
			return nil, errors.New("row exceeds 5 components")
		}
	}
	return &apiValue{kind: apiRow, data: row}, nil
}

func builtinEmbedField(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var name, value string
	var inline bool
	if err := starlarkgo.UnpackArgs("embed_field", args, kwargs, "name", &name, "value", &value, "inline?", &inline); err != nil {
		return nil, err
	}
	return &apiValue{kind: apiEmbedField, data: contract.EmbedField{Name: name, Value: value, Inline: inline}}, nil
}

func builtinEmbedAuthor(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var name, url, iconURL string
	if err := starlarkgo.UnpackArgs("embed_author", args, kwargs, "name", &name, "url?", &url, "icon_url?", &iconURL); err != nil {
		return nil, err
	}
	return &apiValue{kind: apiEmbedAuthor, data: contract.EmbedAuthor{Name: name, URL: url, IconURL: iconURL}}, nil
}

func builtinEmbedFooter(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var text, iconURL string
	if err := starlarkgo.UnpackArgs("embed_footer", args, kwargs, "text", &text, "icon_url?", &iconURL); err != nil {
		return nil, err
	}
	return &apiValue{kind: apiEmbedFooter, data: contract.EmbedFooter{Text: text, IconURL: iconURL}}, nil
}

func builtinEmbed(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var title, description, url, imageURL, thumbnailURL string
	var color int
	var fieldsValue, authorRaw, footerRaw starlarkgo.Value
	if err := starlarkgo.UnpackArgs("embed", args, kwargs,
		"title?", &title,
		"description?", &description,
		"url?", &url,
		"color?", &color,
		"fields?", &fieldsValue,
		"author?", &authorRaw,
		"footer?", &footerRaw,
		"image_url?", &imageURL,
		"thumbnail_url?", &thumbnailURL,
	); err != nil {
		return nil, err
	}
	authorValue, err := optionalAPIValue(authorRaw, apiEmbedAuthor)
	if err != nil {
		return nil, fmt.Errorf("author: %w", err)
	}
	footerValue, err := optionalAPIValue(footerRaw, apiEmbedFooter)
	if err != nil {
		return nil, fmt.Errorf("footer: %w", err)
	}
	fieldValues, err := apiValueList(fieldsValue, apiEmbedField)
	if err != nil {
		return nil, fmt.Errorf("fields: %w", err)
	}
	embed := contract.Embed{Title: title, Description: description, URL: url, Color: color, ImageURL: imageURL, ThumbnailURL: thumbnailURL, Fields: make([]contract.EmbedField, len(fieldValues))}
	for index, value := range fieldValues {
		embed.Fields[index] = value.data.(contract.EmbedField)
	}
	if authorValue != nil {
		if authorValue.kind != apiEmbedAuthor {
			return nil, errors.New("author must be mamacord.embed_author")
		}
		author := authorValue.data.(contract.EmbedAuthor)
		embed.Author = &author
	}
	if footerValue != nil {
		if footerValue.kind != apiEmbedFooter {
			return nil, errors.New("footer must be mamacord.embed_footer")
		}
		footer := footerValue.data.(contract.EmbedFooter)
		embed.Footer = &footer
	}
	return &apiValue{kind: apiEmbed, data: embed}, nil
}

func builtinTextInput(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var id, label, style, placeholder, value string
	var required bool
	var minLength, maxLength int
	if err := starlarkgo.UnpackArgs("text_input", args, kwargs, "id", &id, "label", &label, "style?", &style, "required?", &required, "placeholder?", &placeholder, "value?", &value, "min_length?", &minLength, "max_length?", &maxLength); err != nil {
		return nil, err
	}
	if style == "" {
		style = "short"
	}
	field := contract.TextInput{ID: id, Label: label, Style: contract.TextInputStyle(style), Required: required, Placeholder: placeholder, Value: value, MinLength: minLength, MaxLength: maxLength}
	return &apiValue{kind: apiTextInput, data: field}, nil
}
func builtinModalView(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var handler, title string
	var fields starlarkgo.Value
	if err := starlarkgo.UnpackArgs("modal_view", args, kwargs, "handler", &handler, "title", &title, "fields", &fields); err != nil {
		return nil, err
	}
	values, err := apiValueList(fields, apiTextInput)
	if err != nil {
		return nil, err
	}
	modal := contract.ModalView{Handler: handler, Title: title, Fields: make([]contract.TextInput, len(values))}
	for i, value := range values {
		modal.Fields[i] = value.data.(contract.TextInput)
	}
	return &apiValue{kind: apiModalView, data: modal}, nil
}
func builtinShowModal(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var view *apiValue
	if err := starlarkgo.UnpackArgs("show_modal", args, kwargs, "view", &view); err != nil {
		return nil, err
	}
	if view == nil || view.kind != apiModalView {
		return nil, fmt.Errorf("view must be mamacord.modal_view")
	}
	return effectValue(effectModal, view.data.(contract.ModalView)), nil
}
func builtinKVPut(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var key string
	var value, expectedValue starlarkgo.Value
	if err := starlarkgo.UnpackArgs("kv_put", args, kwargs, "key", &key, "value", &value, "expected_version?", &expectedValue); err != nil {
		return nil, err
	}
	persistent, err := lowerPersistentValue(value)
	if err != nil {
		return nil, err
	}
	if err := persistent.ValidateState(); err != nil {
		return nil, err
	}
	expected, err := optionalUint64(expectedValue)
	if err != nil {
		return nil, err
	}
	return effectValue(effectKVPut, kvPutDeclaration{key: key, value: persistent, expectedVersion: expected}), nil
}
func builtinKVDelete(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var key string
	var expectedValue starlarkgo.Value
	if err := starlarkgo.UnpackArgs("kv_delete", args, kwargs, "key", &key, "expected_version?", &expectedValue); err != nil {
		return nil, err
	}
	expected, err := optionalUint64(expectedValue)
	if err != nil {
		return nil, err
	}
	return effectValue(effectKVDelete, struct {
		key      string
		expected *uint64
	}{key: key, expected: expected}), nil
}
func builtinAutocompleteChoice(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var name string
	var value starlarkgo.Value
	if err := starlarkgo.UnpackArgs("autocomplete_choice", args, kwargs, "name", &name, "value", &value); err != nil {
		return nil, err
	}
	choice := contract.AutocompleteChoice{Name: name}
	switch typed := value.(type) {
	case starlarkgo.String:
		choice.Value = contract.ChoiceValue{Kind: contract.ChoiceString, String: string(typed)}
	case starlarkgo.Int:
		integer, ok := typed.Int64()
		if !ok {
			return nil, fmt.Errorf("integer outside int64")
		}
		choice.Value = contract.ChoiceValue{Kind: contract.ChoiceInteger, Integer: integer}
	case starlarkgo.Float:
		choice.Value = contract.ChoiceValue{Kind: contract.ChoiceNumber, Number: float64(typed)}
	default:
		return nil, fmt.Errorf("unsupported autocomplete value %s", value.Type())
	}
	return &apiValue{kind: apiAutoChoice, data: choice}, nil
}
func builtinAutocompleteChoices(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var choices starlarkgo.Value
	if err := starlarkgo.UnpackArgs("autocomplete_choices", args, kwargs, "choices", &choices); err != nil {
		return nil, err
	}
	values, err := apiValueList(choices, apiAutoChoice)
	if err != nil {
		return nil, err
	}
	out := make([]contract.AutocompleteChoice, len(values))
	for i, value := range values {
		out[i] = value.data.(contract.AutocompleteChoice)
	}
	return effectValue(effectAutocomplete, autocompleteDeclaration{choices: out}), nil
}
func effectValue(kind effectKind, data any) *apiValue {
	return &apiValue{kind: apiEffect, data: effectDeclaration{kind: kind, data: data}}
}
func messageRows(value starlarkgo.Value) ([]contract.ComponentRow, error) {
	values, err := apiValueList(value, apiRow)
	if err != nil {
		return nil, err
	}
	rows := make([]contract.ComponentRow, len(values))
	for i, item := range values {
		rows[i] = item.data.(contract.ComponentRow).DeepClone()
	}
	return rows, nil
}
func messageEmbeds(value starlarkgo.Value) ([]contract.Embed, error) {
	values, err := apiValueList(value, apiEmbed)
	if err != nil {
		return nil, err
	}
	embeds := make([]contract.Embed, len(values))
	for index, item := range values {
		embeds[index] = item.data.(contract.Embed).DeepClone()
	}
	return embeds, nil
}

func optionalUint64(value starlarkgo.Value) (*uint64, error) {
	if value == nil || value == starlarkgo.None {
		return nil, nil
	}
	integer, ok := value.(starlarkgo.Int)
	if !ok {
		return nil, fmt.Errorf("expected_version must be int or None")
	}
	raw, ok := integer.Uint64()
	if !ok {
		return nil, errors.New("expected_version is outside uint64")
	}
	return &raw, nil
}

func optionalAPIValue(value starlarkgo.Value, kind apiValueKind) (*apiValue, error) {
	if value == nil || value == starlarkgo.None {
		return nil, nil
	}
	typed, ok := value.(*apiValue)
	if !ok || typed == nil || typed.kind != kind {
		return nil, fmt.Errorf("want mamacord.%s, got %s", kind, value.Type())
	}
	return typed, nil
}
