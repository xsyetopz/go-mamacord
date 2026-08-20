package starlark

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

func buildGenerationWithID(t *testing.T, id, content string, print func(string)) *Generation {
	t.Helper()
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command", "reply")
def handle(ctx):
    return [reply(content="` + content + `")]
def setup(bot):
    bot.add_cog(cog(name="Swap", commands=[slash_command(name="swap", description="Swap", handler=handle)]))
PLUGIN=plugin(setup=setup)
`
	compiled, err := CompileBundle(context.Background(), &memoryBundle{sources: map[string][]byte{"//:plugin.star": []byte(source)}}, DefaultLimits())
	if err != nil {
		t.Fatal(err)
	}
	initialized, err := compiled.Initialize(context.Background(), AuthorAPI(), print)
	if err != nil {
		t.Fatal(err)
	}
	generation, err := initialized.Setup(context.Background(), contract.GenerationID(id), print)
	if err != nil {
		t.Fatal(err)
	}
	return generation
}

func TestGenerationManagerAtomicallySwapsAndRetires(t *testing.T) {
	t.Parallel()
	manager, err := NewGenerationManager(time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	old := buildGenerationWithID(t, "old", "old", nil)
	next := buildGenerationWithID(t, "next", "next", nil)
	if err := manager.Activate(old); err != nil {
		t.Fatal(err)
	}
	invocation := contract.Invocation{PluginID: "test", Route: "command:slash:swap", Kind: contract.InvocationCommand, ResponseState: contract.ResponseUnacknowledged, Author: &contract.UserRef{ID: "1"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"swap"}}}
	outcome, err := manager.Invoke(context.Background(), invocation, InvocationServices{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := outcome.Operations[0].(*contract.MessageOperation).Message.Content; got != "old" {
		t.Fatalf("old content=%q", got)
	}
	if err := manager.Activate(next); err != nil {
		t.Fatal(err)
	}
	outcome, err = manager.Invoke(context.Background(), invocation, InvocationServices{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := outcome.Operations[0].(*contract.MessageOperation).Message.Content; got != "next" {
		t.Fatalf("next content=%q", got)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := manager.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if manager.Current() != nil {
		t.Fatal("manager retained current generation")
	}
	old.lifecycleMu.Lock()
	oldReleased := old.released
	old.lifecycleMu.Unlock()
	next.lifecycleMu.Lock()
	nextReleased := next.released
	next.lifecycleMu.Unlock()
	if !oldReleased || !nextReleased {
		t.Fatal("retired generations were not released")
	}
}

func TestGenerationManagerCancelsCallbackAtRetirementDeadline(t *testing.T) {
	t.Parallel()
	started := make(chan struct{}, 1)
	printFn := func(message string) {
		if message == "started" {
			select {
			case started <- struct{}{}:
			default:
			}
		}
	}
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command", "reply")
def handle(ctx):
    print("started")
    total = 0
    for value in range(1000000000):
        total += value
    return [reply(content=str(total))]
def setup(bot):
    bot.add_cog(cog(name="Slow", commands=[slash_command(name="slow", description="Slow", handler=handle)]))
PLUGIN=plugin(setup=setup)
`
	limits := DefaultLimits()
	limits.InvokeTimeout = 5 * time.Second
	limits.InvokeSteps = 1000000000
	compiled, err := CompileBundle(context.Background(), &memoryBundle{sources: map[string][]byte{"//:plugin.star": []byte(source)}}, limits)
	if err != nil {
		t.Fatal(err)
	}
	initialized, err := compiled.Initialize(context.Background(), AuthorAPI(), printFn)
	if err != nil {
		t.Fatal(err)
	}
	old, err := initialized.Setup(context.Background(), "old", printFn)
	if err != nil {
		t.Fatal(err)
	}
	next := buildGenerationWithID(t, "next", "next", nil)
	retirementErrors := make(chan error, 2)
	manager, err := NewGenerationManager(10*time.Millisecond, func(err error) { retirementErrors <- err })
	if err != nil {
		t.Fatal(err)
	}
	if err := manager.Activate(old); err != nil {
		t.Fatal(err)
	}
	invocation := contract.Invocation{PluginID: "test", Route: "command:slash:slow", Kind: contract.InvocationCommand, ResponseState: contract.ResponseUnacknowledged, Author: &contract.UserRef{ID: "1"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"slow"}}}
	invokeDone := make(chan error, 1)
	go func() {
		_, err := manager.Invoke(context.Background(), invocation, InvocationServices{}, printFn)
		invokeDone <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("callback did not start")
	}
	if err := manager.Activate(next); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-invokeDone:
		if err == nil || !IsErrorKind(err, ErrorCanceled) {
			t.Fatalf("callback error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("callback was not canceled")
	}
	select {
	case err := <-retirementErrors:
		if !errors.Is(err, ErrRetirementDeadline) {
			t.Fatalf("retirement error=%v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("retirement result missing")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := manager.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
