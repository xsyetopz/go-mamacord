package starlark

import (
	"context"
	"errors"
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
	invocation := contract.Invocation{PluginID: "resource", Generation: "generation-1", Route: "command:slash:resource", Kind: contract.InvocationCommand, ResponseState: contract.ResponseUnacknowledged, Author: &contract.UserRef{ID: "175928847299117063"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"resource"}, Options: []contract.OptionValue{{Name: "path", Kind: contract.OptionString, String: "assets/message.txt"}}}}
	reader := resourceReaderStub{values: map[string][]byte{"assets/message.txt": []byte("hello")}}
	if _, err := generation.InvokeWithServices(context.Background(), invocation, InvocationServices{Resources: reader}, nil); err == nil {
		t.Fatal("resource read without capability")
	}
	outcome, err := generation.InvokeWithServices(context.Background(), invocation, InvocationServices{Resources: reader, Capabilities: []contract.Capability{contract.CapabilityResourcesRead}}, nil)
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
			InvocationServices{
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
