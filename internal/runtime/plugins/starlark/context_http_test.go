package starlark

import (
	"context"
	"strings"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

type stubHTTPClient struct {
	calls    int
	url      string
	maxBytes int64
}

func (client *stubHTTPClient) GetJSON(_ context.Context, rawURL string, maxBytes int64) (contract.Value, bool, error) {
	client.calls++
	client.url = rawURL
	client.maxBytes = maxBytes
	value, err := contract.ObjectValue([]contract.Field{{Key: "response", Value: contract.StringValue("https://cdn.example/gif.gif")}})
	return value, true, err
}

func TestContextHTTPRequiresCapabilityAndDeclaredExactHost(t *testing.T) {
	t.Parallel()
	source := `load("@mamacord//api.star", "plugin", "cog", "slash_command", "reply")
def handle(ctx):
    result = ctx.http_get_json(url="https://kawaii.red/api/gif/hug?token=anonymous", max_bytes=1024)
    return [reply(content=result["response"])]
def setup(bot):
    bot.add_cog(cog(name="HTTP", commands=[slash_command(name="http", description="HTTP", handler=handle)]))
PLUGIN=plugin(setup=setup)
`
	generation := buildTestGeneration(t, source)
	invocation := contract.Invocation{PluginID: "fun", Generation: generation.ID(), Route: "command:slash:http", Kind: contract.InvocationCommand, ResponseState: contract.ResponseUnacknowledged, Author: &contract.UserRef{ID: "1"}, Command: &contract.CommandInput{Kind: contract.CommandSlash, Path: []string{"http"}}}
	client := &stubHTTPClient{}
	services := InvocationServices{HTTP: client, HTTPHosts: []string{"kawaii.red"}, Capabilities: []contract.Capability{contract.CapabilityNetworkHTTP}}
	outcome, err := generation.InvokeWithServices(context.Background(), invocation, services, nil)
	if err != nil {
		t.Fatal(err)
	}
	if client.calls != 1 || client.url != "https://kawaii.red/api/gif/hug?token=anonymous" || client.maxBytes != 1024 {
		t.Fatalf("client=%#v", client)
	}
	if outcome.Operations[0].(*contract.MessageOperation).Message.Content != "https://cdn.example/gif.gif" {
		t.Fatalf("outcome=%#v", outcome)
	}
	services.HTTPHosts = []string{"example.com"}
	if _, err := generation.InvokeWithServices(context.Background(), invocation, services, nil); err == nil || !IsErrorKind(err, ErrorInvocation) {
		t.Fatalf("undeclared host: %v", err)
	}
	if client.calls != 1 {
		t.Fatal("HTTP client called for undeclared host")
	}
}

func TestAuthorizedHTTPURLRejectsAmbientTargets(t *testing.T) {
	t.Parallel()
	tests := []string{"http://kawaii.red/x", "https://user@kawaii.red/x", "https://kawaii.red:8443/x", "https://127.0.0.1/x", "https://metadata.internal/x", "https://kawaii.red/x#fragment", "https://kawaii.red./x", "https://kawaii.red/" + strings.Repeat("x", 4097)}
	for _, raw := range tests {
		if _, err := authorizedHTTPURL(raw, []string{"kawaii.red"}); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
