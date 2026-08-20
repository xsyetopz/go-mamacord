package contract

import (
	"testing"
)

func TestRouteCatalogRejectsSignatureMismatches(t *testing.T) {
	t.Parallel()

	catalog, err := validDefinition().Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	invocation := validInvocation()
	signature, err := catalog.Resolve(invocation)
	if err != nil {
		t.Fatalf("Resolve command: %v", err)
	}
	if signature.Type != RouteCommand || signature.CommandKind != CommandSlash {
		t.Fatalf("unexpected signature: %#v", signature)
	}

	tests := []struct {
		name   string
		mutate func(*Invocation)
	}{
		{name: "path", mutate: func(value *Invocation) { value.Command.Path[1] = "other" }},
		{name: "kind", mutate: func(value *Invocation) {
			value.Command.Kind = CommandUser
			value.Command.TargetUser = &UserRef{ID: "40"}
			value.Command.Options = nil
		}},
		{name: "extra option", mutate: func(value *Invocation) {
			value.Command.Options = append(value.Command.Options, OptionValue{Name: "extra", Kind: OptionString})
		}},
		{name: "missing required", mutate: func(value *Invocation) { value.Command.Options = value.Command.Options[1:] }},
		{name: "integer bound", mutate: func(value *Invocation) { value.Command.Options[1].Integer = 11 }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			value := invocation.DeepClone()
			test.mutate(&value)
			if _, err := catalog.Resolve(value); err == nil {
				t.Fatal("signature mismatch accepted")
			}
		})
	}

	component := Invocation{InvocationIdentity: InvocationIdentity{PluginID: "example", Generation: "generation-1", Route: "component:save", Kind: InvocationComponent}, InvocationInteractionContext: InvocationInteractionContext{ResponseState: ResponseUnacknowledged}, InvocationActorContext: InvocationActorContext{Guild: &GuildRef{ID: "10"}, Author: &UserRef{ID: "30"}}, InvocationInput: InvocationInput{Component: &ComponentInput{ID: "save", Kind: ComponentButton}}}
	if _, err := catalog.Resolve(component); err != nil {
		t.Fatalf("Resolve component: %v", err)
	}
	component.Component.Kind = ComponentStringSelect
	if _, err := catalog.Resolve(component); err == nil {
		t.Fatal("component kind mismatch accepted")
	}

	modal := Invocation{InvocationIdentity: InvocationIdentity{PluginID: "example", Generation: "generation-1", Route: "modal:edit", Kind: InvocationModal}, InvocationInteractionContext: InvocationInteractionContext{ResponseState: ResponseUnacknowledged, ModalOrigin: ModalOriginCommand}, InvocationActorContext: InvocationActorContext{Author: &UserRef{ID: "30"}}, InvocationInput: InvocationInput{Modal: &ModalInput{ID: "edit", Fields: []NamedString{{Name: "text", Value: "new"}}}}}
	if _, err := catalog.Resolve(modal); err != nil {
		t.Fatalf("Resolve modal: %v", err)
	}
	modal.Modal.Fields[0].Name = "unknown"
	if _, err := catalog.Resolve(modal); err == nil {
		t.Fatal("undeclared modal field accepted")
	}

	autocomplete := Invocation{InvocationIdentity: InvocationIdentity{PluginID: "example", Generation: "generation-1", Route: "autocomplete:tool:count", Kind: InvocationAutocomplete}, InvocationInteractionContext: InvocationInteractionContext{ResponseState: ResponseUnacknowledged}, InvocationActorContext: InvocationActorContext{Author: &UserRef{ID: "30"}}, InvocationInput: InvocationInput{Autocomplete: &AutocompleteInput{Path: []string{"tool", "create"}, Option: "count", Focused: OptionValue{Name: "count", Kind: OptionInteger, ScalarOptionValue: ScalarOptionValue{Integer: 2}}, Options: []OptionValue{{Name: "text", Kind: OptionString, ScalarOptionValue: ScalarOptionValue{String: "hello"}}}}}}
	if _, err := catalog.Resolve(autocomplete); err != nil {
		t.Fatalf("Resolve autocomplete: %v", err)
	}
	autocomplete.Autocomplete.Focused.Kind = OptionString
	if _, err := catalog.Resolve(autocomplete); err == nil {
		t.Fatal("autocomplete kind mismatch accepted")
	}
}

func TestCatalogValidatesOutputHandlers(t *testing.T) {
	t.Parallel()
	catalog, err := validDefinition().Compile()
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	invocation := validInvocation()
	message := Message{Content: "choose", Components: []ComponentRow{{Components: []MessageComponent{&Button{Handler: "save", Label: "Save", Style: ButtonPrimary}}}}}
	outcome := Outcome{Operations: []Operation{&MessageOperation{Message: message}}}
	if err := catalog.ValidateOutcome(invocation, outcome); err != nil {
		t.Fatalf("declared handler rejected: %v", err)
	}
	message.Components[0].Components[0].(*Button).Handler = "missing"
	if err := catalog.ValidateOutcome(invocation, Outcome{Operations: []Operation{&MessageOperation{Message: message}}}); err == nil {
		t.Fatal("undeclared component handler accepted")
	}

	modal := ModalView{Handler: "edit", Title: "Edit", Fields: []TextInput{{ID: "text", Label: "Text", Style: TextInputShort, Required: true}}}
	if err := catalog.ValidateOutcome(invocation, Outcome{Operations: []Operation{&ModalOperation{Modal: modal}}}); err != nil {
		t.Fatalf("declared modal rejected: %v", err)
	}
	modal.Fields[0].ID = "other"
	if err := catalog.ValidateOutcome(invocation, Outcome{Operations: []Operation{&ModalOperation{Modal: modal}}}); err == nil {
		t.Fatal("undeclared modal field accepted")
	}
}
