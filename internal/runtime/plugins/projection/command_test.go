package projection

import (
	"encoding/json"
	"testing"
)

func TestCommandOptionJSONRemainsFlat(t *testing.T) {
	t.Parallel()
	minimum := 2
	data, err := json.Marshal(CommandOption{
		OptionPresentation: OptionPresentation{Name: "query", Type: "string", Description: "Query", Required: true},
		OptionBounds:       OptionBounds{MinLength: &minimum},
		ChannelTypes:       []int{0},
	})
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"name", "type", "description", "required", "min_length", "channel_types"} {
		if _, ok := payload[key]; !ok {
			t.Fatalf("missing flat key %q in %s", key, data)
		}
	}
	for _, key := range []string{"OptionPresentation", "OptionBounds"} {
		if _, ok := payload[key]; ok {
			t.Fatalf("unexpected composition key %q in %s", key, data)
		}
	}
}
