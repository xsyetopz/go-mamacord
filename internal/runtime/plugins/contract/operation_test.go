package contract

import (
	"testing"
)

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
	invocation.Command.Options = append(invocation.Command.Options, OptionValue{Name: "file", Kind: OptionAttachment, ReferenceOptionValue: ReferenceOptionValue{Attachment: &AttachmentRef{ID: "40", Filename: "emoji.png", Size: 100}}})
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
