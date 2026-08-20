package generation

import (
	"context"
	"errors"
	"sync"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

var errGenerationRetired = errors.New("generation retirement canceled active callback")

func (generation *Generation) invocationContext(parent context.Context) (context.Context, func()) {
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancelCause(parent)
	stop := context.AfterFunc(generation.retireCtx, func() { cancel(errGenerationRetired) })
	return ctx, func() { stop(); cancel(nil) }
}

func (generation *Generation) cancelActive() {
	if generation != nil && generation.retireCancel != nil {
		generation.retireCancel(errGenerationRetired)
	}
}

func (generation *Generation) acquire() (func(), error) {
	if generation == nil {
		return nil, errors.New("generation is nil")
	}
	generation.lifecycleMu.Lock()
	if !generation.accepting || generation.released {
		generation.lifecycleMu.Unlock()
		return nil, errors.New("generation is draining or released")
	}
	generation.active++
	generation.lifecycleMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			generation.lifecycleMu.Lock()
			generation.active--
			generation.closeDrainedLocked()
			generation.lifecycleMu.Unlock()
		})
	}, nil
}

func (generation *Generation) BeginDrain() {
	if generation == nil {
		return
	}
	generation.lifecycleMu.Lock()
	generation.accepting = false
	generation.closeDrainedLocked()
	generation.lifecycleMu.Unlock()
}

func (generation *Generation) Drain(ctx context.Context) error {
	if generation == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	generation.BeginDrain()
	generation.lifecycleMu.Lock()
	drained := generation.drained
	generation.lifecycleMu.Unlock()
	select {
	case <-drained:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (generation *Generation) Release() error {
	if generation == nil {
		return nil
	}
	generation.lifecycleMu.Lock()
	defer generation.lifecycleMu.Unlock()
	if generation.released {
		return nil
	}
	if generation.accepting || generation.active != 0 {
		return errors.New("generation must drain before release")
	}
	generation.released = true
	if generation.retireCancel != nil {
		generation.retireCancel(nil)
	}
	generation.routes = nil
	generation.catalog = nil
	generation.definition = contract.Definition{}
	return nil
}

func (generation *Generation) closeDrainedLocked() {
	if generation.accepting || generation.active != 0 || generation.drainClosed {
		return
	}
	close(generation.drained)
	generation.drainClosed = true
}
