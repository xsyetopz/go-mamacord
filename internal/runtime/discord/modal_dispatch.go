package discordruntime

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/disgo/events"

	commandapi "github.com/xsyetopz/go-mamacord/internal/commands/api"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	discordplugin "github.com/xsyetopz/go-mamacord/internal/runtime/discord/plugin"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/router"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

func (b *Bot) onModal(e *events.ModalSubmitInteractionCreate) {
	ctx := context.Background()
	b.incInteraction()

	locale := e.Locale()
	t := commandapi.Translator{Registry: b.i18n, Locale: locale, UserID: uint64(e.User().ID)}

	customID := strings.TrimSpace(e.Data.CustomID)
	if d := b.modalCooldown(customID); d > 0 {
		if remaining, ok := b.cooldowns.Take(uint64(e.User().ID), modalCooldownKey(customID), d, time.Now()); !ok {
			_ = e.CreateMessage(
				interactions.NoticeMessage(
					interactions.KindWarning,
					"",
					t.S("err.cooldown", map[string]any{"Seconds": cooldownSecs(remaining)}),
					true,
				),
			)
			return
		}
	}

	pluginID, localID, ok := pluginhost.ParseCustomID(customID)
	if !ok {
		_ = e.Acknowledge()
		return
	}
	if !b.moduleEnabled(pluginID) {
		_ = e.Acknowledge()
		return
	}
	if guildID := e.GuildID(); guildID != nil {
		enabled, err := b.guildPluginEnabled(ctx, uint64(*guildID), pluginID)
		if err != nil {
			b.incInteractionFailure()
			b.incPluginFailure()
			b.logger.Error("plugin modal permission check failed", slog.String("custom_id", customID), slog.String("err", err.Error()))
			_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
			return
		}
		if !enabled {
			_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindWarning, "", "This plugin is disabled in this server.", true))
			return
		}
	}
	route, ok := b.pluginRoutes[pluginID]
	if !ok {
		_ = e.Acknowledge()
		return
	}

	res, hasValue, err := route.Host.HandleModal(ctx, pluginID, localID, pluginhost.Payload{
		GuildID:   router.SnowflakePtrToString(e.GuildID()),
		ChannelID: e.Channel().ID().String(),
		UserID:    e.User().ID.String(),
		Locale:    locale.Code(),
		Options:   router.ModalOptions(e, pluginID),
	})
	if err != nil {
		b.incInteractionFailure()
		b.incPluginFailure()
		b.logger.Error("plugin modal failed", slog.String("custom_id", customID), slog.String("err", err.Error()))
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
		return
	}

	if !hasValue {
		_ = e.Acknowledge()
		return
	}

	action, err := discordplugin.ParseAction(pluginID, res, false, discordplugin.ResponseModalSubmit)
	if err != nil {
		b.incInteractionFailure()
		b.incPluginFailure()
		b.logger.Error(
			"plugin modal response parse failed",
			slog.String("custom_id", customID),
			slog.String("err", err.Error()),
		)
		_ = e.CreateMessage(discordplugin.ErrorMessage(b.prodMode, t, err))
		return
	}

	switch action.Kind {
	case discordplugin.ActionNone:
		_ = e.Acknowledge()
	case discordplugin.ActionModal:
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
	case discordplugin.ActionUpdate:
		_ = e.UpdateMessage(action.Update)
	case discordplugin.ActionMessage:
		_ = e.CreateMessage(action.Create)
	default:
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
	}
}
