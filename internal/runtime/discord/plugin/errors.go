package plugin

import (
	"strings"

	"github.com/disgoorg/disgo/discord"

	commandtext "github.com/xsyetopz/go-mamacord/internal/commandtext"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
)

func ErrorMessage(prodMode bool, t commandtext.Translator, err error) discord.MessageCreate {
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
