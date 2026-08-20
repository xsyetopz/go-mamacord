package contextapi_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/author"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/compile"
	contextapi "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/execution/context"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/generation"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/internal/evaluation"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

type resourceReaderStub struct{ values map[string][]byte }

func (reader resourceReaderStub) ReadResource(_ context.Context, path string) ([]byte, error) {
	value, ok := reader.values[path]
	if !ok {
		return nil, errors.New("missing resource")
	}
	return append([]byte(nil), value...), nil
}

func TestContextResourceRequiresCapabilityAndCanonicalDeclaredPath(t *testing.T) {
	source := `load("@mamacord//api.star", "cog", "plugin", "reply", "slash_command", "string_option")
def run(ctx):
    content = str(ctx.resource(ctx.option("path")))
    return [reply(content=content)]
def setup(bot):
    bot.add_cog(cog(name="Resources", commands=[slash_command(name="resource", description="Resource", handler=run, options=[string_option(name="path", description="Path", required=True)])]))
PLUGIN = plugin(setup=setup)
`
	generation := buildTestGeneration(t, source)
	invocation := contract.Invocation{InvocationIdentity: contract.InvocationIdentity{PluginID: "resource", Generation: "generation-1", Route: "command:slash:resource", Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}, InvocationActorContext: contract.InvocationActorContext{Author: &contract.UserRef{ID: "175928847299117063"}}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"resource"}, Options: []contract.OptionValue{{Name: "path", Kind: contract.OptionString, ScalarOptionValue: contract.ScalarOptionValue{String: "assets/message.txt"}}}}}}
	reader := resourceReaderStub{values: map[string][]byte{"assets/message.txt": []byte("hello")}}
	if _, err := generation.InvokeWithServices(context.Background(), invocation, contextapi.InvocationServices{Resources: reader}, nil); err == nil {
		t.Fatal("resource read without capability")
	}
	outcome, err := generation.InvokeWithServices(context.Background(), invocation, contextapi.InvocationServices{Resources: reader, Capabilities: []contract.Capability{contract.CapabilityResourcesRead}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	message, ok := outcome.Operations[0].(*contract.MessageOperation)
	if !ok {
		t.Fatalf("outcome=%#v", outcome)
	}
	if message.Message.Content != "hello" {
		t.Fatalf("content=%q", message.Message.Content)
	}
	for _, resourcePath := range []string{"../secret", "assets\\message.txt", "/assets/message.txt", "assets/missing.txt"} {
		invalid := invocation.DeepClone()
		invalid.Command.Options[0].String = resourcePath
		_, err := generation.InvokeWithServices(
			context.Background(),
			invalid,
			contextapi.InvocationServices{
				Resources:    reader,
				Capabilities: []contract.Capability{contract.CapabilityResourcesRead},
			},
			nil,
		)
		if err == nil {
			t.Errorf("resource path %q succeeded", resourcePath)
		}
	}
}

type memoryBundle struct{ sources map[string][]byte }

func (bundle *memoryBundle) ReadSource(label string, maxBytes int64) ([]byte, error) {
	source, ok := bundle.sources[label]
	if !ok {
		return nil, fmt.Errorf("source %q not found", label)
	}
	if int64(len(source)) > maxBytes {
		return nil, fmt.Errorf("source %q is too large", label)
	}
	return append([]byte(nil), source...), nil
}

func buildTestGeneration(t *testing.T, source string) *generation.Generation {
	t.Helper()
	compiled, err := compile.CompileBundle(context.Background(), &memoryBundle{sources: map[string][]byte{evaluation.EntrypointLabel: []byte(source)}}, evaluation.DefaultLimits())
	if err != nil {
		t.Fatalf("compile.CompileBundle: %v", err)
	}
	initialized, err := compiled.Initialize(context.Background(), author.AuthorAPI(), nil)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	result, err := generation.BuildInitialized(context.Background(), initialized, "generation-1", nil)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	return result
}
