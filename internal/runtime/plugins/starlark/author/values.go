package author

import (
	"fmt"

	starlarkgo "go.starlark.net/starlark"
)

type apiValueKind string

const (
	apiPlugin     apiValueKind = "plugin"
	apiCog        apiValueKind = "cog"
	apiCommand    apiValueKind = "command"
	apiOption     apiValueKind = "option"
	apiChoice     apiValueKind = "choice"
	apiCheck      apiValueKind = "check"
	apiComponent  apiValueKind = "component"
	apiModal      apiValueKind = "modal"
	apiModalField apiValueKind = "modal_field"
	apiListener   apiValueKind = "listener"
	apiTask       apiValueKind = "task"
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
	commandIdentity
	commandBehavior
	commandComposition
}

type commandIdentity struct {
	kind          string
	id            string
	name          string
	description   string
	descriptionID string
}

type commandBehavior struct {
	handler     starlarkgo.Callable
	ephemeral   bool
	deferMode   string
	permissions []string
}

type commandComposition struct {
	options  []*apiValue
	children []*apiValue
	checks   []*apiValue
}

type optionDeclaration struct {
	optionIdentity
	optionChoices
	optionNumericBounds
	optionLengthBounds
}

type optionIdentity struct {
	kind          string
	name          string
	description   string
	descriptionID string
	required      bool
}

type optionChoices struct {
	choices      []*apiValue
	channelKinds []string
	autocomplete starlarkgo.Callable
}

type optionNumericBounds struct {
	minInteger *int64
	maxInteger *int64
	minNumber  *float64
	maxNumber  *float64
}

type optionLengthBounds struct {
	minLength *int
	maxLength *int
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
