package plugin

import (
	"errors"
	"strings"

	luaplugin "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

type AutomationAction struct {
	Type      string
	ChannelID string
	GuildID   string
	UserID    string
	UntilUnix int64
	Message   luaplugin.EncodedValue
}

func ParseAutomationActions(raw luaplugin.EncodedValue) ([]AutomationAction, error) {
	decoded, err := raw.Decode()
	if err != nil {
		return nil, err
	}

	m, ok := decoded.(map[string]any)
	if !ok {
		return nil, errors.New("automation response must be an object")
	}
	actionsRaw, ok := m["actions"]
	if !ok {
		return nil, errors.New("automation response missing actions")
	}
	list, ok := actionsRaw.([]any)
	if !ok {
		return nil, errors.New("actions must be an array")
	}
	if len(list) == 0 {
		return nil, nil
	}

	out := make([]AutomationAction, 0, len(list))
	for _, item := range list {
		im, isMap := item.(map[string]any)
		if !isMap {
			return nil, errors.New("action must be an object")
		}
		typ, _ := im["type"].(string)
		typ = strings.ToLower(strings.TrimSpace(typ))
		if typ == "" {
			return nil, errors.New("action missing type")
		}
		ch, _ := im["channel_id"].(string)
		guildID, _ := im["guild_id"].(string)
		uid, _ := im["user_id"].(string)
		untilUnix, _ := asInt64(im, "until_unix")
		message, err := luaplugin.EncodeValue(im["message"])
		if err != nil {
			return nil, errors.New("action message must be JSON encodable")
		}
		out = append(out, AutomationAction{
			Type:      typ,
			ChannelID: strings.TrimSpace(ch),
			GuildID:   strings.TrimSpace(guildID),
			UserID:    strings.TrimSpace(uid),
			UntilUnix: untilUnix,
			Message:   message,
		})
	}
	return out, nil
}
