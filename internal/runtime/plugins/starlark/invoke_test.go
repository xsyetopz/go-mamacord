package starlark

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

func TestGenerationInvokesTypedEffects(t *testing.T) {
	t.Parallel()
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command", "option", "component", "modal", "modal_field", "custom_check", "reply", "update", "button", "row", "kv_put", "autocomplete_choice", "autocomplete_choices", "embed", "select", "select_option")

def policy(ctx):
    return False

def complete(ctx):
    return [autocomplete_choices([autocomplete_choice(name="One", value=1)])]

def command_handler(ctx):
    current = ctx.state("counter", {"count": 0})
    return [kv_put(key="counter", value={"count": current["count"] + 1}), reply(content="hello " + ctx.option("name"), embeds=[embed(description="example")], components=[row([button(handler="save", label="Save")]), row([select(handler="save", kind="string", options=[select_option(label="One", value="one")])])])]

def component_handler(ctx):
    return [update(content="updated")]

def modal_handler(ctx):
    return [reply(content=ctx.modal_fields["text"], ephemeral=True)]

def setup(bot):
    bot.add_cog(cog(
        name="Example",
        commands=[slash_command(name="example", description="Example", handler=command_handler, options=[option(kind="string", name="name", description="Name", required=True), option(kind="integer", name="count", description="Count", autocomplete=complete)], checks=[custom_check(id="policy", handler=policy)])],
        components=[component(id="save", handler=component_handler, kinds=["button", "string_select"])],
        modals=[modal(id="edit", handler=modal_handler, fields=[modal_field(id="text", required=True)])],
    ))
PLUGIN = plugin(setup=setup)
`
	generation := buildTestGeneration(t, source)
	command := contract.Invocation{PluginID: "example", Generation: generation.ID(), Route: "command:slash:example", Kind: contract.InvocationCommand, ResponseState: contract.ResponseUnacknowledged, Guild: &contract.GuildRef{ID: "10"}, State: []contract.StateEntry{{Key: "counter", Value: mustObjectValue(t, []contract.Field{{Key: "count", Value: contract.IntValue(1)}})}}, Author: &contract.UserRef{ID: "30"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"example"}, Options: []contract.OptionValue{{Name: "name", Kind: contract.OptionString, String: "Mamacord"}}}}
	outcome, err := generation.InvokeWithServices(context.Background(), command, InvocationServices{Capabilities: []contract.Capability{contract.CapabilityStorageKV}}, nil)
	if err != nil {
		t.Fatalf("Invoke command: %v", err)
	}
	if len(outcome.Operations) != 2 {
		t.Fatalf("operations: %#v", outcome.Operations)
	}
	if put, ok := outcome.Operations[0].(*contract.KVPutOperation); !ok || put.Key != "counter" {
		t.Fatalf("put: %#v", outcome.Operations[0])
	}
	message, ok := outcome.Operations[1].(*contract.MessageOperation)
	if !ok || message.Message.Content != "hello Mamacord" {
		t.Fatalf("message: %#v", outcome.Operations[1])
	}

	component := contract.Invocation{PluginID: "example", Generation: generation.ID(), Route: "component:save", Kind: contract.InvocationComponent, ResponseState: contract.ResponseUnacknowledged, Guild: &contract.GuildRef{ID: "10"}, Author: &contract.UserRef{ID: "30"}, Component: &contract.ComponentInput{ID: "save", Kind: contract.ComponentButton}}
	outcome, err = generation.Invoke(context.Background(), component, nil)
	if err != nil {
		t.Fatalf("Invoke component: %v", err)
	}
	if _, ok := outcome.Operations[0].(*contract.UpdateOperation); !ok {
		t.Fatalf("component operation: %#v", outcome.Operations[0])
	}

	modal := contract.Invocation{PluginID: "example", Generation: generation.ID(), Route: "modal:edit", Kind: contract.InvocationModal, ResponseState: contract.ResponseUnacknowledged, ModalOrigin: contract.ModalOriginCommand, Author: &contract.UserRef{ID: "30"}, Modal: &contract.ModalInput{ID: "edit", Fields: []contract.NamedString{{Name: "text", Value: "saved"}}}}
	outcome, err = generation.Invoke(context.Background(), modal, nil)
	if err != nil {
		t.Fatalf("Invoke modal: %v", err)
	}
	if reply := outcome.Operations[0].(*contract.MessageOperation); reply.Message.Content != "saved" || !reply.Ephemeral {
		t.Fatalf("modal reply: %#v", reply)
	}

	autocomplete := contract.Invocation{PluginID: "example", Generation: generation.ID(), Route: "autocomplete:slash:example:count", Kind: contract.InvocationAutocomplete, ResponseState: contract.ResponseUnacknowledged, Author: &contract.UserRef{ID: "30"}, Autocomplete: &contract.AutocompleteInput{Path: []string{"example"}, Option: "count", Focused: contract.OptionValue{Name: "count", Kind: contract.OptionInteger, Integer: 1}}}
	outcome, err = generation.Invoke(context.Background(), autocomplete, nil)
	if err != nil {
		t.Fatalf("Invoke autocomplete: %v", err)
	}
	if _, ok := outcome.Operations[0].(*contract.AutocompleteChoicesOperation); !ok {
		t.Fatalf("autocomplete operation: %#v", outcome.Operations[0])
	}

	check := contract.Invocation{PluginID: "example", Generation: generation.ID(), Route: "check:policy", Kind: contract.InvocationCheck, Author: &contract.UserRef{ID: "30"}, Check: &contract.CheckInput{}}
	decision, err := generation.Check(context.Background(), check, nil)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if decision.Kind != contract.CheckDeniedSilent {
		t.Fatalf("decision: %#v", decision)
	}
}

func TestGenerationReturnsAuthorizedDomainEffects(t *testing.T) {
	t.Parallel()
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command", "set_slowmode", "reply")
def handle(ctx):
    return [set_slowmode(channel_id=ctx.channel["id"], seconds=0), reply(content="done")]
def setup(bot):
    bot.add_cog(cog(name="Manager", commands=[slash_command(name="slowmode", description="Slowmode", handler=handle)]))
PLUGIN=plugin(setup=setup)
`
	generation := buildTestGeneration(t, source)
	invocation := contract.Invocation{PluginID: "manager", Generation: generation.ID(), Route: "command:slash:slowmode", Kind: contract.InvocationCommand, ResponseState: contract.ResponseUnacknowledged, Guild: &contract.GuildRef{ID: "10"}, Channel: &contract.ChannelRef{ID: "20", GuildID: "10", Kind: contract.ChannelText}, Author: &contract.UserRef{ID: "30"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"slowmode"}}}
	if _, err := generation.Invoke(context.Background(), invocation, nil); err == nil || !IsErrorKind(err, ErrorValidation) {
		t.Fatalf("ungranted domain effect: %v", err)
	}
	outcome, err := generation.InvokeWithServices(context.Background(), invocation, InvocationServices{Capabilities: []contract.Capability{contract.CapabilityDiscordChannels}}, nil)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	operation, ok := outcome.Operations[0].(*contract.SetSlowmodeOperation)
	if !ok || operation.ChannelID != "20" || operation.Seconds != 0 {
		t.Fatalf("operation: %#v", outcome.Operations[0])
	}
}

func TestGenerationLowersPredeferredReply(t *testing.T) {
	t.Parallel()
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command", "reply")
def handle(ctx):
    return [reply(content="done")]
def setup(bot):
    bot.add_cog(cog(name="Deferred", commands=[slash_command(name="deferred", description="Deferred", handler=handle, defer="create")]))
PLUGIN=plugin(setup=setup)
`
	generation := buildTestGeneration(t, source)
	invocation := contract.Invocation{PluginID: "example", Generation: generation.ID(), Route: "command:slash:deferred", Kind: contract.InvocationCommand, ResponseState: contract.ResponseDeferredCreate, Author: &contract.UserRef{ID: "1"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"deferred"}}}
	outcome, err := generation.Invoke(context.Background(), invocation, nil)
	if err != nil {
		t.Fatalf("Invoke: %v", err)
	}
	if _, ok := outcome.Operations[0].(*contract.EditResponseOperation); !ok {
		t.Fatalf("operation: %#v", outcome.Operations[0])
	}
}

func TestGenerationRejectsStaleAndInvalidResults(t *testing.T) {
	t.Parallel()
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command")
def handle(ctx):
    return {"not": "effects"}
def setup(bot):
    bot.add_cog(cog(name="Bad", commands=[slash_command(name="bad", description="Bad", handler=handle)]))
PLUGIN=plugin(setup=setup)
`
	generation := buildTestGeneration(t, source)
	invocation := contract.Invocation{PluginID: "example", Generation: generation.ID(), Route: "command:slash:bad", Kind: contract.InvocationCommand, ResponseState: contract.ResponseUnacknowledged, Author: &contract.UserRef{ID: "1"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"bad"}}}
	if _, err := generation.Invoke(context.Background(), invocation, nil); err == nil || !IsErrorKind(err, ErrorResult) {
		t.Fatalf("invalid result error: %v", err)
	}
	invocation.Generation = "old"
	if _, err := generation.Invoke(context.Background(), invocation, nil); err == nil || !IsErrorKind(err, ErrorStale) {
		t.Fatalf("stale error: %v", err)
	}
}

func TestGenerationConcurrentInvocation(t *testing.T) {
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command", "reply")
def handle(ctx):
    return [reply(content="ok")]
def setup(bot):
    bot.add_cog(cog(name="Concurrent", commands=[slash_command(name="concurrent", description="Concurrent", handler=handle)]))
PLUGIN=plugin(setup=setup)
`
	generation := buildTestGeneration(t, source)
	invocation := contract.Invocation{PluginID: "example", Generation: generation.ID(), Route: "command:slash:concurrent", Kind: contract.InvocationCommand, ResponseState: contract.ResponseUnacknowledged, Author: &contract.UserRef{ID: "1"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"concurrent"}}}
	var wait sync.WaitGroup
	errors := make(chan error, 32)
	for range 32 {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := generation.Invoke(context.Background(), invocation, nil)
			errors <- err
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Errorf("Invoke: %v", err)
		}
	}
}

func TestGenerationInvocationLimitsAndCancellation(t *testing.T) {
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command", "reply")
def handle(ctx):
    total = 0
    for i in range(1000000000):
        total += i
    return [reply(content=str(total))]
def setup(bot):
    bot.add_cog(cog(name="Bounded", commands=[slash_command(name="bounded", description="Bounded", handler=handle)]))
PLUGIN=plugin(setup=setup)
`
	invocation := contract.Invocation{PluginID: "example", Generation: "generation-1", Route: "command:slash:bounded", Kind: contract.InvocationCommand, ResponseState: contract.ResponseUnacknowledged, Author: &contract.UserRef{ID: "1"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"bounded"}}}

	stepGeneration := buildTestGeneration(t, source)
	stepGeneration.limits.InvokeSteps = 100
	stepGeneration.limits.InvokeTimeout = time.Second
	if _, err := stepGeneration.Invoke(context.Background(), invocation, nil); err == nil || !IsErrorKind(err, ErrorStepLimit) {
		t.Fatalf("step limit error: %v", err)
	}

	deadlineGeneration := buildTestGeneration(t, source)
	deadlineGeneration.limits.InvokeSteps = ^uint64(0)
	deadlineGeneration.limits.InvokeTimeout = time.Millisecond
	if _, err := deadlineGeneration.Invoke(context.Background(), invocation, nil); err == nil || !IsErrorKind(err, ErrorDeadline) {
		t.Fatalf("deadline error: %v", err)
	}

	canceledGeneration := buildTestGeneration(t, source)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := canceledGeneration.Invoke(ctx, invocation, nil); err == nil || !IsErrorKind(err, ErrorCanceled) {
		t.Fatalf("caller cancellation error: %v", err)
	}
}

type testLocalizer struct{ calls int }

func (localizer *testLocalizer) Localize(_ context.Context, request LocalizationRequest) (string, error) {
	localizer.calls++
	if request.PluginID != "example" || request.Locale != "en-US" || request.MessageID != "greeting" || request.Data.Kind() != contract.ValueObject {
		return "", fmt.Errorf("unexpected request: %#v", request)
	}
	return "Hello", nil
}

func TestContextLocalizationIsExplicitAndBounded(t *testing.T) {
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command", "reply")
def handle(ctx):
    first = ctx.t("greeting", {"Name": "Mamacord"})
    second = ctx.t("greeting", {})
    return [reply(content=first + second)]
def setup(bot):
    bot.add_cog(cog(name="Localized", commands=[slash_command(name="localized", description="Localized", handler=handle)]))
PLUGIN=plugin(setup=setup)
`
	generation := buildTestGeneration(t, source)
	invocation := contract.Invocation{PluginID: "example", Generation: generation.ID(), Route: "command:slash:localized", Kind: contract.InvocationCommand, ResponseState: contract.ResponseUnacknowledged, Locale: "en-US", Author: &contract.UserRef{ID: "1"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"localized"}}}
	localizer := &testLocalizer{}
	outcome, err := generation.InvokeWithServices(context.Background(), invocation, InvocationServices{Localizer: localizer}, nil)
	if err != nil {
		t.Fatalf("InvokeWithServices: %v", err)
	}
	if localizer.calls != 2 || outcome.Operations[0].(*contract.MessageOperation).Message.Content != "HelloHello" {
		t.Fatalf("calls=%d outcome=%#v", localizer.calls, outcome)
	}
	generation.limits.MaxHostCalls = 1
	if _, err := generation.InvokeWithServices(context.Background(), invocation, InvocationServices{Localizer: localizer}, nil); err == nil || !IsErrorKind(err, ErrorInvocation) {
		t.Fatalf("host-call limit error: %v", err)
	}
}

func TestGenerationDrainAndRelease(t *testing.T) {
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command", "reply")
def handle(ctx):
    return [reply(content="ok")]
def setup(bot):
    bot.add_cog(cog(name="Drain", commands=[slash_command(name="drain", description="Drain", handler=handle)]))
PLUGIN=plugin(setup=setup)
`
	generation := buildTestGeneration(t, source)
	started := make(chan struct{})
	unblock := make(chan struct{})
	generation.routes["command:slash:drain"] = starlarkgo.NewBuiltin("blocked", func(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, _ starlarkgo.Tuple, _ []starlarkgo.Tuple) (starlarkgo.Value, error) {
		close(started)
		<-unblock
		return starlarkgo.NewList([]starlarkgo.Value{effectValue(effectReply, replyDeclaration{message: contract.Message{Content: "done"}})}), nil
	})
	invocation := contract.Invocation{PluginID: "example", Generation: generation.ID(), Route: "command:slash:drain", Kind: contract.InvocationCommand, ResponseState: contract.ResponseUnacknowledged, Author: &contract.UserRef{ID: "1"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"drain"}}}
	result := make(chan error, 1)
	go func() { _, err := generation.Invoke(context.Background(), invocation, nil); result <- err }()
	<-started
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := generation.Drain(ctx); err == nil {
		t.Fatal("drain completed while invocation was active")
	}
	if _, err := generation.Invoke(context.Background(), invocation, nil); err == nil || !IsErrorKind(err, ErrorStale) {
		t.Fatalf("new invocation during drain: %v", err)
	}
	close(unblock)
	if err := <-result; err != nil {
		t.Fatalf("active invocation: %v", err)
	}
	if err := generation.Drain(context.Background()); err != nil {
		t.Fatalf("Drain: %v", err)
	}
	if err := generation.Release(); err != nil {
		t.Fatalf("Release: %v", err)
	}
	if len(generation.Definition().Cogs) != 0 {
		t.Fatal("released generation retained definition")
	}
	if _, err := generation.Invoke(context.Background(), invocation, nil); err == nil || !IsErrorKind(err, ErrorStale) {
		t.Fatalf("released invocation: %v", err)
	}
}

func mustObjectValue(t *testing.T, fields []contract.Field) contract.Value {
	t.Helper()
	value, err := contract.ObjectValue(fields)
	if err != nil {
		t.Fatalf("ObjectValue: %v", err)
	}
	return value
}

func buildTestGeneration(t *testing.T, source string) *Generation {
	t.Helper()
	compiled, err := CompileBundle(context.Background(), &memoryBundle{sources: map[string][]byte{EntrypointLabel: []byte(source)}}, DefaultLimits())
	if err != nil {
		t.Fatalf("CompileBundle: %v", err)
	}
	initialized, err := compiled.Initialize(context.Background(), AuthorAPI(), nil)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	generation, err := initialized.Setup(context.Background(), "generation-1", nil)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	return generation
}
