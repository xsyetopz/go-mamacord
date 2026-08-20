package contract

import (
	"errors"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"strings"
	"unicode/utf8"
)

const MaxOutcomeOperations = 25

type Operation interface {
	pluginOperation()
	cloneOperation() Operation
}

type MessageOperation struct {
	Message   Message
	Ephemeral bool
}

func (*MessageOperation) pluginOperation() {}
func (operation *MessageOperation) cloneOperation() Operation {
	if operation == nil {
		return (*MessageOperation)(nil)
	}
	copy := *operation
	copy.Message = operation.Message.DeepClone()
	return &copy
}

type UpdateOperation struct {
	Patch MessagePatch
}

func (*UpdateOperation) pluginOperation() {}
func (operation *UpdateOperation) cloneOperation() Operation {
	if operation == nil {
		return (*UpdateOperation)(nil)
	}
	copy := *operation
	copy.Patch = operation.Patch.DeepClone()
	return &copy
}

type EditResponseOperation struct {
	Patch MessagePatch
}

func (*EditResponseOperation) pluginOperation() {}
func (operation *EditResponseOperation) cloneOperation() Operation {
	if operation == nil {
		return (*EditResponseOperation)(nil)
	}
	copy := *operation
	copy.Patch = operation.Patch.DeepClone()
	return &copy
}

type ModalOperation struct {
	Modal ModalView
}

func (*ModalOperation) pluginOperation() {}
func (operation *ModalOperation) cloneOperation() Operation {
	if operation == nil {
		return (*ModalOperation)(nil)
	}
	copy := *operation
	copy.Modal = operation.Modal.DeepClone()
	return &copy
}

type AutocompleteChoicesOperation struct {
	Choices []AutocompleteChoice
}

func (*AutocompleteChoicesOperation) pluginOperation() {}
func (operation *AutocompleteChoicesOperation) cloneOperation() Operation {
	if operation == nil {
		return (*AutocompleteChoicesOperation)(nil)
	}
	copy := *operation
	copy.Choices = append([]AutocompleteChoice(nil), operation.Choices...)
	return &copy
}

type AutocompleteChoice struct {
	Name  string
	Value ChoiceValue
}

type KVPutOperation struct {
	Key             string
	Value           Value
	ExpectedVersion *uint64
}

func (*KVPutOperation) pluginOperation() {}
func (operation *KVPutOperation) cloneOperation() Operation {
	if operation == nil {
		return (*KVPutOperation)(nil)
	}
	copy := *operation
	copy.Value = operation.Value.Clone()
	if operation.ExpectedVersion != nil {
		expected := *operation.ExpectedVersion
		copy.ExpectedVersion = &expected
	}
	return &copy
}

type KVDeleteOperation struct {
	Key             string
	ExpectedVersion *uint64
}

func (*KVDeleteOperation) pluginOperation() {}
func (operation *KVDeleteOperation) cloneOperation() Operation {
	if operation == nil {
		return (*KVDeleteOperation)(nil)
	}
	copy := *operation
	if operation.ExpectedVersion != nil {
		expected := *operation.ExpectedVersion
		copy.ExpectedVersion = &expected
	}
	return &copy
}

type BestEffortOperation struct{ Operation Operation }

func (*BestEffortOperation) pluginOperation() {}
func (operation *BestEffortOperation) cloneOperation() Operation {
	if operation == nil {
		return (*BestEffortOperation)(nil)
	}
	return &BestEffortOperation{Operation: cloneOperationValue(operation.Operation)}
}
func (operation *BestEffortOperation) validateBestEffort(invocation Invocation) error {
	if operation == nil || operation.Operation == nil {
		return errors.New("best-effort operation is required")
	}
	switch value := operation.Operation.(type) {
	case domainOperation:
		return value.validateDomain(invocation)
	case *AppendAuditOperation:
		return value.validateDomain(invocation)
	default:
		return fmt.Errorf("operation %T cannot be best-effort", operation.Operation)
	}
}

type GuardedOperation struct {
	Operation Operation
	Failure   Operation
}

func (*GuardedOperation) pluginOperation() {}
func (operation *GuardedOperation) cloneOperation() Operation {
	if operation == nil {
		return (*GuardedOperation)(nil)
	}
	return &GuardedOperation{Operation: cloneOperationValue(operation.Operation), Failure: cloneOperationValue(operation.Failure)}
}
func cloneOperationValue(operation Operation) Operation {
	if operation == nil {
		return nil
	}
	return operation.cloneOperation()
}
func (operation *GuardedOperation) validateGuarded(invocation Invocation) error {
	if operation == nil || operation.Operation == nil || operation.Failure == nil {
		return errors.New("guarded operation requires operation and failure")
	}
	switch value := operation.Operation.(type) {
	case domainOperation:
		if err := value.validateDomain(invocation); err != nil {
			return err
		}
	case *KVPutOperation:
		if value == nil {
			return errors.New("guarded KV put is nil")
		}
		if err := validateKVScope(invocation, value.Key); err != nil {
			return err
		}
		if err := value.Value.ValidateState(); err != nil {
			return err
		}
	case *KVDeleteOperation:
		if value == nil {
			return errors.New("guarded KV delete is nil")
		}
		if err := validateKVScope(invocation, value.Key); err != nil {
			return err
		}
	default:
		return fmt.Errorf("operation %T cannot be guarded", operation.Operation)
	}
	switch failure := operation.Failure.(type) {
	case *MessageOperation:
		if failure == nil {
			return errors.New("guarded failure message is nil")
		}
		if invocation.ResponseState != ResponseUnacknowledged || invocation.Kind != InvocationCommand && invocation.Kind != InvocationComponent && invocation.Kind != InvocationModal {
			return errors.New("guarded failure message requires unacknowledged interaction")
		}
		return failure.Message.Validate()
	case *EditResponseOperation:
		if failure == nil || invocation.ResponseState != ResponseDeferredCreate {
			return errors.New("guarded failure edit requires deferred-create response")
		}
		return failure.Patch.Validate()
	case *UpdateOperation:
		if failure == nil {
			return errors.New("guarded failure update is nil")
		}
		allowedInitial := invocation.ResponseState == ResponseUnacknowledged && (invocation.Kind == InvocationComponent || invocation.Kind == InvocationModal && invocation.ModalOrigin == ModalOriginComponent)
		allowedDeferred := invocation.ResponseState == ResponseDeferredUpdate && (invocation.Kind == InvocationComponent || invocation.Kind == InvocationModal)
		if !allowedInitial && !allowedDeferred {
			return errors.New("guarded failure update is invalid for invocation")
		}
		return failure.Patch.Validate()
	default:
		return fmt.Errorf("guarded failure %T is not a response operation", operation.Failure)
	}
}

type OptionalString struct {
	Set   bool
	Value string
}

type OptionalEmbeds struct {
	Set    bool
	Values []Embed
}

type OptionalComponentRows struct {
	Set    bool
	Values []ComponentRow
}

type MessagePatch struct {
	Content    OptionalString
	Embeds     OptionalEmbeds
	Components OptionalComponentRows
}

func (patch MessagePatch) DeepClone() MessagePatch {
	out := patch
	if patch.Embeds.Values != nil {
		out.Embeds.Values = make([]Embed, len(patch.Embeds.Values))
		for index, embed := range patch.Embeds.Values {
			out.Embeds.Values[index] = embed.DeepClone()
		}
	}
	if patch.Components.Values != nil {
		out.Components.Values = make([]ComponentRow, len(patch.Components.Values))
		for index, row := range patch.Components.Values {
			out.Components.Values[index] = row.DeepClone()
		}
	}
	return out
}

func (patch MessagePatch) Validate() error {
	if !patch.Content.Set && !patch.Embeds.Set && !patch.Components.Set {
		return errors.New("message patch has no fields")
	}
	if patch.Content.Set && utf8.RuneCountInString(patch.Content.Value) > 2000 {
		return errors.New("message patch content exceeds 2000 characters")
	}
	if patch.Embeds.Set {
		if len(patch.Embeds.Values) > 10 {
			return errors.New("message patch exceeds 10 embeds")
		}
		total := 0
		for index, embed := range patch.Embeds.Values {
			count, err := embed.validate()
			if err != nil {
				return fmt.Errorf("embed %d: %w", index+1, err)
			}
			total += count
		}
		if total > 6000 {
			return errors.New("message patch embed text exceeds 6000 characters")
		}
	}
	if patch.Components.Set {
		if len(patch.Components.Values) > 5 {
			return errors.New("message patch exceeds 5 component rows")
		}
		for index, row := range patch.Components.Values {
			if err := row.Validate(); err != nil {
				return fmt.Errorf("component row %d: %w", index+1, err)
			}
		}
	}
	return nil
}

type Message struct {
	Content    string
	Embeds     []Embed
	Components []ComponentRow
}

func (message Message) DeepClone() Message {
	out := message
	if message.Embeds != nil {
		out.Embeds = make([]Embed, len(message.Embeds))
		for index, embed := range message.Embeds {
			out.Embeds[index] = embed.DeepClone()
		}
	}
	if message.Components != nil {
		out.Components = make([]ComponentRow, len(message.Components))
		for index, row := range message.Components {
			out.Components[index] = row.DeepClone()
		}
	}
	return out
}

type Embed struct {
	Title        string
	Description  string
	URL          string
	Color        int
	Fields       []EmbedField
	Author       *EmbedAuthor
	Footer       *EmbedFooter
	ImageURL     string
	ThumbnailURL string
}

type EmbedField struct {
	Name   string
	Value  string
	Inline bool
}
type EmbedAuthor struct{ Name, URL, IconURL string }
type EmbedFooter struct{ Text, IconURL string }

func (embed Embed) DeepClone() Embed {
	out := embed
	out.Fields = append([]EmbedField(nil), embed.Fields...)
	if embed.Author != nil {
		author := *embed.Author
		out.Author = &author
	}
	if embed.Footer != nil {
		footer := *embed.Footer
		out.Footer = &footer
	}
	return out
}

type ComponentRow struct {
	Components []MessageComponent
}

func (row ComponentRow) DeepClone() ComponentRow {
	out := ComponentRow{Components: make([]MessageComponent, len(row.Components))}
	for index, component := range row.Components {
		out.Components[index] = component.cloneMessageComponent()
	}
	return out
}

type MessageComponent interface {
	messageComponent()
	cloneMessageComponent() MessageComponent
}

type ButtonStyle string

const (
	ButtonPrimary   ButtonStyle = "primary"
	ButtonSecondary ButtonStyle = "secondary"
	ButtonSuccess   ButtonStyle = "success"
	ButtonDanger    ButtonStyle = "danger"
	ButtonLink      ButtonStyle = "link"
)

type Emoji struct {
	ID       string
	Name     string
	Animated bool
}

func (emoji Emoji) Validate() error {
	if strings.TrimSpace(emoji.ID) == "" && strings.TrimSpace(emoji.Name) == "" {
		return errors.New("emoji requires id or name")
	}
	if utf8.RuneCountInString(emoji.Name) > 32 {
		return errors.New("emoji name exceeds 32 characters")
	}
	return nil
}

type Button struct {
	Handler  string
	Label    string
	Style    ButtonStyle
	URL      string
	Emoji    *Emoji
	Disabled bool
}

func (*Button) messageComponent() {}
func (button *Button) cloneMessageComponent() MessageComponent {
	if button == nil {
		return (*Button)(nil)
	}
	copy := *button
	if button.Emoji != nil {
		emoji := *button.Emoji
		copy.Emoji = &emoji
	}
	return &copy
}

type SelectKind string

const (
	SelectString      SelectKind = "string"
	SelectUser        SelectKind = "user"
	SelectRole        SelectKind = "role"
	SelectMentionable SelectKind = "mentionable"
	SelectChannel     SelectKind = "channel"
)

type Select struct {
	Handler      string
	Kind         SelectKind
	Placeholder  string
	MinValues    int
	MaxValues    int
	Disabled     bool
	Options      []SelectOption
	ChannelKinds []ChannelKind
}

func (*Select) messageComponent() {}
func (selectMenu *Select) cloneMessageComponent() MessageComponent {
	if selectMenu == nil {
		return (*Select)(nil)
	}
	copy := *selectMenu
	copy.Options = append([]SelectOption(nil), selectMenu.Options...)
	for index := range copy.Options {
		if selectMenu.Options[index].Emoji != nil {
			emoji := *selectMenu.Options[index].Emoji
			copy.Options[index].Emoji = &emoji
		}
	}
	copy.ChannelKinds = append([]ChannelKind(nil), selectMenu.ChannelKinds...)
	return &copy
}

type SelectOption struct {
	Label       string
	Value       string
	Description string
	Emoji       *Emoji
	Default     bool
}

type ModalView struct {
	Handler string
	Title   string
	Fields  []TextInput
}

func (modal ModalView) DeepClone() ModalView {
	modal.Fields = append([]TextInput(nil), modal.Fields...)
	return modal
}

type TextInputStyle string

const (
	TextInputShort     TextInputStyle = "short"
	TextInputParagraph TextInputStyle = "paragraph"
)

type TextInput struct {
	ID          string
	Label       string
	Style       TextInputStyle
	Placeholder string
	Value       string
	Required    bool
	MinLength   int
	MaxLength   int
}

type CheckDecisionKind string

const (
	CheckAllowed       CheckDecisionKind = "allow"
	CheckDeniedSilent  CheckDecisionKind = "deny_silent"
	CheckDeniedMessage CheckDecisionKind = "deny_message"
)

type CheckDecision struct {
	Kind   CheckDecisionKind
	Denial *MessageOperation
}

func AllowedCheck() CheckDecision      { return CheckDecision{Kind: CheckAllowed} }
func SilentDeniedCheck() CheckDecision { return CheckDecision{Kind: CheckDeniedSilent} }
func DeniedCheck(denial *MessageOperation) CheckDecision {
	decision := CheckDecision{Kind: CheckDeniedMessage}
	if denial != nil {
		decision.Denial = denial.cloneOperation().(*MessageOperation)
	}
	return decision
}
func (decision CheckDecision) DeepClone() CheckDecision {
	if decision.Denial != nil {
		decision.Denial = decision.Denial.cloneOperation().(*MessageOperation)
	}
	return decision
}
func (decision CheckDecision) Validate() error {
	switch decision.Kind {
	case CheckAllowed, CheckDeniedSilent:
		if decision.Denial != nil {
			return fmt.Errorf("%s check cannot contain a denial reply", decision.Kind)
		}
	case CheckDeniedMessage:
		if decision.Denial == nil {
			return errors.New("message denial requires a reply")
		}
		if err := decision.Denial.Message.Validate(); err != nil {
			return fmt.Errorf("denial reply: %w", err)
		}
		if !decision.Denial.Ephemeral {
			return errors.New("check denial reply must be ephemeral")
		}
	default:
		return fmt.Errorf("unsupported check decision %q", decision.Kind)
	}
	return nil
}

type Outcome struct {
	Operations []Operation
}

func (outcome Outcome) DeepClone() Outcome {
	out := Outcome{Operations: make([]Operation, len(outcome.Operations))}
	for index, operation := range outcome.Operations {
		if operation != nil {
			out.Operations[index] = operation.cloneOperation()
		}
	}
	return out
}

func (outcome Outcome) Validate(invocation Invocation) error {
	if err := invocation.Validate(); err != nil {
		return fmt.Errorf("invocation: %w", err)
	}
	if len(outcome.Operations) > MaxOutcomeOperations {
		return fmt.Errorf("outcome exceeds %d operations", MaxOutcomeOperations)
	}
	terminal := false
	choices := false

	for index, operation := range outcome.Operations {
		if operation == nil || reflect.ValueOf(operation).Kind() == reflect.Pointer && reflect.ValueOf(operation).IsNil() {
			return fmt.Errorf("operation %d is nil", index+1)
		}
		if terminal {
			return fmt.Errorf("operation %d follows a terminal interaction response", index+1)
		}
		switch value := operation.(type) {
		case *MessageOperation:
			if value == nil {
				return fmt.Errorf("message operation %d is nil", index+1)
			}
			if invocation.ResponseState != ResponseUnacknowledged || invocation.Kind == InvocationAutocomplete || invocation.Kind == InvocationEvent || invocation.Kind == InvocationTask {
				return fmt.Errorf("message operation %d is invalid for %s/%s", index+1, invocation.Kind, invocation.ResponseState)
			}
			if err := value.Message.Validate(); err != nil {
				return fmt.Errorf("message operation %d: %w", index+1, err)
			}
			terminal = true
		case *UpdateOperation:
			if value == nil {
				return fmt.Errorf("update operation %d is nil", index+1)
			}
			allowedInitial := invocation.ResponseState == ResponseUnacknowledged && (invocation.Kind == InvocationComponent || invocation.Kind == InvocationModal && invocation.ModalOrigin == ModalOriginComponent)
			allowedDeferred := invocation.ResponseState == ResponseDeferredUpdate && (invocation.Kind == InvocationComponent || invocation.Kind == InvocationModal)
			if !allowedInitial && !allowedDeferred {
				return fmt.Errorf("update operation %d is invalid for %s/%s", index+1, invocation.Kind, invocation.ResponseState)
			}
			if err := value.Patch.Validate(); err != nil {
				return fmt.Errorf("update operation %d: %w", index+1, err)
			}
			terminal = true
		case *EditResponseOperation:
			if value == nil {
				return fmt.Errorf("edit response operation %d is nil", index+1)
			}
			if invocation.ResponseState != ResponseDeferredCreate {
				return fmt.Errorf("edit response operation %d requires deferred-create state", index+1)
			}
			if err := value.Patch.Validate(); err != nil {
				return fmt.Errorf("edit response operation %d: %w", index+1, err)
			}
			terminal = true
		case *ModalOperation:
			if value == nil {
				return fmt.Errorf("modal operation %d is nil", index+1)
			}
			if invocation.ResponseState != ResponseUnacknowledged || invocation.Kind != InvocationCommand && invocation.Kind != InvocationComponent {
				return fmt.Errorf("modal operation %d is invalid for %s/%s", index+1, invocation.Kind, invocation.ResponseState)
			}
			if err := value.Modal.Validate(); err != nil {
				return fmt.Errorf("modal operation %d: %w", index+1, err)
			}
			terminal = true
		case *AutocompleteChoicesOperation:
			if value == nil {
				return fmt.Errorf("autocomplete choices operation %d is nil", index+1)
			}
			if invocation.Kind != InvocationAutocomplete || invocation.ResponseState != ResponseUnacknowledged {
				return fmt.Errorf("autocomplete choices operation %d requires unacknowledged autocomplete invocation", index+1)
			}
			if err := value.ValidateFor(invocation.Autocomplete.Focused.Kind); err != nil {
				return fmt.Errorf("autocomplete choices operation %d: %w", index+1, err)
			}
			choices = true
			terminal = true
		case *KVPutOperation:
			if value == nil {
				return fmt.Errorf("kv put operation %d is nil", index+1)
			}
			if err := validateKVScope(invocation, value.Key); err != nil {
				return fmt.Errorf("kv put operation %d: %w", index+1, err)
			}
			if err := value.Value.ValidateState(); err != nil {
				return fmt.Errorf("kv put operation %d value: %w", index+1, err)
			}
		case *KVDeleteOperation:
			if value == nil {
				return fmt.Errorf("kv delete operation %d is nil", index+1)
			}
			if err := validateKVScope(invocation, value.Key); err != nil {
				return fmt.Errorf("kv delete operation %d: %w", index+1, err)
			}
		case *BestEffortOperation:
			if err := value.validateBestEffort(invocation); err != nil {
				return fmt.Errorf("best-effort operation %d: %w", index+1, err)
			}
		case *GuardedOperation:
			if err := value.validateGuarded(invocation); err != nil {
				return fmt.Errorf("guarded operation %d: %w", index+1, err)
			}
		case domainOperation:
			if invocation.Kind == InvocationAutocomplete {
				return fmt.Errorf("domain operation %d is invalid for autocomplete", index+1)
			}
			if err := value.validateDomain(invocation); err != nil {
				return fmt.Errorf("domain operation %d: %w", index+1, err)
			}
		default:
			return fmt.Errorf("operation %d has unsupported type %T", index+1, operation)
		}
	}

	requiresTerminal := invocation.ResponseState == ResponseUnacknowledged || invocation.ResponseState == ResponseDeferredCreate || invocation.ResponseState == ResponseDeferredUpdate
	if (invocation.Kind == InvocationCommand || invocation.Kind == InvocationComponent || invocation.Kind == InvocationModal) && requiresTerminal && !terminal {
		return errors.New("interaction outcome requires a terminal response")
	}
	if invocation.Kind == InvocationAutocomplete && !choices {
		return errors.New("autocomplete outcome requires choices")
	}
	return nil
}

func (operation AutocompleteChoicesOperation) ValidateFor(optionKind OptionKind) error {
	if len(operation.Choices) > 25 {
		return errors.New("autocomplete exceeds 25 choices")
	}
	seen := make(map[string]struct{}, len(operation.Choices))
	for index, choice := range operation.Choices {
		if strings.TrimSpace(choice.Name) == "" || utf8.RuneCountInString(choice.Name) > 100 {
			return fmt.Errorf("choice %d name must contain 1 to 100 characters", index+1)
		}
		if err := choice.Value.Validate(); err != nil {
			return fmt.Errorf("choice %q: %w", choice.Name, err)
		}
		expected := ChoiceString
		if optionKind == OptionInteger {
			expected = ChoiceInteger
		} else if optionKind == OptionNumber {
			expected = ChoiceNumber
		} else if optionKind != OptionString {
			return fmt.Errorf("option kind %q cannot autocomplete", optionKind)
		}
		if choice.Value.Kind != expected {
			return fmt.Errorf("choice %q kind %q does not match option kind %q", choice.Name, choice.Value.Kind, optionKind)
		}
		key := choiceIdentity(choice)
		if _, exists := seen[key]; exists {
			return fmt.Errorf("duplicate autocomplete choice %q", choice.Name)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func choiceIdentity(choice AutocompleteChoice) string {
	value := choice.Value
	switch value.Kind {
	case ChoiceString:
		return string(value.Kind) + "\x00" + value.String
	case ChoiceInteger:
		return fmt.Sprintf("%s\x00%d", value.Kind, value.Integer)
	case ChoiceNumber:
		return fmt.Sprintf("%s\x00%016x", value.Kind, math.Float64bits(value.Number))
	default:
		return string(value.Kind)
	}
}

func validateKVScope(invocation Invocation, key string) error {
	if invocation.Guild == nil || strings.TrimSpace(invocation.Guild.ID) == "" {
		return errors.New("guild context is required")
	}
	if key != strings.TrimSpace(key) || !kvKeyPattern.MatchString(key) {
		return errors.New("key is not canonical")
	}
	return nil
}

func (message Message) Validate() error {
	if utf8.RuneCountInString(message.Content) > 2000 {
		return errors.New("message content exceeds 2000 characters")
	}
	if len(message.Embeds) > 10 {
		return errors.New("message exceeds 10 embeds")
	}
	if len(message.Components) > 5 {
		return errors.New("message exceeds 5 component rows")
	}
	if message.Content == "" && len(message.Embeds) == 0 && len(message.Components) == 0 {
		return errors.New("message is empty")
	}
	totalEmbedRunes := 0
	for index, embed := range message.Embeds {
		count, err := embed.validate()
		if err != nil {
			return fmt.Errorf("embed %d: %w", index+1, err)
		}
		totalEmbedRunes += count
	}
	if totalEmbedRunes > 6000 {
		return errors.New("message embed text exceeds 6000 characters")
	}
	for index, row := range message.Components {
		if err := row.Validate(); err != nil {
			return fmt.Errorf("component row %d: %w", index+1, err)
		}
	}
	return nil
}

func (embed Embed) validate() (int, error) {
	if embed.Color < 0 || embed.Color > 0xFFFFFF {
		return 0, errors.New("color is outside RGB range")
	}
	if utf8.RuneCountInString(embed.Title) > 256 {
		return 0, errors.New("title exceeds 256 characters")
	}
	if utf8.RuneCountInString(embed.Description) > 4096 {
		return 0, errors.New("description exceeds 4096 characters")
	}
	if len(embed.Fields) > 25 {
		return 0, errors.New("embed exceeds 25 fields")
	}
	for _, rawURL := range []string{embed.URL, embed.ImageURL, embed.ThumbnailURL} {
		if err := validateHTTPSURL(rawURL); err != nil {
			return 0, err
		}
	}
	total := utf8.RuneCountInString(embed.Title) + utf8.RuneCountInString(embed.Description)
	for index, field := range embed.Fields {
		if field.Name == "" || utf8.RuneCountInString(field.Name) > 256 {
			return 0, fmt.Errorf("field %d name must contain 1 to 256 characters", index+1)
		}
		if field.Value == "" || utf8.RuneCountInString(field.Value) > 1024 {
			return 0, fmt.Errorf("field %d value must contain 1 to 1024 characters", index+1)
		}
		total += utf8.RuneCountInString(field.Name) + utf8.RuneCountInString(field.Value)
	}
	if embed.Author != nil {
		if embed.Author.Name == "" || utf8.RuneCountInString(embed.Author.Name) > 256 {
			return 0, errors.New("author name must contain 1 to 256 characters")
		}
		if err := validateHTTPSURL(embed.Author.URL); err != nil {
			return 0, err
		}
		if err := validateHTTPSURL(embed.Author.IconURL); err != nil {
			return 0, err
		}
		total += utf8.RuneCountInString(embed.Author.Name)
	}
	if embed.Footer != nil {
		if embed.Footer.Text == "" || utf8.RuneCountInString(embed.Footer.Text) > 2048 {
			return 0, errors.New("footer text must contain 1 to 2048 characters")
		}
		if err := validateHTTPSURL(embed.Footer.IconURL); err != nil {
			return 0, err
		}
		total += utf8.RuneCountInString(embed.Footer.Text)
	}
	return total, nil
}

func (row ComponentRow) Validate() error {
	if len(row.Components) == 0 || len(row.Components) > 5 {
		return errors.New("component row must contain 1 to 5 components")
	}
	for index, component := range row.Components {
		if component == nil {
			return fmt.Errorf("component %d is nil", index+1)
		}
		switch value := component.(type) {
		case *Button:
			if value == nil {
				return fmt.Errorf("button %d is nil", index+1)
			}
			if err := value.Validate(); err != nil {
				return fmt.Errorf("button %d: %w", index+1, err)
			}
		case *Select:
			if value == nil {
				return errors.New("select is nil")
			}
			if len(row.Components) != 1 {
				return errors.New("a select must be the only component in its row")
			}
			if err := value.Validate(); err != nil {
				return fmt.Errorf("select: %w", err)
			}
		default:
			return fmt.Errorf("component %d has unsupported type %T", index+1, component)
		}
	}
	return nil
}

func (button Button) Validate() error {
	if utf8.RuneCountInString(button.Label) > 80 {
		return errors.New("label exceeds 80 characters")
	}
	switch button.Style {
	case ButtonPrimary, ButtonSecondary, ButtonSuccess, ButtonDanger:
		if !handlerIDPattern.MatchString(button.Handler) {
			return errors.New("custom button handler is invalid")
		}
		if button.URL != "" {
			return errors.New("custom button cannot have a URL")
		}
	case ButtonLink:
		if button.Handler != "" {
			return errors.New("link button cannot have a handler")
		}
		if err := validateHTTPSURLRequired(button.URL); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported button style %q", button.Style)
	}
	if strings.TrimSpace(button.Label) == "" && button.Emoji == nil {
		return errors.New("button requires a label or emoji")
	}
	if button.Emoji != nil {
		if err := button.Emoji.Validate(); err != nil {
			return fmt.Errorf("button emoji: %w", err)
		}
	}
	return nil
}

func (selectMenu Select) Validate() error {
	if !handlerIDPattern.MatchString(selectMenu.Handler) {
		return errors.New("select handler is invalid")
	}
	if utf8.RuneCountInString(selectMenu.Placeholder) > 150 {
		return errors.New("placeholder exceeds 150 characters")
	}
	minValues, maxValues := selectMenu.MinValues, selectMenu.MaxValues
	if minValues == 0 {
		minValues = 1
	}
	if maxValues == 0 {
		maxValues = 1
	}
	if minValues < 0 || maxValues < 1 || minValues > maxValues || maxValues > 25 {
		return errors.New("select min/max values are invalid")
	}
	switch selectMenu.Kind {
	case SelectString:
		if len(selectMenu.Options) == 0 || len(selectMenu.Options) > 25 {
			return errors.New("string select must contain 1 to 25 options")
		}
		if len(selectMenu.ChannelKinds) != 0 {
			return errors.New("string select cannot contain channel kinds")
		}
	case SelectUser, SelectRole, SelectMentionable:
		if len(selectMenu.Options) != 0 || len(selectMenu.ChannelKinds) != 0 {
			return errors.New("entity select contains unsupported options or channel kinds")
		}
	case SelectChannel:
		if len(selectMenu.Options) != 0 {
			return errors.New("channel select cannot contain string options")
		}
	default:
		return fmt.Errorf("unsupported select kind %q", selectMenu.Kind)
	}
	seenKinds := make(map[ChannelKind]struct{}, len(selectMenu.ChannelKinds))
	for _, kind := range selectMenu.ChannelKinds {
		if !validChannelKind(kind) {
			return fmt.Errorf("unsupported select channel kind %q", kind)
		}
		if _, exists := seenKinds[kind]; exists {
			return fmt.Errorf("duplicate select channel kind %q", kind)
		}
		seenKinds[kind] = struct{}{}
	}
	seen := make(map[string]struct{}, len(selectMenu.Options))
	defaults := 0
	for index, option := range selectMenu.Options {
		if strings.TrimSpace(option.Label) == "" || utf8.RuneCountInString(option.Label) > 100 {
			return fmt.Errorf("option %d label must contain 1 to 100 characters", index+1)
		}
		if option.Value == "" || utf8.RuneCountInString(option.Value) > 100 {
			return fmt.Errorf("option %d value must contain 1 to 100 characters", index+1)
		}
		if utf8.RuneCountInString(option.Description) > 100 {
			return fmt.Errorf("option %d description exceeds 100 characters", index+1)
		}
		if _, exists := seen[option.Value]; exists {
			return fmt.Errorf("duplicate select option value %q", option.Value)
		}
		seen[option.Value] = struct{}{}
		if option.Emoji != nil {
			if err := option.Emoji.Validate(); err != nil {
				return fmt.Errorf("option %d emoji: %w", index+1, err)
			}
		}
		if option.Default {
			defaults++
		}
	}
	if defaults > maxValues {
		return errors.New("default select options exceed maximum values")
	}
	return nil
}

func (modal ModalView) Validate() error {
	if !handlerIDPattern.MatchString(modal.Handler) {
		return errors.New("modal handler is invalid")
	}
	if modal.Title == "" || utf8.RuneCountInString(modal.Title) > 45 {
		return errors.New("modal title must contain 1 to 45 characters")
	}
	if len(modal.Fields) == 0 || len(modal.Fields) > 5 {
		return errors.New("modal must contain 1 to 5 fields")
	}
	seen := make(map[string]struct{}, len(modal.Fields))
	for index, field := range modal.Fields {
		if err := field.Validate(); err != nil {
			return fmt.Errorf("field %d: %w", index+1, err)
		}
		if _, exists := seen[field.ID]; exists {
			return fmt.Errorf("duplicate modal field id %q", field.ID)
		}
		seen[field.ID] = struct{}{}
	}
	return nil
}

func (field TextInput) Validate() error {
	if strings.TrimSpace(field.ID) == "" {
		return errors.New("id is required")
	}
	if utf8.RuneCountInString(field.ID) > 100 {
		return errors.New("id exceeds 100 characters")
	}
	if field.Label == "" || utf8.RuneCountInString(field.Label) > 45 {
		return errors.New("label must contain 1 to 45 characters")
	}
	if field.Style != TextInputShort && field.Style != TextInputParagraph {
		return fmt.Errorf("unsupported text input style %q", field.Style)
	}
	if utf8.RuneCountInString(field.Placeholder) > 100 {
		return errors.New("placeholder exceeds 100 characters")
	}
	if field.MinLength < 0 || field.MaxLength < 0 || field.MaxLength > 4000 {
		return errors.New("min/max length is invalid")
	}
	effectiveMaximum := field.MaxLength
	if effectiveMaximum == 0 {
		effectiveMaximum = 4000
	}
	if field.MinLength > effectiveMaximum {
		return errors.New("minimum length exceeds maximum")
	}
	valueLength := utf8.RuneCountInString(field.Value)
	if valueLength > effectiveMaximum {
		return errors.New("value exceeds maximum length")
	}
	if field.Value != "" && valueLength < field.MinLength {
		return errors.New("value is shorter than minimum length")
	}
	return nil
}

func validateHTTPSURLRequired(raw string) error {
	if raw == "" {
		return errors.New("URL is required")
	}
	return validateHTTPSURL(raw)
}

func validateHTTPSURL(raw string) error {
	if raw == "" {
		return nil
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("URL %q must be absolute HTTPS", raw)
	}
	return nil
}
