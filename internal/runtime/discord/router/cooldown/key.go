package cooldown

import (
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
)

func SlashCooldownKey(e *events.ApplicationCommandInteractionCreate, cmdName string) string {
	key := strings.ToLower(strings.TrimSpace(cmdName))
	if key == "" || e == nil {
		return key
	}
	if e.Data.Type() != discord.ApplicationCommandTypeSlash {
		switch e.Data.Type() {
		case discord.ApplicationCommandTypeUser:
			return "user:" + key
		case discord.ApplicationCommandTypeMessage:
			return "message:" + key
		default:
			return key
		}
	}
	data := e.SlashCommandInteractionData()

	group := ""
	if data.SubCommandGroupName != nil {
		group = strings.ToLower(strings.TrimSpace(*data.SubCommandGroupName))
	}

	sub := ""
	if data.SubCommandName != nil {
		sub = strings.ToLower(strings.TrimSpace(*data.SubCommandName))
	}
	if sub == "" {
		return key
	}

	if group != "" {
		return key + ":" + group + ":" + sub
	}
	return key + ":" + sub
}
