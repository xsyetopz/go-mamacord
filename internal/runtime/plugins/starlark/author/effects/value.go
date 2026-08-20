package effects

import (
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/internal/evaluation"
	starlarkgo "go.starlark.net/starlark"
)

type valueKind string

const (
	valueEffect       valueKind = "effect"
	valueButton       valueKind = "button"
	valueSelect       valueKind = "select"
	valueSelectOption valueKind = "select_option"
	valueEmbed        valueKind = "embed"
	valueEmbedField   valueKind = "embed_field"
	valueEmbedAuthor  valueKind = "embed_author"
	valueEmbedFooter  valueKind = "embed_footer"
	valueRow          valueKind = "row"
	valueTextInput    valueKind = "text_input"
	valueModalView    valueKind = "modal_view"
	valueAutoChoice   valueKind = "autocomplete_choice"
)

type authoredValue struct {
	kind valueKind
	data any
}

func (value *authoredValue) String() string {
	if value == nil {
		return "<nil>"
	}
	return fmt.Sprintf("<mamacord %s>", value.kind)
}

func (value *authoredValue) Type() string {
	if value == nil {
		return "mamacord.nil"
	}
	return "mamacord." + string(value.kind)
}

func (*authoredValue) Freeze()                {}
func (*authoredValue) Truth() starlarkgo.Bool { return starlarkgo.True }
func (value *authoredValue) Hash() (uint32, error) {
	return 0, fmt.Errorf("unhashable: %s", value.Type())
}

type effectKind string

const (
	effectReply        effectKind = "reply"
	effectUpdate       effectKind = "update"
	effectModal        effectKind = "modal"
	effectKVPut        effectKind = "kv_put"
	effectKVDelete     effectKind = "kv_delete"
	effectAutocomplete effectKind = "autocomplete"
	effectDomain       effectKind = "domain"
	effectGuarded      effectKind = "guarded"
	effectBestEffort   effectKind = "best_effort"
)

type effectDeclaration struct {
	kind effectKind
	data any
}

type guardedEffectDeclaration struct {
	operation effectDeclaration
	failure   effectDeclaration
}

type replyDeclaration struct {
	message   contract.Message
	ephemeral bool
}

type updateDeclaration struct{ patch contract.MessagePatch }

type kvPutDeclaration struct {
	key             string
	value           contract.Value
	expectedVersion *uint64
}

type autocompleteDeclaration struct{ choices []contract.AutocompleteChoice }

func valueList(raw starlarkgo.Value, kind valueKind) ([]*authoredValue, error) {
	if raw == nil || raw == starlarkgo.None {
		return nil, nil
	}
	iterable, ok := raw.(starlarkgo.Iterable)
	if !ok {
		return nil, fmt.Errorf("want iterable of mamacord.%s, got %s", kind, raw.Type())
	}
	iterator := iterable.Iterate()
	defer iterator.Done()
	var item starlarkgo.Value
	var values []*authoredValue
	for iterator.Next(&item) {
		typed, ok := item.(*authoredValue)
		if !ok || typed == nil || typed.kind != kind {
			return nil, fmt.Errorf("want mamacord.%s, got %s", kind, item.Type())
		}
		values = append(values, typed)
		if len(values) > evaluation.MaxCollectionItems {
			return nil, fmt.Errorf("collection exceeds %d items", evaluation.MaxCollectionItems)
		}
	}
	return values, nil
}

func stringList(raw starlarkgo.Value) ([]string, error) {
	if raw == nil || raw == starlarkgo.None {
		return nil, nil
	}
	iterable, ok := raw.(starlarkgo.Iterable)
	if !ok {
		return nil, fmt.Errorf("want iterable of strings, got %s", raw.Type())
	}
	iterator := iterable.Iterate()
	defer iterator.Done()
	var item starlarkgo.Value
	var values []string
	for iterator.Next(&item) {
		text, ok := starlarkgo.AsString(item)
		if !ok {
			return nil, fmt.Errorf("want string, got %s", item.Type())
		}
		values = append(values, text)
		if len(values) > evaluation.MaxCollectionItems {
			return nil, fmt.Errorf("collection exceeds %d items", evaluation.MaxCollectionItems)
		}
	}
	return values, nil
}
