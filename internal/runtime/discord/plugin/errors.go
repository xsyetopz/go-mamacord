package plugin

import (
	"strings"

	"github.com/disgoorg/disgo/discord"

	commandapi "github.com/xsyetopz/go-mamacord/internal/commands/api"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
)

func ErrorMessage(prodMode bool, t commandapi.Translator, err error) discord.MessageCreate {
	if prodMode {
		return interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true)
	}

	body := strings.TrimSpace(err.Error())
	if body == "" {
		body = unknownErrText
	}

	return interactions.NoticeMessage(interactions.KindError, "Plugin response rejected", body, true)
}

const unknownErrText = "UNKNOWN"
