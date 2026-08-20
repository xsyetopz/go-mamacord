package starlark

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

const pluginGlobalName = "PLUGIN"

var (
	apiIDPattern          = regexp.MustCompile(`^[a-z][a-z0-9_.-]{0,63}$`)
	apiCommandNamePattern = regexp.MustCompile(`^[a-z0-9_-]{1,32}$`)
)

type Generation struct {
	id     contract.GenerationID
	limits Limits

	lifecycleMu  sync.Mutex
	accepting    bool
	active       int
	drained      chan struct{}
	drainClosed  bool
	released     bool
	retireCtx    context.Context
	retireCancel context.CancelCauseFunc

	definition contract.Definition
	catalog    *contract.RouteCatalog
	routes     map[contract.RouteID]starlarkgo.Callable
}

func (generation *Generation) ID() contract.GenerationID {
	if generation == nil {
		return ""
	}
	return generation.id
}
func (generation *Generation) Definition() contract.Definition {
	if generation == nil {
		return contract.Definition{}
	}
	generation.lifecycleMu.Lock()
	defer generation.lifecycleMu.Unlock()
	return generation.definition.DeepClone()
}

func (bundle *InitializedBundle) Setup(ctx context.Context, id contract.GenerationID, print func(string)) (*Generation, error) {
	if bundle == nil || bundle.entry == nil {
		return nil, newRuntimeError(ErrorSetup, "setup", EntrypointLabel, errors.New("initialized bundle is required"))
	}
	if strings.TrimSpace(string(id)) == "" {
		return nil, newRuntimeError(ErrorValidation, "setup", EntrypointLabel, errors.New("generation id is required"))
	}
	global, exists := bundle.entry[pluginGlobalName]
	if !exists {
		return nil, newRuntimeError(ErrorValidation, "setup", EntrypointLabel, errors.New("PLUGIN global is required"))
	}
	plugin, ok := global.(*apiValue)
	if !ok || plugin == nil || plugin.kind != apiPlugin {
		return nil, newRuntimeError(ErrorValidation, "setup", EntrypointLabel, fmt.Errorf("PLUGIN must be mamacord.plugin, got %s", global.Type()))
	}
	declaration := plugin.data.(pluginDeclaration)
	if err := validateFunction(declaration.setup, "setup"); err != nil {
		return nil, newRuntimeError(ErrorValidation, "setup", EntrypointLabel, err)
	}

	registrar := &botRegistrar{}
	execution := newThreadExecution(ctx, "plugin-setup", bundle.limits.InitSteps, bundle.limits.InitTimeout, bundle.limits.MaxPrints, bundle.limits.MaxPrintBytes, print)
	result, err := starlarkgo.Call(execution.thread, declaration.setup, starlarkgo.Tuple{registrar}, nil)
	registrar.closed.Store(true)
	if err != nil {
		return nil, execution.Finish("setup", EntrypointLabel, err)
	}
	if result != starlarkgo.None {
		execution.Close()
		return nil, newRuntimeError(ErrorValidation, "setup", EntrypointLabel, fmt.Errorf("setup must return None, got %s", result.Type()))
	}
	if err := execution.Finish("setup", EntrypointLabel, nil); err != nil {
		return nil, err
	}
	if len(registrar.cogs) == 0 {
		return nil, newRuntimeError(ErrorValidation, "setup", EntrypointLabel, errors.New("setup must register at least one cog"))
	}

	lowerer := generationLowerer{id: id, routes: make(map[contract.RouteID]starlarkgo.Callable)}
	definition, err := lowerer.lower(registrar.cogs)
	if err != nil {
		return nil, newRuntimeError(ErrorValidation, "setup", EntrypointLabel, err)
	}
	catalog, err := definition.Compile()
	if err != nil {
		return nil, newRuntimeError(ErrorValidation, "setup", EntrypointLabel, err)
	}
	for _, callable := range lowerer.routes {
		callable.Freeze()
	}
	bundle.Freeze()
	retireCtx, retireCancel := context.WithCancelCause(context.Background())
	return &Generation{
		id:           id,
		limits:       bundle.limits,
		accepting:    true,
		retireCtx:    retireCtx,
		retireCancel: retireCancel,
		drained:      make(chan struct{}),
		definition:   definition,
		catalog:      catalog,
		routes:       lowerer.routes,
	}, nil
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
