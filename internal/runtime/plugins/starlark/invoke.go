package starlark

import (
	"context"
	"errors"
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

func (generation *Generation) Invoke(ctx context.Context, invocation contract.Invocation, print func(string)) (contract.Outcome, error) {
	return generation.InvokeWithServices(ctx, invocation, InvocationServices{}, print)
}

func (generation *Generation) InvokeWithServices(ctx context.Context, invocation contract.Invocation, services InvocationServices, print func(string)) (contract.Outcome, error) {
	if generation == nil {
		return contract.Outcome{}, newRuntimeError(ErrorStale, "invoke", "", errors.New("generation is nil"))
	}
	if invocation.Generation != generation.id {
		return contract.Outcome{}, newRuntimeError(ErrorStale, "invoke", string(invocation.Route), fmt.Errorf("invocation generation %q does not match %q", invocation.Generation, generation.id))
	}
	if invocation.Kind == contract.InvocationCheck {
		return contract.Outcome{}, newRuntimeError(ErrorValidation, "invoke", string(invocation.Route), errors.New("check route requires Check"))
	}
	release, err := generation.acquire()
	if err != nil {
		return contract.Outcome{}, newRuntimeError(ErrorStale, "invoke", string(invocation.Route), err)
	}
	defer release()
	ctx, stopRetirement := generation.invocationContext(ctx)
	defer stopRetirement()
	if _, err := generation.catalog.Resolve(invocation); err != nil {
		return contract.Outcome{}, newRuntimeError(ErrorValidation, "invoke", string(invocation.Route), err)
	}
	if len(invocation.State) != 0 && !services.allows(contract.CapabilityStorageKV) {
		return contract.Outcome{}, newRuntimeError(ErrorValidation, "invoke", string(invocation.Route), errors.New("state input requires storage.kv capability"))
	}
	callable, exists := generation.routes[invocation.Route]
	if !exists {
		return contract.Outcome{}, newRuntimeError(ErrorStale, "invoke", string(invocation.Route), errors.New("route callable is absent"))
	}
	steps, timeout := generation.limits.InvokeSteps, generation.limits.InvokeTimeout
	if invocation.Kind == contract.InvocationAutocomplete {
		steps, timeout = generation.limits.CheckSteps, generation.limits.CheckTimeout
	}
	execution := newThreadExecution(ctx, "plugin-"+string(invocation.Kind), steps, timeout, generation.limits.MaxPrints, generation.limits.MaxPrintBytes, print)
	result, err := starlarkgo.Call(execution.thread, callable, starlarkgo.Tuple{newContextValue(execution.context, invocation, services, generation.limits.MaxHostCalls)}, nil)
	if err != nil {
		return contract.Outcome{}, execution.Finish("invoke", string(invocation.Route), err)
	}
	if err := execution.Finish("invoke", string(invocation.Route), nil); err != nil {
		return contract.Outcome{}, err
	}
	outcome, err := lowerOutcome(result, invocation)
	if err != nil {
		return contract.Outcome{}, newRuntimeError(ErrorResult, "invoke", string(invocation.Route), err)
	}
	if err := generation.catalog.ValidateOutcome(invocation, outcome); err != nil {
		return contract.Outcome{}, newRuntimeError(ErrorResult, "invoke", string(invocation.Route), err)
	}
	for _, operation := range outcome.Operations {
		for _, capability := range contract.RequiredCapabilities(operation) {
			if !services.allows(capability) {
				return contract.Outcome{}, newRuntimeError(ErrorValidation, "invoke", string(invocation.Route), fmt.Errorf("capability %q is not granted", capability))
			}
		}
	}
	return outcome, nil
}

func (generation *Generation) Check(ctx context.Context, invocation contract.Invocation, print func(string)) (contract.CheckDecision, error) {
	return generation.CheckWithServices(ctx, invocation, InvocationServices{}, print)
}

func (generation *Generation) CheckWithServices(ctx context.Context, invocation contract.Invocation, services InvocationServices, print func(string)) (contract.CheckDecision, error) {
	if generation == nil {
		return contract.CheckDecision{}, newRuntimeError(ErrorStale, "check", "", errors.New("generation is nil"))
	}
	if invocation.Generation != generation.id {
		return contract.CheckDecision{}, newRuntimeError(ErrorStale, "check", string(invocation.Route), errors.New("stale generation"))
	}
	if invocation.Kind != contract.InvocationCheck {
		return contract.CheckDecision{}, newRuntimeError(ErrorValidation, "check", string(invocation.Route), errors.New("invocation is not a check"))
	}
	release, err := generation.acquire()
	if err != nil {
		return contract.CheckDecision{}, newRuntimeError(ErrorStale, "check", string(invocation.Route), err)
	}
	defer release()
	ctx, stopRetirement := generation.invocationContext(ctx)
	defer stopRetirement()
	if _, err := generation.catalog.Resolve(invocation); err != nil {
		return contract.CheckDecision{}, newRuntimeError(ErrorValidation, "check", string(invocation.Route), err)
	}
	callable, exists := generation.routes[invocation.Route]
	if !exists {
		return contract.CheckDecision{}, newRuntimeError(ErrorStale, "check", string(invocation.Route), errors.New("route callable is absent"))
	}
	execution := newThreadExecution(ctx, "plugin-check", generation.limits.CheckSteps, generation.limits.CheckTimeout, generation.limits.MaxPrints, generation.limits.MaxPrintBytes, print)
	result, err := starlarkgo.Call(execution.thread, callable, starlarkgo.Tuple{newContextValue(execution.context, invocation, services, generation.limits.MaxHostCalls)}, nil)
	if err != nil {
		return contract.CheckDecision{}, execution.Finish("check", string(invocation.Route), err)
	}
	if err := execution.Finish("check", string(invocation.Route), nil); err != nil {
		return contract.CheckDecision{}, err
	}
	decision, err := lowerCheckDecision(result)
	if err != nil {
		return contract.CheckDecision{}, newRuntimeError(ErrorResult, "check", string(invocation.Route), err)
	}
	if err := generation.catalog.ValidateCheckDecision(invocation, decision); err != nil {
		return contract.CheckDecision{}, newRuntimeError(ErrorResult, "check", string(invocation.Route), err)
	}
	return decision, nil
}

func lowerOutcome(result starlarkgo.Value, invocation contract.Invocation) (contract.Outcome, error) {
	if result == starlarkgo.None {
		return contract.Outcome{}, nil
	}
	iterable, ok := result.(starlarkgo.Iterable)
	if !ok {
		return contract.Outcome{}, fmt.Errorf("handler must return a list or tuple of effects, got %s", result.Type())
	}
	iterator := iterable.Iterate()
	defer iterator.Done()
	outcome := contract.Outcome{}
	var item starlarkgo.Value
	for iterator.Next(&item) {
		if len(outcome.Operations) >= contract.MaxOutcomeOperations {
			return contract.Outcome{}, fmt.Errorf("handler result exceeds %d operations", contract.MaxOutcomeOperations)
		}
		value, ok := item.(*apiValue)
		if !ok || value == nil || value.kind != apiEffect {
			return contract.Outcome{}, fmt.Errorf("handler result item must be mamacord.effect, got %s", item.Type())
		}
		operation, err := lowerEffect(value.data.(effectDeclaration), invocation)
		if err != nil {
			return contract.Outcome{}, err
		}
		outcome.Operations = append(outcome.Operations, operation)
	}
	return outcome, nil
}
func lowerEffect(effect effectDeclaration, invocation contract.Invocation) (contract.Operation, error) {
	switch effect.kind {
	case effectReply:
		value := effect.data.(replyDeclaration)
		switch invocation.ResponseState {
		case contract.ResponseUnacknowledged:
			return &contract.MessageOperation{Message: value.message.DeepClone(), Ephemeral: value.ephemeral}, nil
		case contract.ResponseDeferredCreate:
			return &contract.EditResponseOperation{Patch: messageAsPatch(value.message)}, nil
		case contract.ResponseDeferredUpdate:
			return &contract.UpdateOperation{Patch: messageAsPatch(value.message)}, nil
		default:
			return nil, fmt.Errorf("reply is invalid for response state %q", invocation.ResponseState)
		}
	case effectUpdate:
		return &contract.UpdateOperation{Patch: effect.data.(updateDeclaration).patch.DeepClone()}, nil
	case effectModal:
		value := effect.data.(contract.ModalView)
		return &contract.ModalOperation{Modal: value.DeepClone()}, nil
	case effectKVPut:
		value := effect.data.(kvPutDeclaration)
		return &contract.KVPutOperation{Key: value.key, Value: value.value.Clone(), ExpectedVersion: cloneUint64(value.expectedVersion)}, nil
	case effectKVDelete:
		value := effect.data.(struct {
			key      string
			expected *uint64
		})
		return &contract.KVDeleteOperation{Key: value.key, ExpectedVersion: cloneUint64(value.expected)}, nil
	case effectAutocomplete:
		value := effect.data.(autocompleteDeclaration)
		return &contract.AutocompleteChoicesOperation{Choices: append([]contract.AutocompleteChoice(nil), value.choices...)}, nil
	case effectBestEffort:
		operation, err := lowerEffect(effect.data.(effectDeclaration), invocation)
		if err != nil {
			return nil, err
		}
		return &contract.BestEffortOperation{Operation: operation}, nil
	case effectGuarded:
		value := effect.data.(guardedEffectDeclaration)
		operation, err := lowerEffect(value.operation, invocation)
		if err != nil {
			return nil, err
		}
		failure, err := lowerEffect(value.failure, invocation)
		if err != nil {
			return nil, err
		}
		return &contract.GuardedOperation{Operation: operation, Failure: failure}, nil
	case effectDomain:
		operation, ok := effect.data.(contract.Operation)
		if !ok || operation == nil {
			return nil, errors.New("domain effect has invalid operation")
		}
		return contract.Outcome{Operations: []contract.Operation{operation}}.DeepClone().Operations[0], nil
	default:
		return nil, fmt.Errorf("unsupported effect kind %q", effect.kind)
	}
}
func messageAsPatch(message contract.Message) contract.MessagePatch {
	patch := contract.MessagePatch{Content: contract.OptionalString{Set: true, Value: message.Content}}
	if message.Embeds != nil {
		patch.Embeds = contract.OptionalEmbeds{Set: true, Values: message.DeepClone().Embeds}
	}
	if message.Components != nil {
		patch.Components = contract.OptionalComponentRows{Set: true, Values: message.DeepClone().Components}
	}
	return patch
}
func lowerCheckDecision(result starlarkgo.Value) (contract.CheckDecision, error) {
	if boolean, ok := result.(starlarkgo.Bool); ok {
		if bool(boolean) {
			return contract.AllowedCheck(), nil
		}
		return contract.SilentDeniedCheck(), nil
	}
	value, ok := result.(*apiValue)
	if !ok || value == nil || value.kind != apiEffect {
		return contract.CheckDecision{}, fmt.Errorf("check must return bool or reply effect, got %s", result.Type())
	}
	effect := value.data.(effectDeclaration)
	if effect.kind != effectReply {
		return contract.CheckDecision{}, fmt.Errorf("check denial must be a reply effect")
	}
	reply := effect.data.(replyDeclaration)
	return contract.DeniedCheck(&contract.MessageOperation{Message: reply.message.DeepClone(), Ephemeral: reply.ephemeral}), nil
}

func cloneUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
