package generation

import (
	"context"
	"errors"
	"fmt"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/compile"
	contextapi "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/execution/context"
	"sync"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/author"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/execution"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/internal/evaluation"
	starlarkgo "go.starlark.net/starlark"
)

type Generation struct {
	id     contract.GenerationID
	limits evaluation.Limits
	generationLifecycle
	generationProgram
}

type generationLifecycle struct {
	lifecycleMu  sync.Mutex
	accepting    bool
	active       int
	drained      chan struct{}
	drainClosed  bool
	released     bool
	retireCtx    context.Context
	retireCancel context.CancelCauseFunc
}

type generationProgram struct {
	definition contract.Definition
	catalog    *contract.RouteCatalog
	routes     map[contract.RouteID]starlarkgo.Callable
}

func BuildInitialized(ctx context.Context, bundle *compile.InitializedBundle, id contract.GenerationID, print func(string)) (*Generation, error) {
	result, err := author.Setup(ctx, bundle, id, print)
	if err != nil {
		return nil, err
	}
	return newGeneration(result), nil
}

func newGeneration(result *author.SetupResult) *Generation {
	retireCtx, retireCancel := context.WithCancelCause(context.Background())
	return &Generation{
		id: result.ID, limits: result.Limits,
		generationLifecycle: generationLifecycle{
			accepting: true, drained: make(chan struct{}), retireCtx: retireCtx, retireCancel: retireCancel,
		},
		generationProgram: generationProgram{
			definition: result.Definition, catalog: result.Catalog, routes: result.Routes,
		},
	}
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

func (generation *Generation) Invoke(ctx context.Context, invocation contract.Invocation, print func(string)) (contract.Outcome, error) {
	return generation.InvokeWithServices(ctx, invocation, contextapi.InvocationServices{}, print)
}

func (generation *Generation) InvokeWithServices(ctx context.Context, invocation contract.Invocation, services contextapi.InvocationServices, print func(string)) (contract.Outcome, error) {
	if generation == nil {
		return contract.Outcome{}, evaluation.NewRuntimeError(evaluation.ErrorStale, "invoke", "", errors.New("generation is nil"))
	}
	if invocation.Generation != generation.id {
		return contract.Outcome{}, evaluation.NewRuntimeError(evaluation.ErrorStale, "invoke", string(invocation.Route), fmt.Errorf("invocation generation %q does not match %q", invocation.Generation, generation.id))
	}
	release, err := generation.acquire()
	if err != nil {
		return contract.Outcome{}, evaluation.NewRuntimeError(evaluation.ErrorStale, "invoke", string(invocation.Route), err)
	}
	defer release()
	ctx, stopRetirement := generation.invocationContext(ctx)
	defer stopRetirement()
	return execution.Invoke(ctx, generation.executionProgram(), invocation, services, print)
}

func (generation *Generation) Check(ctx context.Context, invocation contract.Invocation, print func(string)) (contract.CheckDecision, error) {
	return generation.CheckWithServices(ctx, invocation, contextapi.InvocationServices{}, print)
}

func (generation *Generation) CheckWithServices(ctx context.Context, invocation contract.Invocation, services contextapi.InvocationServices, print func(string)) (contract.CheckDecision, error) {
	if generation == nil {
		return contract.CheckDecision{}, evaluation.NewRuntimeError(evaluation.ErrorStale, "check", "", errors.New("generation is nil"))
	}
	if invocation.Generation != generation.id {
		return contract.CheckDecision{}, evaluation.NewRuntimeError(evaluation.ErrorStale, "check", string(invocation.Route), errors.New("stale generation"))
	}
	release, err := generation.acquire()
	if err != nil {
		return contract.CheckDecision{}, evaluation.NewRuntimeError(evaluation.ErrorStale, "check", string(invocation.Route), err)
	}
	defer release()
	ctx, stopRetirement := generation.invocationContext(ctx)
	defer stopRetirement()
	return execution.Check(ctx, generation.executionProgram(), invocation, services, print)
}

func (generation *Generation) executionProgram() execution.Program {
	return execution.Program{Limits: generation.limits, Catalog: generation.catalog, Routes: generation.routes}
}
