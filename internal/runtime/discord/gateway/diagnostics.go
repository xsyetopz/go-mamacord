package gateway

import (
	"errors"
	"fmt"
	"strings"

	"github.com/disgoorg/disgo/gateway"
	"github.com/gorilla/websocket"
)

const discordCloseCodeDisallowedIntents = 4014

func DecorateOpenError(err error, intents gateway.Intents) error {
	if err == nil {
		return nil
	}

	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		return err
	}
	if closeErr.Code != discordCloseCodeDisallowedIntents {
		return err
	}

	requested := formatIntents(intents)
	privileged := formatPrivilegedIntents(intents)
	fix := privilegedIntentFix(intents)
	if fix == "" {
		fix = "Fix: in the Discord Developer Portal, enable the privileged gateway intents your bot requests (Application -> Bot -> Privileged Gateway Intents)."
	}

	msg := fmt.Sprintf(
		"discord gateway close %d (%s); requested_intents=%s; privileged_requested=%s; %s",
		closeErr.Code,
		strings.TrimSpace(closeErr.Text),
		requested,
		privileged,
		fix,
	)
	return fmt.Errorf("%w; %s", err, msg)
}

func formatIntents(intents gateway.Intents) string {
	// Keep the output stable and readable; we only list intents we actually know about
	// (current set + privileged ones) to avoid surprises when disgo adds new flags.
	known := []struct {
		name   string
		intent gateway.Intents
	}{
		{"Guilds", gateway.IntentGuilds},
		{"GuildMembers", gateway.IntentGuildMembers},
		{"GuildModeration", gateway.IntentGuildModeration},
		{"GuildInvites", gateway.IntentGuildInvites},
		{"DirectMessages", gateway.IntentDirectMessages},
		{"GuildPresences", gateway.IntentGuildPresences},
		{"MessageContent", gateway.IntentMessageContent},
	}

	out := make([]string, 0, len(known))
	for _, item := range known {
		if intents.Has(item.intent) {
			out = append(out, item.name)
		}
	}
	if len(out) == 0 {
		return "[]"
	}
	return "[" + strings.Join(out, " ") + "]"
}

func formatPrivilegedIntents(intents gateway.Intents) string {
	priv := intents & gateway.IntentsPrivileged
	if priv == gateway.IntentsNone {
		return "[]"
	}
	return formatIntents(priv)
}

func privilegedIntentFix(intents gateway.Intents) string {
	priv := intents & gateway.IntentsPrivileged
	if priv == gateway.IntentsNone {
		return ""
	}

	needs := make([]string, 0, 3)
	if priv.Has(gateway.IntentGuildMembers) {
		needs = append(needs, "Server Members Intent")
	}
	if priv.Has(gateway.IntentGuildPresences) {
		needs = append(needs, "Presence Intent")
	}
	if priv.Has(gateway.IntentMessageContent) {
		needs = append(needs, "Message Content Intent")
	}
	if len(needs) == 0 {
		return ""
	}
	return "Fix: enable " + strings.Join(needs, ", ") + " in the Discord Developer Portal (Application -> Bot -> Privileged Gateway Intents)."
}
