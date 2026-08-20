package contract

import (
	"strings"
	"testing"
)

func TestInvocationValidationAndDeepClone(t *testing.T) {
	t.Parallel()

	invocation := validInvocation()
	if err := invocation.Validate(); err != nil {
		t.Fatalf("valid invocation rejected: %v", err)
	}
	clone := invocation.DeepClone()
	clone.Command.Path[0] = "changed"
	clone.Command.Options[0].String = "changed"
	if invocation.Command.Path[0] != "tool" {
		t.Fatal("command path was aliased")
	}
	if invocation.Command.Options[0].String != "hello" {
		t.Fatal("option value was aliased")
	}

	invalid := invocation
	invalid.Component = &ComponentInput{ID: "extra"}
	if err := invalid.Validate(); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Fatalf("expected input-count error, got %v", err)
	}
	invalid = invocation
	invalid.Kind = InvocationModal
	if err := invalid.Validate(); err == nil || !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("expected kind mismatch, got %v", err)
	}
}

func TestCheckDecision(t *testing.T) {
	t.Parallel()

	if err := AllowedCheck().Validate(); err != nil {
		t.Fatalf("allowed check rejected: %v", err)
	}
	if err := SilentDeniedCheck().Validate(); err != nil {
		t.Fatalf("silent denial rejected: %v", err)
	}
	denial := MessageOperation{Message: Message{Content: "denied"}, Ephemeral: true}
	decision := DeniedCheck(&denial)
	if err := decision.Validate(); err != nil {
		t.Fatalf("denied check rejected: %v", err)
	}
	clone := decision.DeepClone()
	clone.Denial.Message.Content = "changed"
	if decision.Denial.Message.Content != "denied" {
		t.Fatal("check denial was aliased")
	}
	invalid := CheckDecision{Kind: CheckAllowed, Denial: &denial}
	if err := invalid.Validate(); err == nil {
		t.Fatal("allowed check accepted a denial reply")
	}
	if err := (CheckDecision{Kind: CheckDeniedMessage}).Validate(); err == nil {
		t.Fatal("message denial accepted without reply")
	}
}

func TestOutcomeInvocationModes(t *testing.T) {
	t.Parallel()

	command := validInvocation()
	update := &UpdateOperation{Patch: MessagePatch{Content: OptionalString{Set: true, Value: "too early"}}}
	if err := (Outcome{Operations: []Operation{update}}).Validate(command); err == nil {
		t.Fatal("command accepted message update")
	}
	command.ResponseState = ResponseDeferredCreate
	edit := &EditResponseOperation{Patch: MessagePatch{Content: OptionalString{Set: true, Value: "done"}}}
	if err := (Outcome{Operations: []Operation{edit}}).Validate(command); err != nil {
		t.Fatalf("deferred command completion rejected: %v", err)
	}

	autocomplete := Invocation{InvocationIdentity: InvocationIdentity{PluginID: "example", Generation: "generation-1", Route: "autocomplete:query", Kind: InvocationAutocomplete}, InvocationInteractionContext: InvocationInteractionContext{ResponseState: ResponseUnacknowledged}, InvocationActorContext: InvocationActorContext{Author: &UserRef{ID: "30", Username: "alice"}}, InvocationInput: InvocationInput{Autocomplete: &AutocompleteInput{Path: []string{"search"}, Option: "query", Focused: OptionValue{Name: "query", Kind: OptionString, ScalarOptionValue: ScalarOptionValue{String: "ma"}}}}}
	choices := Outcome{Operations: []Operation{&AutocompleteChoicesOperation{Choices: []AutocompleteChoice{{Name: "Mamacord", Value: ChoiceValue{Kind: ChoiceString, String: "mamacord"}}}}}}
	if err := choices.Validate(autocomplete); err != nil {
		t.Fatalf("autocomplete choices rejected: %v", err)
	}
	wrongKind := Outcome{Operations: []Operation{&AutocompleteChoicesOperation{Choices: []AutocompleteChoice{{Name: "One", Value: ChoiceValue{Kind: ChoiceInteger, Integer: 1}}}}}}
	if err := wrongKind.Validate(autocomplete); err == nil {
		t.Fatal("autocomplete accepted mismatched choice kind")
	}
	if err := (Outcome{Operations: []Operation{&MessageOperation{Message: Message{Content: "wrong"}}}}).Validate(autocomplete); err == nil {
		t.Fatal("autocomplete accepted interaction message")
	}

	event := Invocation{InvocationIdentity: InvocationIdentity{PluginID: "example", Generation: "generation-1", Route: "event:join", Kind: InvocationEvent}, InvocationInput: InvocationInput{Event: &EventInput{Name: "guild_member_join", Data: NullValue()}}}
	if err := (Outcome{}).Validate(event); err != nil {
		t.Fatalf("empty event outcome rejected: %v", err)
	}
	if err := (Outcome{Operations: []Operation{&MessageOperation{Message: Message{Content: "wrong"}}}}).Validate(event); err == nil {
		t.Fatal("event accepted interaction reply")
	}
}

func validInvocation() Invocation {
	return Invocation{InvocationIdentity: InvocationIdentity{PluginID: "example",
		Generation: "generation-1",
		Route:      "command:tool:create",
		Kind:       InvocationCommand}, InvocationInteractionContext: InvocationInteractionContext{ResponseState: ResponseUnacknowledged}, InvocationActorContext: InvocationActorContext{Guild: &GuildRef{ID: "10", Name: "Guild"},
		Channel: &ChannelRef{ID: "20", GuildID: "10", Name: "general", Kind: ChannelText},
		Author:  &UserRef{ID: "30", Username: "alice"},
		Locale:  "en-US"}, InvocationInput: InvocationInput{Command: &CommandInput{
		Kind:    CommandSlash,
		Path:    []string{"tool", "create"},
		Options: []OptionValue{{Name: "text", Kind: OptionString, ScalarOptionValue: ScalarOptionValue{String: "hello"}}, {Name: "count", Kind: OptionInteger, ScalarOptionValue: ScalarOptionValue{Integer: 2}}},
	}},
	}
}
