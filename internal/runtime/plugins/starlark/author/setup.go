package author

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/compile"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/internal/evaluation"
	starlarkgo "go.starlark.net/starlark"
)

const pluginGlobalName = "PLUGIN"

var (
	apiIDPattern          = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
	apiCommandNamePattern = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
)

type SetupResult struct {
	ID         contract.GenerationID
	Limits     evaluation.Limits
	Definition contract.Definition
	Catalog    *contract.RouteCatalog
	Routes     map[contract.RouteID]starlarkgo.Callable
}

func Setup(ctx context.Context, bundle *compile.InitializedBundle, id contract.GenerationID, print func(string)) (*SetupResult, error) {
	if !bundle.Ready() {
		return nil, evaluation.NewRuntimeError(evaluation.ErrorSetup, "setup", evaluation.EntrypointLabel, errors.New("initialized bundle is required"))
	}
	if strings.TrimSpace(string(id)) == "" {
		return nil, evaluation.NewRuntimeError(evaluation.ErrorValidation, "setup", evaluation.EntrypointLabel, errors.New("generation id is required"))
	}
	global, exists := bundle.Global(pluginGlobalName)
	if !exists {
		return nil, evaluation.NewRuntimeError(evaluation.ErrorValidation, "setup", evaluation.EntrypointLabel, errors.New("PLUGIN global is required"))
	}
	plugin, ok := global.(*apiValue)
	if !ok || plugin == nil || plugin.kind != apiPlugin {
		return nil, evaluation.NewRuntimeError(evaluation.ErrorValidation, "setup", evaluation.EntrypointLabel, fmt.Errorf("PLUGIN must be mamacord.plugin, got %s", global.Type()))
	}
	declaration := plugin.data.(pluginDeclaration)
	if err := validateFunction(declaration.setup, "setup"); err != nil {
		return nil, evaluation.NewRuntimeError(evaluation.ErrorValidation, "setup", evaluation.EntrypointLabel, err)
	}

	limits := bundle.Limits()
	registrar := &botRegistrar{}
	execution := evaluation.New(ctx, "plugin-setup", limits.InitSteps, limits.InitTimeout, limits.MaxPrints, limits.MaxPrintBytes, print)
	result, err := starlarkgo.Call(execution.Thread(), declaration.setup, starlarkgo.Tuple{registrar}, nil)
	registrar.closed.Store(true)
	if err != nil {
		return nil, execution.Finish("setup", evaluation.EntrypointLabel, err)
	}
	if result != starlarkgo.None {
		execution.Close()
		return nil, evaluation.NewRuntimeError(evaluation.ErrorValidation, "setup", evaluation.EntrypointLabel, fmt.Errorf("setup must return None, got %s", result.Type()))
	}
	if err := execution.Finish("setup", evaluation.EntrypointLabel, nil); err != nil {
		return nil, err
	}
	if len(registrar.cogs) == 0 {
		return nil, evaluation.NewRuntimeError(evaluation.ErrorValidation, "setup", evaluation.EntrypointLabel, errors.New("setup must register at least one cog"))
	}

	lowerer := generationLowerer{id: id, routes: make(map[contract.RouteID]starlarkgo.Callable)}
	definition, err := lowerer.lower(registrar.cogs)
	if err != nil {
		return nil, evaluation.NewRuntimeError(evaluation.ErrorValidation, "setup", evaluation.EntrypointLabel, err)
	}
	catalog, err := definition.Compile()
	if err != nil {
		return nil, evaluation.NewRuntimeError(evaluation.ErrorValidation, "setup", evaluation.EntrypointLabel, err)
	}
	for _, callable := range lowerer.routes {
		callable.Freeze()
	}
	bundle.Freeze()
	return &SetupResult{ID: id, Limits: limits, Definition: definition, Catalog: catalog, Routes: lowerer.routes}, nil
}

type botRegistrar struct {
	closed atomic.Bool
	cogs   []*apiValue
}

func (registrar *botRegistrar) String() string { return "<mamacord bot registrar>" }
func (*botRegistrar) Type() string             { return "mamacord.bot" }
func (registrar *botRegistrar) Freeze()        { registrar.closed.Store(true) }
func (*botRegistrar) Truth() starlarkgo.Bool   { return starlarkgo.True }
func (*botRegistrar) Hash() (uint32, error)    { return 0, errors.New("unhashable: mamacord.bot") }
func (registrar *botRegistrar) Attr(name string) (starlarkgo.Value, error) {
	if name != "add_cog" {
		return nil, nil
	}
	return starlarkgo.NewBuiltin("bot.add_cog", func(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		if registrar.closed.Load() {
			return nil, errors.New("setup registrar is closed")
		}
		var value *apiValue
		if err := starlarkgo.UnpackArgs("bot.add_cog", args, kwargs, "cog", &value); err != nil {
			return nil, err
		}
		if value == nil || value.kind != apiCog {
			return nil, errors.New("bot.add_cog requires mamacord.cog")
		}
		if len(registrar.cogs) >= 100 {
			return nil, errors.New("setup exceeds 100 cogs")
		}
		registrar.cogs = append(registrar.cogs, value)
		return starlarkgo.None, nil
	}), nil
}
func (*botRegistrar) AttrNames() []string { return []string{"add_cog"} }

func validateFunction(callable starlarkgo.Callable, role string) error {
	function, ok := callable.(*starlarkgo.Function)
	if !ok || function == nil {
		return fmt.Errorf("%s must be a Starlark function", role)
	}
	if function.NumParams() != 1 || function.NumKwonlyParams() != 0 || function.HasVarargs() || function.HasKwargs() || function.ParamDefault(0) != nil {
		return fmt.Errorf("%s must accept exactly one required positional parameter", role)
	}
	return nil
}
