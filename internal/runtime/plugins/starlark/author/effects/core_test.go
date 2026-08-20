package effects

import (
	"reflect"
	"sort"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

func TestAPIContract(t *testing.T) {
	expected := []string{
		"add_role", "append_audit", "attempt", "autocomplete_choice", "autocomplete_choices",
		"best_effort", "button", "clear_timezone", "create_checkin", "create_emoji",
		"create_reminder", "create_role", "create_sticker", "create_warning", "delete_emoji",
		"delete_reminder", "delete_role", "delete_sticker", "delete_warning", "edit_emoji",
		"edit_role", "edit_sticker", "embed", "embed_author", "embed_field", "embed_footer",
		"kv_delete", "kv_put", "modal_view", "purge_messages", "remove_role", "reply", "row",
		"select", "select_option", "send_channel", "send_dm", "set_nickname", "set_slowmode",
		"set_timezone", "show_modal", "text_input", "timeout_member", "update",
	}

	api := API()
	actual := make([]string, 0, len(api))
	for name, raw := range api {
		actual = append(actual, name)
		builtin, ok := raw.(*starlarkgo.Builtin)
		if !ok {
			t.Fatalf("API value %q has type %T, want *starlark.Builtin", name, raw)
		}
		if builtin.Name() != name {
			t.Errorf("builtin %q reports name %q", name, builtin.Name())
		}
	}
	sort.Strings(actual)
	if !reflect.DeepEqual(actual, expected) {
		t.Fatalf("effect API names = %v, want %v", actual, expected)
	}
}

func TestLowerOutcomeContracts(t *testing.T) {
	reply := callBuiltin(t, "reply", starlarkgo.Tuple{starlarkgo.String("hello"), starlarkgo.True})
	dm := callBuiltin(t, "send_dm", starlarkgo.Tuple{starlarkgo.String("user-1"), starlarkgo.String("notice")})
	bestEffortDM := callBuiltin(t, "best_effort", starlarkgo.Tuple{dm})

	outcome, err := LowerOutcome(
		starlarkgo.NewList([]starlarkgo.Value{reply, bestEffortDM}),
		contract.Invocation{InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}},
	)
	if err != nil {
		t.Fatalf("LowerOutcome() error = %v", err)
	}
	if len(outcome.Operations) != 2 {
		t.Fatalf("operation count = %d, want 2", len(outcome.Operations))
	}
	message, ok := outcome.Operations[0].(*contract.MessageOperation)
	if !ok {
		t.Fatalf("first operation has type %T, want *contract.MessageOperation", outcome.Operations[0])
	}
	if message.Message.Content != "hello" || !message.Ephemeral {
		t.Fatalf("reply operation = %#v", message)
	}
	bestEffort, ok := outcome.Operations[1].(*contract.BestEffortOperation)
	if !ok {
		t.Fatalf("second operation has type %T, want *contract.BestEffortOperation", outcome.Operations[1])
	}
	sendDM, ok := bestEffort.Operation.(*contract.SendDMOperation)
	if !ok {
		t.Fatalf("best-effort operation has type %T, want *contract.SendDMOperation", bestEffort.Operation)
	}
	if sendDM.UserID != "user-1" || sendDM.Message.Content != "notice" {
		t.Fatalf("send DM operation = %#v", sendDM)
	}
}

func TestLowerCheckDecisionReplyContract(t *testing.T) {
	reply := callBuiltin(t, "reply", starlarkgo.Tuple{starlarkgo.String("denied"), starlarkgo.True})
	decision, err := LowerCheckDecision(reply)
	if err != nil {
		t.Fatalf("LowerCheckDecision() error = %v", err)
	}
	if decision.Kind != contract.CheckDeniedMessage || decision.Denial == nil {
		t.Fatalf("decision = %#v, want denial", decision)
	}
	if decision.Denial.Message.Content != "denied" || !decision.Denial.Ephemeral {
		t.Fatalf("denial = %#v", decision.Denial)
	}
}

func callBuiltin(t *testing.T, name string, args starlarkgo.Tuple) starlarkgo.Value {
	t.Helper()
	raw, ok := API()[name]
	if !ok {
		t.Fatalf("builtin %q is absent", name)
	}
	result, err := starlarkgo.Call(&starlarkgo.Thread{Name: "effects-test"}, raw, args, nil)
	if err != nil {
		t.Fatalf("%s() error = %v", name, err)
	}
	return result
}
