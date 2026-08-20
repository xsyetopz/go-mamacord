package author

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/compile"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/starlark/internal/evaluation"
)

func TestAuthorAPIStagesAndLowersDefinition(t *testing.T) {
	t.Parallel()
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command", "group", "subcommand", "option", "choice", "component", "modal", "modal_field", "listener", "task", "guild_only", "has_permissions")

def handle(ctx):
    return []

def complete(ctx):
    return []

def setup(bot):
    create = subcommand(
        name="create",
        description="Create a tool",
        handler=handle,
        options=[
            option(kind="string", name="text", description="Text", required=True, min_length=1, max_length=100),
            option(kind="integer", name="count", description="Count", autocomplete=complete),
        ],
    )
    tools = group(name="tool", description="Tool commands", children=[create])
    bot.add_cog(cog(
        name="Tools",
        checks=[guild_only()],
        commands=[tools],
        components=[component(id="save", handler=handle, kinds=["button"], checks=[has_permissions(["manage_messages"])])],
        modals=[modal(id="edit", handler=handle, fields=[modal_field(id="text", required=True)])],
        listeners=[listener(id="join", event="guild_member_join", handler=handle)],
        tasks=[task(id="sweep", schedule="*/5 * * * *", handler=handle)],
    ))

PLUGIN = plugin(setup=setup)
ENTRY_MUTABLE = []
`
	bundle := &memoryBundle{sources: map[string][]byte{evaluation.EntrypointLabel: []byte(source)}}
	compiled, err := compile.CompileBundle(context.Background(), bundle, evaluation.DefaultLimits())
	if err != nil {
		t.Fatalf("compile.CompileBundle: %v", err)
	}
	initialized, err := compiled.Initialize(context.Background(), AuthorAPI(), nil)
	if err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	result, err := Setup(context.Background(), initialized, "generation-1", nil)
	if err != nil {
		t.Fatalf("Setup: %v", err)
	}
	definition := result.Definition
	if err := definition.Validate(); err != nil {
		t.Fatalf("Definition: %v", err)
	}
	if len(definition.Cogs) != 1 || len(definition.Cogs[0].Commands) != 1 {
		t.Fatalf("definition: %#v", definition)
	}
	leaf := definition.Cogs[0].Commands[0].Children[0]
	if leaf.Route != "command:slash:tool.create" || leaf.Options[1].Autocomplete != "autocomplete:slash:tool.create:count" {
		t.Fatalf("leaf: %#v", leaf)
	}
	if len(result.Routes) != 6 {
		t.Fatalf("routes = %d, want 6", len(result.Routes))
	}
	if _, err := result.Catalog.Resolve(contract.Invocation{InvocationIdentity: contract.InvocationIdentity{PluginID: "test", Generation: result.ID, Route: leaf.Route, Kind: contract.InvocationCommand}, InvocationInteractionContext: contract.InvocationInteractionContext{ResponseState: contract.ResponseUnacknowledged}, InvocationActorContext: contract.InvocationActorContext{Author: &contract.UserRef{ID: "1"}}, InvocationInput: contract.InvocationInput{Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"tool", "create"}, Options: []contract.OptionValue{{Name: "text", Kind: contract.OptionString, ScalarOptionValue: contract.ScalarOptionValue{String: "hello"}}}}}}); err != nil {
		t.Fatalf("catalog resolve: %v", err)
	}
}

func TestSetupRejectsInvalidPluginShapes(t *testing.T) {
	t.Parallel()
	tests := []struct{ name, source, contains string }{
		{name: "missing plugin", source: "x = 1\n", contains: "PLUGIN global is required"},
		{name: "wrong plugin", source: "PLUGIN = 1\n", contains: "PLUGIN must be"},
		{name: "setup arity", source: `load("@mamacord//api.star", "plugin")
def setup(bot, extra):
    pass
PLUGIN = plugin(setup=setup)
`, contains: "exactly one"},
		{name: "no cogs", source: `load("@mamacord//api.star", "plugin")
def setup(bot):
    pass
PLUGIN = plugin(setup=setup)
`, contains: "at least one cog"},
		{name: "bad handler", source: `load("@mamacord//api.star", "plugin", "cog", "slash_command")
def handler(ctx, extra):
    pass
def setup(bot):
    bot.add_cog(cog(name="Bad", commands=[slash_command(name="bad", description="Bad", handler=handler)]))
PLUGIN = plugin(setup=setup)
`, contains: "exactly one"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			compiled, err := compile.CompileBundle(context.Background(), &memoryBundle{sources: map[string][]byte{evaluation.EntrypointLabel: []byte(test.source)}}, evaluation.DefaultLimits())
			if err != nil {
				t.Fatalf("compile.CompileBundle: %v", err)
			}
			initialized, err := compiled.Initialize(context.Background(), AuthorAPI(), nil)
			if err != nil {
				t.Fatalf("Initialize: %v", err)
			}
			_, err = Setup(context.Background(), initialized, "generation-1", nil)
			if err == nil || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("error = %v, want %q", err, test.contains)
			}
		})
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
