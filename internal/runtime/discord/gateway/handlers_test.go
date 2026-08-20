package gateway

import (
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"testing"
)

func TestGatewayPluginInvocationProjectsBoundedUserData(t *testing.T) {
	name := "Tester"
	invocation := gatewayPluginInvocation("175928847299117063", discord.User{ID: snowflake.ID(175928847299117064), Username: "tester", GlobalName: &name}, EventMemberJoin, true)
	if invocation.Author == nil || invocation.Author.Name != "Tester" || !invocation.IsOwner || invocation.Event == nil || invocation.Event.Name != EventMemberJoin {
		t.Fatalf("invocation=%#v", invocation)
	}
	if err := invocation.Event.Data.Validate(); err != nil {
		t.Fatal(err)
	}
}
