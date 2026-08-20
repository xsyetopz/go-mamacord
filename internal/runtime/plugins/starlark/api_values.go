package starlark

import (
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

type apiValueKind string

const (
	apiPlugin       apiValueKind = "plugin"
	apiCog          apiValueKind = "cog"
	apiCommand      apiValueKind = "command"
	apiOption       apiValueKind = "option"
	apiChoice       apiValueKind = "choice"
	apiCheck        apiValueKind = "check"
	apiComponent    apiValueKind = "component"
	apiModal        apiValueKind = "modal"
	apiModalField   apiValueKind = "modal_field"
	apiListener     apiValueKind = "listener"
	apiTask         apiValueKind = "task"
	apiEffect       apiValueKind = "effect"
	apiButton       apiValueKind = "button"
	apiSelect       apiValueKind = "select"
	apiSelectOption apiValueKind = "select_option"
	apiEmbed        apiValueKind = "embed"
	apiEmbedField   apiValueKind = "embed_field"
	apiEmbedAuthor  apiValueKind = "embed_author"
	apiEmbedFooter  apiValueKind = "embed_footer"
	apiRow          apiValueKind = "row"
	apiTextInput    apiValueKind = "text_input"
	apiModalView    apiValueKind = "modal_view"
	apiAutoChoice   apiValueKind = "autocomplete_choice"
)

type apiValue struct {
	kind apiValueKind
	data any
}

func (value *apiValue) String() string {
	if value == nil {
		return "<nil>"
	}
	return fmt.Sprintf("<mamacord %s>", value.kind)
}
func (value *apiValue) Type() string {
	if value == nil {
		return "mamacord.nil"
	}
	return "mamacord." + string(value.kind)
}
func (*apiValue) Freeze()                     {}
func (*apiValue) Truth() starlarkgo.Bool      { return starlarkgo.True }
func (value *apiValue) Hash() (uint32, error) { return 0, fmt.Errorf("unhashable: %s", value.Type()) }

type pluginDeclaration struct{ setup starlarkgo.Callable }

type cogDeclaration struct {
	name       string
	checks     []*apiValue
	commands   []*apiValue
	listeners  []*apiValue
	tasks      []*apiValue
	components []*apiValue
	modals     []*apiValue
}

type commandDeclaration struct {
	kind          string
	id            string
	name          string
	description   string
	descriptionID string
	handler       starlarkgo.Callable
	ephemeral     bool
	deferMode     string
	permissions   []string
	options       []*apiValue
	children      []*apiValue
	checks        []*apiValue
}

type optionDeclaration struct {
	kind          string
	name          string
	description   string
	descriptionID string
	required      bool
	choices       []*apiValue
	minInteger    *int64
	maxInteger    *int64
	minNumber     *float64
	maxNumber     *float64
	minLength     *int
	maxLength     *int
	channelKinds  []string
	autocomplete  starlarkgo.Callable
}

type choiceDeclaration struct {
	name  string
	value starlarkgo.Value
}

type checkDeclaration struct {
	kind        string
	id          string
	permissions []string
	handler     starlarkgo.Callable
}

type componentDeclaration struct {
	id        string
	kinds     []string
	handler   starlarkgo.Callable
	deferMode string
	checks    []*apiValue
}

type modalFieldDeclaration struct {
	id       string
	required bool
}
type modalDeclaration struct {
	id        string
	handler   starlarkgo.Callable
	deferMode string
	fields    []*apiValue
	checks    []*apiValue
}
type listenerDeclaration struct {
	id, event string
	handler   starlarkgo.Callable
	checks    []*apiValue
}
type taskDeclaration struct {
	id, schedule string
	handler      starlarkgo.Callable
	checks       []*apiValue
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
