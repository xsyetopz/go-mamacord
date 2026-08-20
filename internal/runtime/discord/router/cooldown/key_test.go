package cooldown

import (
	"testing"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func TestSlashCooldownKeyNormalizesCommandKindsAndPaths(t *testing.T) {
	t.Parallel()
	if got := SlashCooldownKey(nil, "  PiNg  "); got != "ping" {
		t.Fatalf("nil event key = %q", got)
	}

	sub, group := "Ban", "Admin"
	tests := []struct {
		name string
		data discord.ApplicationCommandInteractionData
		want string
	}{
		{name: "slash root", data: discord.SlashCommandInteractionData{}, want: "ping"},
		{name: "slash group", data: discord.SlashCommandInteractionData{SubCommandName: &sub, SubCommandGroupName: &group}, want: "ping:admin:ban"},
		{name: "user", data: discord.UserCommandInteractionData{}, want: "user:ping"},
		{name: "message", data: discord.MessageCommandInteractionData{}, want: "message:ping"},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			event := &events.ApplicationCommandInteractionCreate{ApplicationCommandInteraction: discord.ApplicationCommandInteraction{Data: test.data}}
			if got := SlashCooldownKey(event, "  PiNg  "); got != test.want {
				t.Fatalf("key = %q, want %q", got, test.want)
			}
		})
	}
}
