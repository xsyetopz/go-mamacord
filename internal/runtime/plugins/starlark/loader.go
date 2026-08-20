package starlark

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	starlarkgo "go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

type CompiledBundle struct {
	limits   Limits
	programs map[string]*starlarkgo.Program
	loads    map[string][]string
}

type InitializedBundle struct {
	limits  Limits
	entry   starlarkgo.StringDict
	modules map[string]starlarkgo.StringDict
}

func CompileBundle(ctx context.Context, bundle SourceBundle, limits Limits) (*CompiledBundle, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if bundle == nil {
		return nil, newRuntimeError(ErrorSource, "compile", EntrypointLabel, errors.New("source bundle is required"))
	}
	if err := limits.Validate(); err != nil {
		return nil, newRuntimeError(ErrorValidation, "compile", "", err)
	}
	compiler := bundleCompiler{
		ctx: ctx, bundle: bundle, limits: limits,
		programs: make(map[string]*starlarkgo.Program),
		loads:    make(map[string][]string), state: make(map[string]compileState),
	}
	if err := compiler.compile(EntrypointLabel, nil); err != nil {
		return nil, err
	}
	return &CompiledBundle{limits: limits, programs: compiler.programs, loads: compiler.loads}, nil
}

type compileState uint8

const (
	compileUnseen compileState = iota
	compileActive
	compileDone
)

type bundleCompiler struct {
	ctx        context.Context
	bundle     SourceBundle
	limits     Limits
	programs   map[string]*starlarkgo.Program
	loads      map[string][]string
	state      map[string]compileState
	totalBytes int64
}

func (compiler *bundleCompiler) compile(label string, stack []string) error {
	if err := compiler.ctx.Err(); err != nil {
		return newRuntimeError(ErrorCanceled, "compile", label, err)
	}
	if compiler.state[label] == compileDone {
		return nil
	}
	if compiler.state[label] == compileActive {
		cycleStart := slices.Index(stack, label)
		cycle := append(append([]string(nil), stack[max(cycleStart, 0):]...), label)
		return newRuntimeError(ErrorLoad, "compile", label, fmt.Errorf("module load cycle: %s", strings.Join(cycle, " -> ")))
	}
	if len(stack)+1 > compiler.limits.MaxLoadDepth {
		return newRuntimeError(ErrorLoad, "compile", label, fmt.Errorf("module load depth exceeds %d", compiler.limits.MaxLoadDepth))
	}
	if len(compiler.programs) >= compiler.limits.MaxModules {
		return newRuntimeError(ErrorLoad, "compile", label, fmt.Errorf("bundle exceeds %d modules", compiler.limits.MaxModules))
	}

	canonical, err := CanonicalBundleLabel(label)
	if err != nil {
		return newRuntimeError(ErrorLoad, "compile", label, err)
	}
	source, err := compiler.bundle.ReadSource(canonical, compiler.limits.MaxFileBytes)
	if err != nil {
		return newRuntimeError(ErrorSource, "compile", canonical, err)
	}
	if int64(len(source)) > compiler.limits.MaxTotalSourceBytes-compiler.totalBytes {
		return newRuntimeError(ErrorSource, "compile", canonical, fmt.Errorf("bundle source exceeds %d aggregate bytes", compiler.limits.MaxTotalSourceBytes))
	}
	compiler.totalBytes += int64(len(source))

	_, program, err := starlarkgo.SourceProgramOptions(&syntax.FileOptions{}, canonical, source, func(string) bool { return false })
	if err != nil {
		return newRuntimeError(ErrorCompile, "compile", canonical, sanitizeEvaluationError(err))
	}
	compiler.state[canonical] = compileActive
	compiler.programs[canonical] = program
	stack = append(stack, canonical)
	loads := make([]string, 0, program.NumLoads())
	for index := 0; index < program.NumLoads(); index++ {
		loaded, _ := program.Load(index)
		if loaded == APIModuleLabel {
			loads = append(loads, loaded)
			continue
		}
		loadedCanonical, err := CanonicalBundleLabel(loaded)
		if err != nil {
			return newRuntimeError(ErrorLoad, "compile", canonical, fmt.Errorf("load %q: %w", loaded, err))
		}
		loads = append(loads, loadedCanonical)
		if err := compiler.compile(loadedCanonical, stack); err != nil {
			return err
		}
	}
	compiler.loads[canonical] = loads
	compiler.state[canonical] = compileDone
	return nil
}

func (bundle *CompiledBundle) Initialize(ctx context.Context, api starlarkgo.StringDict, print func(string)) (*InitializedBundle, error) {
	if bundle == nil || bundle.programs[EntrypointLabel] == nil {
		return nil, newRuntimeError(ErrorInitialize, "initialize", EntrypointLabel, errors.New("compiled bundle is not initialized"))
	}
	apiOwned := cloneStringDict(api)
	for _, value := range apiOwned {
		if value == nil {
			return nil, newRuntimeError(ErrorInitialize, "initialize", APIModuleLabel, errors.New("API module contains a nil value"))
		}
		value.Freeze()
	}
	execution := newThreadExecution(ctx, "plugin-initialize", bundle.limits.InitSteps, bundle.limits.InitTimeout, bundle.limits.MaxPrints, bundle.limits.MaxPrintBytes, print)
	loader := moduleInitializer{
		compiled: bundle, api: apiOwned, execution: execution,
		states: make(map[string]moduleState), modules: make(map[string]starlarkgo.StringDict),
	}
	execution.thread.Load = loader.load
	entry, err := bundle.programs[EntrypointLabel].Init(execution.thread, nil)
	if err != nil {
		classified := execution.Finish("initialize", EntrypointLabel, err)
		if loader.failure != nil && !IsErrorKind(classified, ErrorStepLimit) && !IsErrorKind(classified, ErrorDeadline) && !IsErrorKind(classified, ErrorCanceled) {
			return nil, loader.failure
		}
		return nil, classified
	}
	if err := execution.Finish("initialize", EntrypointLabel, nil); err != nil {
		return nil, err
	}
	return &InitializedBundle{limits: bundle.limits, entry: entry, modules: loader.modules}, nil
}

type moduleState uint8

const (
	moduleUnseen moduleState = iota
	moduleActive
	moduleReady
)

type moduleInitializer struct {
	compiled  *CompiledBundle
	api       starlarkgo.StringDict
	execution *threadExecution
	states    map[string]moduleState
	modules   map[string]starlarkgo.StringDict
	stack     []string
	failure   error
}

func (loader *moduleInitializer) load(_ *starlarkgo.Thread, label string) (starlarkgo.StringDict, error) {
	if loader.failure != nil {
		return nil, loader.failure
	}
	if label == APIModuleLabel {
		return cloneStringDict(loader.api), nil
	}
	canonical, err := CanonicalBundleLabel(label)
	if err != nil {
		return nil, loader.fail(label, err)
	}
	program := loader.compiled.programs[canonical]
	if program == nil {
		return nil, loader.fail(canonical, errors.New("module was not present in the compiled graph"))
	}
	switch loader.states[canonical] {
	case moduleReady:
		return loader.modules[canonical], nil
	case moduleActive:
		return nil, loader.fail(canonical, fmt.Errorf("module initialization cycle: %s -> %s", strings.Join(loader.stack, " -> "), canonical))
	}
	loader.states[canonical] = moduleActive
	loader.stack = append(loader.stack, canonical)
	globals, err := program.Init(loader.execution.thread, nil)
	loader.stack = loader.stack[:len(loader.stack)-1]
	if err != nil {
		loader.states[canonical] = moduleUnseen
		return nil, loader.fail(canonical, err)
	}
	globals.Freeze()
	loader.modules[canonical] = globals
	loader.states[canonical] = moduleReady
	return globals, nil
}

func (loader *moduleInitializer) fail(source string, err error) error {
	failure := newRuntimeError(ErrorLoad, "initialize", source, sanitizeEvaluationError(err))
	loader.failure = failure
	return failure
}

func (bundle *InitializedBundle) Freeze() {
	if bundle == nil {
		return
	}
	bundle.entry.Freeze()
}

func cloneStringDict(values starlarkgo.StringDict) starlarkgo.StringDict {
	if values == nil {
		return nil
	}
	out := make(starlarkgo.StringDict, len(values))
	for name, value := range values {
		out[name] = value
	}
	return out
}
