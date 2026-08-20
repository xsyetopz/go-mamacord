package pluginbridge

import (
	"context"
	"errors"
	"fmt"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/customid"
	"log/slog"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	commandtext "github.com/xsyetopz/go-mamacord/internal/commandtext"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/router"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/router/cooldown"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

type InteractionAccess struct {
	Logger             *slog.Logger
	I18n               i18n.Registry
	Route              func(string) (Route, bool)
	ModuleEnabled      func(string) bool
	GuildPluginEnabled func(context.Context, uint64, string) (bool, error)
	IsOwner            func(uint64) bool
}

type InteractionLimits struct {
	Cooldowns    *cooldown.Tracker
	Policy       cooldown.Policy
	Interactions *interactions.Dispatcher
}

type InteractionMetrics struct {
	IncInteraction        func()
	IncInteractionFailure func()
	IncPluginFailure      func()
}

type InteractionOptions struct {
	InteractionAccess
	InteractionLimits
	InteractionMetrics
}

type InteractionDispatcher struct {
	logger             *slog.Logger
	i18n               i18n.Registry
	route              func(string) (Route, bool)
	moduleEnabled      func(string) bool
	guildPluginEnabled func(context.Context, uint64, string) (bool, error)
	isOwner            func(uint64) bool
	cooldowns          *cooldown.Tracker
	policy             cooldown.Policy
	interactions       *interactions.Dispatcher
	metrics            InteractionMetrics
}

func NewInteractionDispatcher(options InteractionOptions) *InteractionDispatcher {
	return &InteractionDispatcher{
		logger: options.Logger, i18n: options.I18n, route: options.Route,
		moduleEnabled: options.ModuleEnabled, guildPluginEnabled: options.GuildPluginEnabled, isOwner: options.IsOwner,
		cooldowns: options.Cooldowns, policy: options.Policy, interactions: options.Interactions, metrics: options.InteractionMetrics,
	}
}

func (dispatcher *InteractionDispatcher) incInteraction() {
	if dispatcher != nil && dispatcher.metrics.IncInteraction != nil {
		dispatcher.metrics.IncInteraction()
	}
}
func (dispatcher *InteractionDispatcher) incInteractionFailure() {
	if dispatcher != nil && dispatcher.metrics.IncInteractionFailure != nil {
		dispatcher.metrics.IncInteractionFailure()
	}
}
func (dispatcher *InteractionDispatcher) incPluginFailure() {
	if dispatcher != nil && dispatcher.metrics.IncPluginFailure != nil {
		dispatcher.metrics.IncPluginFailure()
	}
}

func (dispatcher *InteractionDispatcher) OnComponent(e *events.ComponentInteractionCreate) {
	dispatcher.interactions.Launch("component", func() { dispatcher.runPluginComponent(e) }, func() {
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindWarning, "", "Mamacord is busy. Try again in a moment.", true))
	})
}
func (dispatcher *InteractionDispatcher) runPluginComponent(e *events.ComponentInteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	dispatcher.incInteraction()
	locale := e.Locale()
	t := commandtext.Translator{Registry: dispatcher.i18n, Locale: locale.Code(), UserID: uint64(e.User().ID)}
	customID := e.Data.CustomID()
	if !dispatcher.takeComponentCooldown(e, t, customID, time.Now()) {
		return
	}
	dispatcher.handlePluginComponent(ctx, e, t, locale, customID)
}
func (dispatcher *InteractionDispatcher) takeComponentCooldown(e *events.ComponentInteractionCreate, t commandtext.Translator, customID string, now time.Time) bool {
	if d := dispatcher.policy.ComponentCooldown(customID); d > 0 {
		if remaining, ok := dispatcher.cooldowns.Take(uint64(e.User().ID), cooldown.ComponentCooldownKey(customID), d, now); !ok {
			_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindWarning, "", t.S("err.cooldown", map[string]any{"Seconds": cooldown.CooldownSeconds(remaining)}), true))
			return false
		}
	}
	return true
}
func (dispatcher *InteractionDispatcher) handlePluginComponent(ctx context.Context, e *events.ComponentInteractionCreate, t commandtext.Translator, locale discord.Locale, customID string) {
	pluginID, localID, ok := customid.Parse(customID)
	if !ok || !dispatcher.moduleEnabled(pluginID) {
		_ = e.Acknowledge()
		return
	}
	if guildID := e.GuildID(); guildID != nil {
		enabled, err := dispatcher.guildPluginEnabled(ctx, uint64(*guildID), pluginID)
		if err != nil {
			dispatcher.pluginComponentError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
			return
		}
		if !enabled {
			_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindWarning, "", "This plugin is disabled in this server.", true))
			return
		}
	}
	route, ok := dispatcher.route(pluginID)
	if !ok {
		_ = e.Acknowledge()
		return
	}
	plan, err := route.Host.PlanComponent(pluginID, localID)
	if err != nil {
		dispatcher.pluginComponentError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
		return
	}
	input := router.ComponentInput(e, localID)
	invocation := router.PluginInvocationContext(e.User(), e.Member(), e.GuildID(), e.Channel(), e.Guild, e.Client().Caches.SelfUser, locale.Code(), dispatcher.isOwner(uint64(e.User().ID)))
	invocation.Route = plan.Route
	invocation.Kind = contract.InvocationComponent
	invocation.Component = &input
	invocation.ResponseState = contract.ResponseUnacknowledged
	admission, denial, err := route.Host.Admit(ctx, pluginID, invocation)
	if err != nil {
		dispatcher.pluginComponentError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
		return
	}
	if admission == nil {
		if err = respondComponent(e, pluginID, denial, contract.ResponseUnacknowledged); err != nil {
			dispatcher.pluginComponentError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
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
		dispatcher.pluginComponentError(ctx, e, t, customID, contract.ResponseUnacknowledged, err)
		return
	}
	terminal, err := admission.Run(ctx, state)
	if err == nil {
		err = respondComponent(e, pluginID, terminal, state)
	}
	if err != nil {
		dispatcher.pluginComponentError(ctx, e, t, customID, state, err)
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
		message, err := ContractMessageCreate(pluginID, *value)
		if err != nil {
			return err
		}
		if state == contract.ResponseDeferredCreate {
			_, err = e.Client().Rest.UpdateInteractionResponse(e.ApplicationID(), e.Token(), discord.MessageUpdate{Content: &message.Content, Embeds: &message.Embeds, Components: &message.Components, AllowedMentions: &discord.AllowedMentions{}})
			return err
		}
		return e.CreateMessage(message)
	case *contract.UpdateOperation:
		update, err := ContractMessageUpdate(pluginID, value.Patch)
		if err != nil {
			return err
		}
		if state == contract.ResponseDeferredUpdate {
			_, err = e.Client().Rest.UpdateInteractionResponse(e.ApplicationID(), e.Token(), update)
			return err
		}
		return e.UpdateMessage(update)
	case *contract.EditResponseOperation:
		update, err := ContractMessageUpdate(pluginID, value.Patch)
		if err != nil {
			return err
		}
		_, err = e.Client().Rest.UpdateInteractionResponse(e.ApplicationID(), e.Token(), update)
		return err
	case *contract.ModalOperation:
		if state != contract.ResponseUnacknowledged {
			return errors.New("cannot open modal after deferral")
		}
		modal, err := ContractModal(pluginID, value.Modal)
		if err != nil {
			return err
		}
		return e.Modal(modal)
	default:
		return fmt.Errorf("unsupported component terminal operation %T", terminal)
	}
}
func (dispatcher *InteractionDispatcher) pluginComponentError(ctx context.Context, e *events.ComponentInteractionCreate, t commandtext.Translator, customID string, state contract.ResponseState, err error) {
	dispatcher.incInteractionFailure()
	dispatcher.incPluginFailure()
	dispatcher.logger.ErrorContext(ctx, "plugin component failed", slog.String("custom_id", customID), slog.String("err", err.Error()))
	message := interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true)
	if state == contract.ResponseUnacknowledged {
		_ = e.CreateMessage(message)
		return
	}
	content := t.S("err.generic", nil)
	_, _ = e.Client().Rest.UpdateInteractionResponse(e.ApplicationID(), e.Token(), discord.MessageUpdate{Content: &content, Embeds: &[]discord.Embed{}, Components: &[]discord.LayoutComponent{}, AllowedMentions: &discord.AllowedMentions{}})
}
