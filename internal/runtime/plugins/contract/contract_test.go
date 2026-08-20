package contract

import (
	"math"
	"strings"
	"testing"
)

func TestValueKindsCloneAndLimits(t *testing.T) {
	t.Parallel()

	finite, err := FloatValue(1.5)
	if err != nil {
		t.Fatalf("FloatValue: %v", err)
	}
	object, err := ObjectValue([]Field{
		{Key: "null", Value: NullValue()},
		{Key: "bool", Value: BoolValue(true)},
		{Key: "int", Value: IntValue(42)},
		{Key: "float", Value: finite},
		{Key: "string", Value: StringValue("hello")},
	})
	if err != nil {
		t.Fatalf("ObjectValue: %v", err)
	}
	list, err := ListValue([]Value{object, StringValue("tail")})
	if err != nil {
		t.Fatalf("ListValue: %v", err)
	}
	if err := list.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	values, ok := list.List()
	if !ok || len(values) != 2 {
		t.Fatalf("unexpected list: %#v, %t", values, ok)
	}
	fields, ok := values[0].Object()
	if !ok || len(fields) != 5 || fields[0].Key != "null" || fields[4].Key != "string" {
		t.Fatalf("object order changed: %#v", fields)
	}
	fields[0].Key = "mutated"
	again, _ := list.List()
	againFields, _ := again[0].Object()
	if againFields[0].Key != "null" {
		t.Fatal("Object exposed mutable internal fields")
	}

	largeFloat, err := FloatValue(1e20)
	if err != nil {
		t.Fatalf("FloatValue(1e20): %v", err)
	}
	if size, err := largeFloat.EncodedSize(); err != nil || size != 21 {
		t.Fatalf("float JSON size: got %d, err %v", size, err)
	}
	if _, err := FloatValue(math.NaN()); err == nil {
		t.Fatal("expected nonfinite float error")
	}
	if _, err := ObjectValue([]Field{{Key: "x", Value: NullValue()}, {Key: "x", Value: NullValue()}}); err == nil {
		t.Fatal("expected duplicate object key error")
	}
	tooMany := make([]Value, MaxValueItems+1)
	for index := range tooMany {
		tooMany[index] = NullValue()
	}
	if _, err := ListValue(tooMany); err == nil {
		t.Fatal("expected aggregate item limit error")
	}
	if err := StringValue(strings.Repeat("x", MaxStateValueBytes)).ValidateState(); err == nil {
		t.Fatal("expected encoded size error")
	}

	deep := NullValue()
	for range MaxValueDepth + 1 {
		deep = Value{kind: ValueList, list: []Value{deep}}
	}
	if err := deep.Validate(); err == nil {
		t.Fatal("expected depth limit error")
	}
}

func TestDefinitionValidationAndDeepClone(t *testing.T) {
	t.Parallel()

	definition := validDefinition()
	if err := definition.Validate(); err != nil {
		t.Fatalf("valid definition rejected: %v", err)
	}
	definition.Cogs[0].Commands = append(definition.Cogs[0].Commands, CommandDefinition{Kind: CommandUser, Name: "tool", Route: "command:user:tool"})
	if err := definition.Validate(); err != nil {
		t.Fatalf("same name in another command kind rejected: %v", err)
	}
	clone := definition.DeepClone()
	clone.Cogs[0].Commands[0].Children[0].Options[0].Name = "changed"
	clone.Cogs[0].Components[0].Checks[0].Permissions[0] = "changed"
	if definition.Cogs[0].Commands[0].Children[0].Options[0].Name != "text" {
		t.Fatal("command option was aliased")
	}
	if definition.Cogs[0].Components[0].Checks[0].Permissions[0] != "manage_messages" {
		t.Fatal("check permissions were aliased")
	}
}

func TestDefinitionRejectsInvalidShapes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		mutate   func(*Definition)
		contains string
	}{
		{name: "duplicate route", mutate: func(d *Definition) { d.Cogs[0].Components[0].Route = "command:tool:create" }, contains: "duplicate route"},
		{name: "required after optional", mutate: func(d *Definition) {
			options := d.Cogs[0].Commands[0].Children[0].Options
			options[0].Required = false
			options[1].Required = true
		}, contains: "follows an optional"},
		{name: "nested group", mutate: func(d *Definition) {
			d.Cogs[0].Commands[0].Children[0].Route = ""
			d.Cogs[0].Commands[0].Children[0].Options = nil
			d.Cogs[0].Commands[0].Children[0].Children = []CommandDefinition{{Kind: CommandGroup, Name: "nested", Description: "nested", Children: []CommandDefinition{{Kind: CommandSubcommand, Name: "leaf", Description: "leaf", Route: "nested"}}}}
		}, contains: "cannot contain children"},
		{name: "container route", mutate: func(d *Definition) { d.Cogs[0].Commands[0].Route = "container" }, contains: "cannot have a route"},
		{name: "duplicate component", mutate: func(d *Definition) { d.Cogs[0].Components = append(d.Cogs[0].Components, d.Cogs[0].Components[0]) }, contains: "duplicate component"},
		{name: "choice kind", mutate: func(d *Definition) {
			d.Cogs[0].Commands[0].Children[0].Options[0].Choices = []ChoiceDefinition{{Name: "one", Value: ChoiceValue{Kind: ChoiceInteger, Integer: 1}}}
		}, contains: "kind does not match"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			definition := validDefinition()
			test.mutate(&definition)
			err := definition.Validate()
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("expected %q error, got %v", test.contains, err)
			}
		})
	}
}

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

func TestOutcomeOrderingValidationAndDeepClone(t *testing.T) {
	t.Parallel()

	invocation := validInvocation()
	state, err := ObjectValue([]Field{{Key: "count", Value: IntValue(2)}})
	if err != nil {
		t.Fatalf("ObjectValue: %v", err)
	}
	outcome := Outcome{Operations: []Operation{
		&KVPutOperation{Key: "counter", Value: state},
		&MessageOperation{Message: Message{Content: "saved"}, Ephemeral: true},
	}}
	if err := outcome.Validate(invocation); err != nil {
		t.Fatalf("valid outcome rejected: %v", err)
	}
	clone := outcome.DeepClone()
	put := clone.Operations[0].(*KVPutOperation)
	put.Value = StringValue("changed")
	original := outcome.Operations[0].(*KVPutOperation)
	if original.Value.Kind() != ValueObject {
		t.Fatal("outcome value was aliased")
	}

	deferredInvocation := invocation
	deferredInvocation.ResponseState = ResponseDeferredCreate
	deferred := Outcome{Operations: []Operation{
		&KVDeleteOperation{Key: "counter"},
		&EditResponseOperation{Patch: MessagePatch{Content: OptionalString{Set: true, Value: "done"}}},
	}}
	if err := deferred.Validate(deferredInvocation); err != nil {
		t.Fatalf("deferred completion outcome rejected: %v", err)
	}

	invalid := []Outcome{
		{Operations: []Operation{&MessageOperation{Message: Message{Content: "one"}}, &MessageOperation{Message: Message{Content: "two"}}}},
		{Operations: []Operation{(*MessageOperation)(nil)}},
		{Operations: []Operation{&ModalOperation{Modal: ModalView{Handler: "edit", Title: "Modal", Fields: []TextInput{{ID: "x", Label: "X", Style: TextInputShort}}}}, &KVDeleteOperation{Key: "counter"}}},
	}
	for index, value := range invalid {
		if err := value.Validate(invocation); err == nil {
			t.Fatalf("invalid outcome %d accepted", index+1)
		}
	}

	withoutGuild := invocation
	withoutGuild.Guild = nil
	if err := (Outcome{Operations: []Operation{&KVDeleteOperation{Key: "counter"}, &MessageOperation{Message: Message{Content: "done"}}}}).Validate(withoutGuild); err == nil {
		t.Fatal("guild-bound state effect accepted without guild")
	}
}

func TestDomainOperationsValidateScopeAndClone(t *testing.T) {
	t.Parallel()
	invocation := validInvocation()
	invocation.NowUnix = 100
	invocation.Command.Options = append(invocation.Command.Options, OptionValue{Name: "file", Kind: OptionAttachment, Attachment: &AttachmentRef{ID: "40", Filename: "emoji.png", Size: 100}})
	color := 0x123456
	outcome := Outcome{Operations: []Operation{
		&SetSlowmodeOperation{ChannelID: "20", Seconds: 0},
		&CreateRoleOperation{Name: "Helpers", Color: &color},
		&CreateEmojiOperation{Name: "wave", AttachmentID: "40"},
		&MessageOperation{Message: Message{Content: "done"}},
	}}
	if err := outcome.Validate(invocation); err != nil {
		t.Fatalf("domain outcome: %v", err)
	}
	clone := outcome.DeepClone()
	cloneRole := clone.Operations[1].(*CreateRoleOperation)
	*cloneRole.Color = 0
	if *outcome.Operations[1].(*CreateRoleOperation).Color != color {
		t.Fatal("role color pointer was aliased")
	}

	withoutGuild := invocation
	withoutGuild.Guild = nil
	if err := outcome.Validate(withoutGuild); err == nil {
		t.Fatal("guild-scoped domain operation accepted without guild")
	}
	badAttachment := Outcome{Operations: []Operation{&CreateEmojiOperation{Name: "wave", AttachmentID: "missing"}, &MessageOperation{Message: Message{Content: "done"}}}}
	if err := badAttachment.Validate(invocation); err == nil {
		t.Fatal("unknown invocation attachment accepted")
	}
	if err := (Outcome{Operations: []Operation{(*SetSlowmodeOperation)(nil), &MessageOperation{Message: Message{Content: "done"}}}}).Validate(invocation); err == nil {
		t.Fatal("typed-nil domain operation accepted")
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

	autocomplete := Invocation{
		PluginID: "example", Generation: "generation-1", Route: "autocomplete:query", Kind: InvocationAutocomplete,
		ResponseState: ResponseUnacknowledged,
		Author:        &UserRef{ID: "30", Username: "alice"},
		Autocomplete:  &AutocompleteInput{Path: []string{"search"}, Option: "query", Focused: OptionValue{Name: "query", Kind: OptionString, String: "ma"}},
	}
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

	event := Invocation{PluginID: "example", Generation: "generation-1", Route: "event:join", Kind: InvocationEvent, Event: &EventInput{Name: "guild_member_join", Data: NullValue()}}
	if err := (Outcome{}).Validate(event); err != nil {
		t.Fatalf("empty event outcome rejected: %v", err)
	}
	if err := (Outcome{Operations: []Operation{&MessageOperation{Message: Message{Content: "wrong"}}}}).Validate(event); err == nil {
		t.Fatal("event accepted interaction reply")
	}
}

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

	component := Invocation{PluginID: "example", Generation: "generation-1", Route: "component:save", Kind: InvocationComponent, ResponseState: ResponseUnacknowledged, Guild: &GuildRef{ID: "10"}, Author: &UserRef{ID: "30"}, Component: &ComponentInput{ID: "save", Kind: ComponentButton}}
	if _, err := catalog.Resolve(component); err != nil {
		t.Fatalf("Resolve component: %v", err)
	}
	component.Component.Kind = ComponentStringSelect
	if _, err := catalog.Resolve(component); err == nil {
		t.Fatal("component kind mismatch accepted")
	}

	modal := Invocation{PluginID: "example", Generation: "generation-1", Route: "modal:edit", Kind: InvocationModal, ResponseState: ResponseUnacknowledged, ModalOrigin: ModalOriginCommand, Author: &UserRef{ID: "30"}, Modal: &ModalInput{ID: "edit", Fields: []NamedString{{Name: "text", Value: "new"}}}}
	if _, err := catalog.Resolve(modal); err != nil {
		t.Fatalf("Resolve modal: %v", err)
	}
	modal.Modal.Fields[0].Name = "unknown"
	if _, err := catalog.Resolve(modal); err == nil {
		t.Fatal("undeclared modal field accepted")
	}

	autocomplete := Invocation{PluginID: "example", Generation: "generation-1", Route: "autocomplete:tool:count", Kind: InvocationAutocomplete, ResponseState: ResponseUnacknowledged, Author: &UserRef{ID: "30"}, Autocomplete: &AutocompleteInput{Path: []string{"tool", "create"}, Option: "count", Focused: OptionValue{Name: "count", Kind: OptionInteger, Integer: 2}, Options: []OptionValue{{Name: "text", Kind: OptionString, String: "hello"}}}}
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

func validDefinition() Definition {
	minLength, maxLength := 1, 100
	minimum, maximum := int64(1), int64(10)
	return Definition{Cogs: []CogDefinition{{
		Name:   "Tools",
		Checks: []CheckDefinition{{Kind: CheckGuildOnly}},
		Commands: []CommandDefinition{{
			Kind:        CommandSlash,
			Name:        "tool",
			Description: "Tool commands",
			Children: []CommandDefinition{{
				Kind:        CommandSubcommand,
				Route:       "command:tool:create",
				Name:        "create",
				Description: "Create a tool",
				Options: []OptionDefinition{
					{Name: "text", Kind: OptionString, Description: "Text", Required: true, MinLength: &minLength, MaxLength: &maxLength},
					{Name: "count", Kind: OptionInteger, Description: "Count", MinInteger: &minimum, MaxInteger: &maximum, Autocomplete: "autocomplete:tool:count"},
				},
			}},
		}},
		Listeners:  []ListenerDefinition{{ID: "member_join", Event: "guild_member_join", Route: "listener:member_join"}},
		Tasks:      []TaskDefinition{{ID: "sweep", Schedule: "*/5 * * * *", Route: "task:sweep"}},
		Components: []ComponentDefinition{{ID: "save", Route: "component:save", Kinds: []ComponentKind{ComponentButton}, Checks: []CheckDefinition{{Kind: CheckHasPermissions, Permissions: []MemberPermission{PermissionManageMessages}}}}},
		Modals:     []ModalDefinition{{ID: "edit", Route: "modal:edit", Fields: []ModalFieldDefinition{{ID: "text", Required: true}}}},
	}}}
}

func validInvocation() Invocation {
	return Invocation{
		PluginID:      "example",
		Generation:    "generation-1",
		Route:         "command:tool:create",
		Kind:          InvocationCommand,
		ResponseState: ResponseUnacknowledged,
		Guild:         &GuildRef{ID: "10", Name: "Guild"},
		Channel:       &ChannelRef{ID: "20", GuildID: "10", Name: "general", Kind: ChannelText},
		Author:        &UserRef{ID: "30", Username: "alice"},
		Locale:        "en-US",
		Command: &CommandInput{
			Kind:    CommandSlash,
			Path:    []string{"tool", "create"},
			Options: []OptionValue{{Name: "text", Kind: OptionString, String: "hello"}, {Name: "count", Kind: OptionInteger, Integer: 2}},
		},
	}
}
