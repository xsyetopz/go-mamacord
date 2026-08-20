package discordruntime

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
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

func (b *Bot) onModal(e *events.ModalSubmitInteractionCreate) {
	b.launchInteraction("modal", func() { b.runPluginModal(e) }, func() {
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindWarning, "", "Mamacord is busy. Try again in a moment.", true))
	})
}
func (b *Bot) runPluginModal(e *events.ModalSubmitInteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	b.incInteraction()
	locale := e.Locale()
	t := commandtext.Translator{Registry: b.i18n, Locale: locale.Code(), UserID: uint64(e.User().ID)}
	customID := strings.TrimSpace(e.Data.CustomID)
	if d := b.modalCooldown(customID); d > 0 {
		if remaining, ok := b.cooldowns.Take(uint64(e.User().ID), modalCooldownKey(customID), d, time.Now()); !ok {
			_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindWarning, "", t.S("err.cooldown", map[string]any{"Seconds": cooldownSecs(remaining)}), true))
			return
		}
	}
	pluginID, localID, ok := pluginhost.ParseCustomID(customID)
	if !ok || !b.moduleEnabled(pluginID) {
		_ = e.Acknowledge()
		return
	}
	if guildID := e.GuildID(); guildID != nil {
		enabled, err := b.guildPluginEnabled(ctx, uint64(*guildID), pluginID)
		if err != nil {
			b.pluginModalError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
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
	plan, err := route.Host.PlanModal(pluginID, localID)
	if err != nil {
		b.pluginModalError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
		return
	}
	origin := contract.ModalOriginCommand
	if e.Message != nil {
		origin = contract.ModalOriginComponent
	}
	input := router.ModalInput(e, pluginID, localID)
	invocation := pluginInvocationContext(e.User(), e.Member(), e.GuildID(), e.Channel(), e.Guild, e.Client().Caches.SelfUser, locale.Code(), b.isOwner(uint64(e.User().ID)))
	invocation.Route = plan.Route
	invocation.Kind = contract.InvocationModal
	invocation.Modal = &input
	invocation.ModalOrigin = origin
	invocation.ResponseState = contract.ResponseUnacknowledged
	admission, denial, err := route.Host.Admit(ctx, pluginID, invocation)
	if err != nil {
		b.pluginModalError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
		return
	}
	if admission == nil {
		if err = respondModal(e, pluginID, denial, contract.ResponseUnacknowledged); err != nil {
			b.pluginModalError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
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
		if origin != contract.ModalOriginComponent {
			err = errors.New("command-origin modal cannot defer a message update")
			break
		}
		err = e.DeferUpdateMessage()
		if err == nil {
			state = contract.ResponseDeferredUpdate
		}
	}
	if err != nil {
		b.pluginModalError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
		return
	}
	terminal, err := admission.Run(ctx, state)
	if err == nil {
		err = respondModal(e, pluginID, terminal, state)
	}
	if err != nil {
		b.pluginModalError(ctx, e, t, customID, state, err)
	}
}
func respondModal(e *events.ModalSubmitInteractionCreate, pluginID string, terminal contract.Operation, state contract.ResponseState) error {
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
		return errors.New("modal submit cannot open another modal")
	default:
		return fmt.Errorf("unsupported modal terminal operation %T", terminal)
	}
}
func (b *Bot) pluginModalError(ctx context.Context, e *events.ModalSubmitInteractionCreate, t commandtext.Translator, customID string, state contract.ResponseState, err error) {
	b.incInteractionFailure()
	b.incPluginFailure()
	b.logger.ErrorContext(ctx, "plugin modal failed", slog.String("custom_id", customID), slog.String("err", err.Error()))
	if state == contract.ResponseUnacknowledged {
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
		return
	}
	content := t.S("err.generic", nil)
	_, _ = e.Client().Rest.UpdateInteractionResponse(e.ApplicationID(), e.Token(), discord.MessageUpdate{Content: &content, Embeds: &[]discord.Embed{}, Components: &[]discord.LayoutComponent{}, AllowedMentions: &discord.AllowedMentions{}})
}
