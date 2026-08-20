package starlark

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

var ErrRetirementDeadline = errors.New("generation retirement deadline exceeded")

type GenerationBuilder struct {
	Limits Limits
	Print  func(string)
}

func (builder GenerationBuilder) Build(ctx context.Context, source SourceBundle, id contract.GenerationID) (*Generation, error) {
	compiled, err := CompileBundle(ctx, source, builder.Limits)
	if err != nil {
		return nil, err
	}
	initialized, err := compiled.Initialize(ctx, AuthorAPI(), builder.Print)
	if err != nil {
		return nil, err
	}
	return initialized.Setup(ctx, id, builder.Print)
}

type GenerationManager struct {
	mu                sync.RWMutex
	current           *Generation
	retirementTimeout time.Duration
	onRetirementError func(error)
	pending           int
	retired           chan struct{}
}

func NewGenerationManager(retirementTimeout time.Duration, onRetirementError func(error)) (*GenerationManager, error) {
	if retirementTimeout <= 0 {
		return nil, errors.New("retirement timeout must be positive")
	}
	retired := make(chan struct{})
	close(retired)
	return &GenerationManager{retirementTimeout: retirementTimeout, onRetirementError: onRetirementError, retired: retired}, nil
}

func (manager *GenerationManager) Current() *Generation {
	if manager == nil {
		return nil
	}
	manager.mu.RLock()
	defer manager.mu.RUnlock()
	return manager.current
}

func (manager *GenerationManager) Activate(next *Generation) error {
	if manager == nil {
		return errors.New("generation manager is nil")
	}
	if next == nil {
		return errors.New("next generation is required")
	}
	next.lifecycleMu.Lock()
	ready := next.accepting && !next.released
	next.lifecycleMu.Unlock()
	if !ready {
		return errors.New("next generation is not accepting callbacks")
	}
	manager.mu.Lock()
	if manager.current == next {
		manager.mu.Unlock()
		return nil
	}
	if manager.current != nil && manager.current.ID() == next.ID() {
		manager.mu.Unlock()
		return fmt.Errorf("generation id %q is already active", next.ID())
	}
	previous := manager.current
	manager.current = next
	if previous != nil {
		manager.beginRetirementLocked()
	}
	manager.mu.Unlock()
	if previous != nil {
		previous.BeginDrain()
		go manager.finishRetirement(previous)
	}
	return nil
}

func (manager *GenerationManager) Invoke(ctx context.Context, invocation contract.Invocation, services InvocationServices, print func(string)) (contract.Outcome, error) {
	if manager == nil {
		return contract.Outcome{}, newRuntimeError(ErrorStale, "invoke", string(invocation.Route), errors.New("generation manager is nil"))
	}
	for attempt := 0; attempt < 2; attempt++ {
		generation := manager.Current()
		if generation == nil {
			return contract.Outcome{}, newRuntimeError(ErrorStale, "invoke", string(invocation.Route), errors.New("no active generation"))
		}
		invocation.Generation = generation.ID()
		outcome, err := generation.InvokeWithServices(ctx, invocation, services, print)
		if !IsErrorKind(err, ErrorStale) || manager.Current() == generation {
			return outcome, err
		}
	}
	return contract.Outcome{}, newRuntimeError(ErrorStale, "invoke", string(invocation.Route), errors.New("active generation changed during dispatch"))
}
func (manager *GenerationManager) Check(ctx context.Context, invocation contract.Invocation, services InvocationServices, print func(string)) (contract.CheckDecision, error) {
	if manager == nil {
		return contract.CheckDecision{}, newRuntimeError(ErrorStale, "check", string(invocation.Route), errors.New("generation manager is nil"))
	}
	for attempt := 0; attempt < 2; attempt++ {
		generation := manager.Current()
		if generation == nil {
			return contract.CheckDecision{}, newRuntimeError(ErrorStale, "check", string(invocation.Route), errors.New("no active generation"))
		}
		invocation.Generation = generation.ID()
		decision, err := generation.CheckWithServices(ctx, invocation, services, print)
		if !IsErrorKind(err, ErrorStale) || manager.Current() == generation {
			return decision, err
		}
	}
	return contract.CheckDecision{}, newRuntimeError(ErrorStale, "check", string(invocation.Route), errors.New("active generation changed during dispatch"))
}

func (manager *GenerationManager) Close(ctx context.Context) error {
	if manager == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	manager.mu.Lock()
	current := manager.current
	manager.current = nil
	if current != nil {
		manager.beginRetirementLocked()
	}
	retired := manager.retired
	manager.mu.Unlock()
	if current != nil {
		current.BeginDrain()
		go manager.finishRetirement(current)
	}
	select {
	case <-retired:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (manager *GenerationManager) beginRetirementLocked() {
	if manager.pending == 0 {
		manager.retired = make(chan struct{})
	}
	manager.pending++
}
func (manager *GenerationManager) finishRetirement(generation *Generation) {
	err := retireGeneration(generation, manager.retirementTimeout)
	manager.mu.Lock()
	manager.pending--
	if manager.pending == 0 {
		close(manager.retired)
	}
	callback := manager.onRetirementError
	manager.mu.Unlock()
	if err != nil && callback != nil {
		callback(err)
	}
}

func retireGeneration(generation *Generation, timeout time.Duration) error {
	generation.BeginDrain()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	err := generation.Drain(ctx)
	cancel()
	if err == nil {
		return generation.Release()
	}
	generation.cancelActive()
	cleanupTimeout := generation.limits.InvokeTimeout + time.Second
	cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), cleanupTimeout)
	cleanupErr := generation.Drain(cleanupCtx)
	cleanupCancel()
	if cleanupErr != nil {
		return errors.Join(ErrRetirementDeadline, err, cleanupErr)
	}
	if releaseErr := generation.Release(); releaseErr != nil {
		return errors.Join(ErrRetirementDeadline, err, releaseErr)
	}
	return errors.Join(ErrRetirementDeadline, err)
}
