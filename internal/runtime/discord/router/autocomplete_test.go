package router

import (
	"testing"

	"github.com/disgoorg/disgo/discord"

	luaplugin "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

func TestParsePluginAutocompleteChoices(t *testing.T) {
	t.Parallel()

	choices, err := ParsePluginAutocompleteChoices("test", luaplugin.EncodedValue(`[
		{"name":"alpha","value":"a"},
		{"name":"beta","value":2},
		{"name":"gamma","value":2.5}
	]`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(choices) != 3 {
		t.Fatalf("unexpected choice count: got %d want 3", len(choices))
	}
	if _, ok := choices[0].(discord.AutocompleteChoiceString); !ok {
		t.Fatalf("expected first choice to be string, got %T", choices[0])
	}
	if _, ok := choices[1].(discord.AutocompleteChoiceInt); !ok {
		t.Fatalf("expected second choice to be int, got %T", choices[1])
	}
	if _, ok := choices[2].(discord.AutocompleteChoiceFloat); !ok {
		t.Fatalf("expected third choice to be float, got %T", choices[2])
	}
}

func TestParsePluginAutocompleteChoicesFromObject(t *testing.T) {
	t.Parallel()

	choices, err := ParsePluginAutocompleteChoices("test", luaplugin.EncodedValue(`{"choices":[{"name":"delta","value":"d"}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(choices) != 1 {
		t.Fatalf("unexpected choice count: got %d want 1", len(choices))
	}
}

func TestParsePluginAutocompleteChoicesFromEncodedJSON(t *testing.T) {
	t.Parallel()

	choices, err := ParsePluginAutocompleteChoices("test", luaplugin.EncodedValue(`{"choices":[{"name":"echo","value":"e"}]}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(choices) != 1 {
		t.Fatalf("unexpected choice count: got %d want 1", len(choices))
	}
	if _, ok := choices[0].(discord.AutocompleteChoiceString); !ok {
		t.Fatalf("expected first choice to be string, got %T", choices[0])
	}
}
