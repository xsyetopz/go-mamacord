package generation

import (
	"context"
	"errors"
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/author/effects"
	contextapi "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/execution/context"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/internal/evaluation"
	starlarkgo "go.starlark.net/starlark"
)

type executionProgram struct {
	Limits  evaluation.Limits
	Catalog *contract.RouteCatalog
	Routes  map[contract.RouteID]starlarkgo.Callable
}

func invokeProgram(ctx context.Context, program executionProgram, invocation contract.Invocation, services contextapi.InvocationServices, print func(string)) (contract.Outcome, error) {
	if invocation.Kind == contract.InvocationCheck {
		return contract.Outcome{}, evaluation.NewRuntimeError(evaluation.ErrorValidation, "invoke", string(invocation.Route), errors.New("check route requires Check"))
	}
	if program.Catalog == nil {
		return contract.Outcome{}, evaluation.NewRuntimeError(evaluation.ErrorStale, "invoke", string(invocation.Route), errors.New("route catalog is absent"))
	}
	if _, err := program.Catalog.Resolve(invocation); err != nil {
		return contract.Outcome{}, evaluation.NewRuntimeError(evaluation.ErrorValidation, "invoke", string(invocation.Route), err)
	}
	if len(invocation.State) != 0 && !services.Allows(contract.CapabilityStorageKV) {
		return contract.Outcome{}, evaluation.NewRuntimeError(evaluation.ErrorValidation, "invoke", string(invocation.Route), errors.New("state input requires storage.kv capability"))
	}
	callable, exists := program.Routes[invocation.Route]
	if !exists {
		return contract.Outcome{}, evaluation.NewRuntimeError(evaluation.ErrorStale, "invoke", string(invocation.Route), errors.New("route callable is absent"))
	}
	steps, timeout := program.Limits.InvokeSteps, program.Limits.InvokeTimeout
	if invocation.Kind == contract.InvocationAutocomplete {
		steps, timeout = program.Limits.CheckSteps, program.Limits.CheckTimeout
	}
	execution := evaluation.New(ctx, "plugin-"+string(invocation.Kind), steps, timeout, program.Limits.MaxPrints, program.Limits.MaxPrintBytes, print)
	result, err := starlarkgo.Call(execution.Thread(), callable, starlarkgo.Tuple{contextapi.NewValue(execution.Context(), invocation, services, program.Limits.MaxHostCalls)}, nil)
	if err != nil {
		return contract.Outcome{}, execution.Finish("invoke", string(invocation.Route), err)
	}
	if err := execution.Finish("invoke", string(invocation.Route), nil); err != nil {
		return contract.Outcome{}, err
	}
	outcome, err := effects.LowerOutcome(result, invocation)
	if err != nil {
		return contract.Outcome{}, evaluation.NewRuntimeError(evaluation.ErrorResult, "invoke", string(invocation.Route), err)
	}
	if err := program.Catalog.ValidateOutcome(invocation, outcome); err != nil {
		return contract.Outcome{}, evaluation.NewRuntimeError(evaluation.ErrorResult, "invoke", string(invocation.Route), err)
	}
	for _, operation := range outcome.Operations {
		for _, capability := range contract.RequiredCapabilities(operation) {
			if !services.Allows(capability) {
				return contract.Outcome{}, evaluation.NewRuntimeError(evaluation.ErrorValidation, "invoke", string(invocation.Route), fmt.Errorf("capability %q is not granted", capability))
			}
		}
	}
	return outcome, nil
}

func checkProgram(ctx context.Context, program executionProgram, invocation contract.Invocation, services contextapi.InvocationServices, print func(string)) (contract.CheckDecision, error) {
	if invocation.Kind != contract.InvocationCheck {
		return contract.CheckDecision{}, evaluation.NewRuntimeError(evaluation.ErrorValidation, "check", string(invocation.Route), errors.New("invocation is not a check"))
	}
	if program.Catalog == nil {
		return contract.CheckDecision{}, evaluation.NewRuntimeError(evaluation.ErrorStale, "check", string(invocation.Route), errors.New("route catalog is absent"))
	}
	if _, err := program.Catalog.Resolve(invocation); err != nil {
		return contract.CheckDecision{}, evaluation.NewRuntimeError(evaluation.ErrorValidation, "check", string(invocation.Route), err)
	}
	callable, exists := program.Routes[invocation.Route]
	if !exists {
		return contract.CheckDecision{}, evaluation.NewRuntimeError(evaluation.ErrorStale, "check", string(invocation.Route), errors.New("route callable is absent"))
	}
	execution := evaluation.New(ctx, "plugin-check", program.Limits.CheckSteps, program.Limits.CheckTimeout, program.Limits.MaxPrints, program.Limits.MaxPrintBytes, print)
	result, err := starlarkgo.Call(execution.Thread(), callable, starlarkgo.Tuple{contextapi.NewValue(execution.Context(), invocation, services, program.Limits.MaxHostCalls)}, nil)
	if err != nil {
		return contract.CheckDecision{}, execution.Finish("check", string(invocation.Route), err)
	}
	if err := execution.Finish("check", string(invocation.Route), nil); err != nil {
		return contract.CheckDecision{}, err
	}
	decision, err := effects.LowerCheckDecision(result)
	if err != nil {
		return contract.CheckDecision{}, evaluation.NewRuntimeError(evaluation.ErrorResult, "check", string(invocation.Route), err)
	}
	if err := program.Catalog.ValidateCheckDecision(invocation, decision); err != nil {
		return contract.CheckDecision{}, evaluation.NewRuntimeError(evaluation.ErrorResult, "check", string(invocation.Route), err)
	}
	return decision, nil
}
