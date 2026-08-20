package discordruntime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	commandtext "github.com/xsyetopz/go-mamacord/internal/commandtext"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	discordpluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/router"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

func (b *Bot) onComponent(e *events.ComponentInteractionCreate) {
	b.launchInteraction("component", func() { b.runPluginComponent(e) }, func() {
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindWarning, "", "Mamacord is busy. Try again in a moment.", true))
	})
}
func (b *Bot) runPluginComponent(e *events.ComponentInteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	b.incInteraction()
	locale := e.Locale()
	t := commandtext.Translator{Registry: b.i18n, Locale: locale.Code(), UserID: uint64(e.User().ID)}
	customID := e.Data.CustomID()
	if !b.takeComponentCooldown(e, t, customID, time.Now()) {
		return
	}
	b.handlePluginComponent(ctx, e, t, locale, customID)
}
func (b *Bot) takeComponentCooldown(e *events.ComponentInteractionCreate, t commandtext.Translator, customID string, now time.Time) bool {
	if d := b.componentCooldown(customID); d > 0 {
		if remaining, ok := b.cooldowns.Take(uint64(e.User().ID), componentCooldownKey(customID), d, now); !ok {
			_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindWarning, "", t.S("err.cooldown", map[string]any{"Seconds": cooldownSecs(remaining)}), true))
			return false
		}
	}
	return true
}
func (b *Bot) handlePluginComponent(ctx context.Context, e *events.ComponentInteractionCreate, t commandtext.Translator, locale discord.Locale, customID string) {
	pluginID, localID, ok := pluginhost.ParseCustomID(customID)
	if !ok || !b.moduleEnabled(pluginID) {
		_ = e.Acknowledge()
		return
	}
	if guildID := e.GuildID(); guildID != nil {
		enabled, err := b.guildPluginEnabled(ctx, uint64(*guildID), pluginID)
		if err != nil {
			b.pluginComponentError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
			return
		}
		if !enabled {
			_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindWarning, "", "This plugin is disabled in this server.", true))
			return
		}
	}
	route, ok := b.catalogSnapshot().pluginRoutes[pluginID]
	if !ok {
		_ = e.Acknowledge()
		return
	}
	plan, err := route.Host.PlanComponent(pluginID, localID)
	if err != nil {
		b.pluginComponentError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
		return
	}
	input := router.ComponentInput(e, localID)
	invocation := pluginInvocationContext(e.User(), e.Member(), e.GuildID(), e.Channel(), e.Guild, e.Client().Caches.SelfUser, locale.Code(), b.isOwner(uint64(e.User().ID)))
	invocation.Route = plan.Route
	invocation.Kind = contract.InvocationComponent
	invocation.Component = &input
	invocation.ResponseState = contract.ResponseUnacknowledged
	admission, denial, err := route.Host.Admit(ctx, pluginID, invocation)
	if err != nil {
		b.pluginComponentError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
		return
	}
	if admission == nil {
		if err = respondComponent(e, pluginID, denial, contract.ResponseUnacknowledged); err != nil {
			b.pluginComponentError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
		}
		return
	}
	state := contract.ResponseUnacknowledged
	switch plan.Defer {
	case contract.DeferCreate:
		err = e.DeferCreateMessage(plan.Ephemeral)
		if err == nil {
			state = contract.ResponseDeferredCreate
		}
	case contract.DeferUpdate:
		err = e.DeferUpdateMessage()
		if err == nil {
			state = contract.ResponseDeferredUpdate
		}
	}
	if err != nil {
		b.pluginComponentError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
		return
	}
	terminal, err := admission.Run(ctx, state)
	if err == nil {
		err = respondComponent(e, pluginID, terminal, state)
	}
	if err != nil {
		b.pluginComponentError(ctx, e, t, customID, state, err)
	}
}
func respondComponent(e *events.ComponentInteractionCreate, pluginID string, terminal contract.Operation, state contract.ResponseState) error {
	if terminal == nil {
		if state == contract.ResponseUnacknowledged {
			return e.Acknowledge()
		}
		if state == contract.ResponseDeferredCreate {
			return e.Client().Rest.DeleteInteractionResponse(e.ApplicationID(), e.Token())
		}
		return nil
	}
	switch value := terminal.(type) {
	case *contract.MessageOperation:
		message, err := discordpluginbridge.ContractMessageCreate(pluginID, *value)
		if err != nil {
			return err
		}
		if state == contract.ResponseDeferredCreate {
			_, err = e.Client().Rest.UpdateInteractionResponse(e.ApplicationID(), e.Token(), discord.MessageUpdate{Content: &message.Content, Embeds: &message.Embeds, Components: &message.Components, AllowedMentions: &discord.AllowedMentions{}})
			return err
		}
		return e.CreateMessage(message)
	case *contract.UpdateOperation:
		update, err := discordpluginbridge.ContractMessageUpdate(pluginID, value.Patch)
		if err != nil {
			return err
		}
		if state == contract.ResponseDeferredUpdate {
			_, err = e.Client().Rest.UpdateInteractionResponse(e.ApplicationID(), e.Token(), update)
			return err
		}
		return e.UpdateMessage(update)
	case *contract.EditResponseOperation:
		update, err := discordpluginbridge.ContractMessageUpdate(pluginID, value.Patch)
		if err != nil {
			return err
		}
		_, err = e.Client().Rest.UpdateInteractionResponse(e.ApplicationID(), e.Token(), update)
		return err
	case *contract.ModalOperation:
		if state != contract.ResponseUnacknowledged {
			return errors.New("cannot open modal after deferral")
		}
		modal, err := discordpluginbridge.ContractModal(pluginID, value.Modal)
		if err != nil {
			return err
		}
		return e.Modal(modal)
	default:
		return fmt.Errorf("unsupported component terminal operation %T", terminal)
	}
}
func (b *Bot) pluginComponentError(ctx context.Context, e *events.ComponentInteractionCreate, t commandtext.Translator, customID string, state contract.ResponseState, err error) {
	b.incInteractionFailure()
	b.incPluginFailure()
	b.logger.ErrorContext(ctx, "plugin component failed", slog.String("custom_id", customID), slog.String("err", err.Error()))
	message := interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true)
	if state == contract.ResponseUnacknowledged {
		_ = e.CreateMessage(message)
		return
	}
	content := t.S("err.generic", nil)
	_, _ = e.Client().Rest.UpdateInteractionResponse(e.ApplicationID(), e.Token(), discord.MessageUpdate{Content: &content, Embeds: &[]discord.Embed{}, Components: &[]discord.LayoutComponent{}, AllowedMentions: &discord.AllowedMentions{}})
}
