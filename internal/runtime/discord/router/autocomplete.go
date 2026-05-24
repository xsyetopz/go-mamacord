package router

import (
	"fmt"

	"github.com/disgoorg/disgo/discord"

	luaplugin "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

func ParsePluginAutocompleteChoices(_ string, raw luaplugin.EncodedValue) ([]discord.AutocompleteChoice, error) {
	value, err := raw.Decode()
	if err != nil {
		return nil, fmt.Errorf("decode autocomplete response: %w", err)
	}

	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []any:
		return autocompleteChoicesFromList(typed)
	case map[string]any:
		if nested, ok := typed["choices"]; ok {
			list, ok := nested.([]any)
			if !ok {
				return nil, fmt.Errorf("choices must be an array")
			}
			return autocompleteChoicesFromList(list)
		}
		return nil, fmt.Errorf("unsupported autocomplete response object")
	default:
		return nil, fmt.Errorf("unsupported autocomplete response type %T", value)
	}
}

func autocompleteChoicesFromList(list []any) ([]discord.AutocompleteChoice, error) {
	out := make([]discord.AutocompleteChoice, 0, len(list))
	for idx, item := range list {
		choiceMap, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("choice %d must be an object", idx+1)
		}
		name, ok := asString(choiceMap, "name")
		if !ok || name == "" {
			return nil, fmt.Errorf("choice %d missing name", idx+1)
		}
		value, ok := choiceMap["value"]
		if !ok {
			return nil, fmt.Errorf("choice %q missing value", name)
		}
		switch typed := value.(type) {
		case string:
			out = append(out, discord.AutocompleteChoiceString{Name: name, Value: typed})
		case float64:
			if typed == float64(int(typed)) {
				out = append(out, discord.AutocompleteChoiceInt{Name: name, Value: int(typed)})
			} else {
				out = append(out, discord.AutocompleteChoiceFloat{Name: name, Value: typed})
			}
		case int:
			out = append(out, discord.AutocompleteChoiceInt{Name: name, Value: typed})
		default:
			return nil, fmt.Errorf("choice %q has unsupported value type %T", name, value)
		}
	}
	return out, nil
}

func asString(m map[string]any, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok {
		return "", false
	}
	return s, true
}
