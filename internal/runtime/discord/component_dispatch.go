package discordruntime

import (
	"context"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	commandtext "github.com/xsyetopz/go-mamacord/internal/commandtext"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	discordplugin "github.com/xsyetopz/go-mamacord/internal/runtime/discord/plugin"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/router"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

func (b *Bot) onComponent(e *events.ComponentInteractionCreate) {
	ctx := context.Background()
	b.incInteraction()

	locale := e.Locale()
	t := commandtext.Translator{Registry: b.i18n, Locale: locale.Code(), UserID: uint64(e.User().ID)}

	customID := e.Data.CustomID()
	if !b.takeComponentCooldown(e, t, customID, time.Now()) {
		return
	}

	b.handlePluginComponent(ctx, e, t, locale, customID)
}

func (b *Bot) takeComponentCooldown(
	e *events.ComponentInteractionCreate,
	t commandtext.Translator,
	customID string,
	now time.Time,
) bool {
	if d := b.componentCooldown(customID); d > 0 {
		if remaining, ok := b.cooldowns.Take(uint64(e.User().ID), componentCooldownKey(customID), d, now); !ok {
			msg := interactions.NoticeMessage(
				interactions.KindWarning,
				"",
				t.S("err.cooldown", map[string]any{"Seconds": cooldownSecs(remaining)}),
				true,
			)
			_ = e.CreateMessage(msg)
			return false
		}
	}
	return true
}

func (b *Bot) handlePluginComponent(
	ctx context.Context,
	e *events.ComponentInteractionCreate,
	t commandtext.Translator,
	locale discord.Locale,
	customID string,
) {
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
			b.logger.ErrorContext(ctx, "plugin component permission check failed", slog.String("custom_id", customID), slog.String("err", err.Error()))
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

	res, hasValue, err := route.Host.HandleComponent(ctx, pluginID, localID, pluginhost.Payload{
		GuildID:   router.SnowflakePtrToString(e.GuildID()),
		ChannelID: e.Channel().ID().String(),
		UserID:    e.User().ID.String(),
		Locale:    locale.Code(),
		Options:   router.ComponentOptions(e),
	})
	if err != nil {
		b.incInteractionFailure()
		b.incPluginFailure()
		b.logger.ErrorContext(
			ctx,
			"plugin component failed",
			slog.String("custom_id", customID),
			slog.String("err", err.Error()),
		)
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
		return
	}

	if !hasValue {
		_ = e.Acknowledge()
		return
	}

	action, err := discordplugin.ParseAction(pluginID, res, false, discordplugin.ResponseComponent)
	if err != nil {
		b.incInteractionFailure()
		b.incPluginFailure()
		b.logger.ErrorContext(
			ctx,
			"plugin component response parse failed",
			slog.String("custom_id", customID),
			slog.String("err", err.Error()),
		)
		_ = e.CreateMessage(discordplugin.ErrorMessage(b.prodMode, t, err))
		return
	}

	b.executePluginActionFromComponent(e, action)
}

func (b *Bot) executePluginActionFromComponent(
	e *events.ComponentInteractionCreate,
	action discordplugin.Action,
) {
	switch action.Kind {
	case discordplugin.ActionNone:
		_ = e.Acknowledge()
	case discordplugin.ActionModal:
		_ = e.Modal(action.Modal)
	case discordplugin.ActionUpdate:
		_ = e.UpdateMessage(action.Update)
	case discordplugin.ActionMessage:
		_ = e.CreateMessage(action.Create)
	default:
		_ = e.Acknowledge()
	}
}
