package plugin

import (
	"testing"

	luaplugin "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

func TestParseAutomationActionsFromEncodedJSON(t *testing.T) {
	t.Parallel()

	actions, err := ParseAutomationActions(luaplugin.EncodedValue(`{
		"actions": [
			{
				"type": "send_dm",
				"user_id": "42",
				"message": {
					"content": "hi"
				}
			}
		]
	}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actions) != 1 {
		t.Fatalf("unexpected action count: got %d want 1", len(actions))
	}
	if actions[0].Type != "send_dm" || actions[0].UserID != "42" {
		t.Fatalf("unexpected action: %#v", actions[0])
	}

	msg, err := actions[0].Message.Decode()
	if err != nil {
		t.Fatalf("decode nested message: %v", err)
	}
	msgMap, ok := msg.(map[string]any)
	if !ok {
		t.Fatalf("expected nested message object, got %T", msg)
	}
	if msgMap["content"] != "hi" {
		t.Fatalf("unexpected nested message: %#v", msgMap)
	}
}

func TestParseAutomationActionsRejectsInvalidEncodedJSON(t *testing.T) {
	t.Parallel()

	_, err := ParseAutomationActions(luaplugin.EncodedValue(`{"actions":`))
	if err == nil {
		t.Fatal("expected invalid encoded json to fail")
	}
}
