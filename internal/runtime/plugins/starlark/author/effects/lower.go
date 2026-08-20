package effects

import (
	"errors"
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

func LowerOutcome(result starlarkgo.Value, invocation contract.Invocation) (contract.Outcome, error) {
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
		value, ok := item.(*authoredValue)
		if !ok || value == nil || value.kind != valueEffect {
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
func LowerCheckDecision(result starlarkgo.Value) (contract.CheckDecision, error) {
	if boolean, ok := result.(starlarkgo.Bool); ok {
		if bool(boolean) {
			return contract.AllowedCheck(), nil
		}
		return contract.SilentDeniedCheck(), nil
	}
	value, ok := result.(*authoredValue)
	if !ok || value == nil || value.kind != valueEffect {
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
