package appcmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"

	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	commandtext "github.com/xsyetopz/go-mamacord/internal/commandtext"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	discordpluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/router"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

type RestrictionCheck func(ctx context.Context, e *events.ApplicationCommandInteractionCreate, t commandtext.Translator) (bool, error)
type SlashCooldownCheck func(e *events.ApplicationCommandInteractionCreate, cmdName string, now time.Time) (remainingSeconds int, ok bool)
type ServicesFactory func(locale discord.Locale) commandruntime.Services
type PluginEnabled func(pluginID string) bool
type GuildCommandEnabled func(ctx context.Context, guildID uint64, pluginID, commandName string) (bool, error)

type Dispatcher struct {
	Logger                *slog.Logger
	I18n                  i18n.Registry
	ProdMode              bool
	Commands              map[string]slashcmd.Command
	PluginCommands        map[string]discordpluginbridge.Route
	PluginUserCommands    map[string]discordpluginbridge.Route
	PluginMessageCommands map[string]discordpluginbridge.Route
	Services              ServicesFactory
	CheckRestrictions     RestrictionCheck
	TakeSlashCooldown     SlashCooldownCheck
	GuildCommandEnabled   GuildCommandEnabled
	IncInteraction        func()
	IncInteractionFailure func()
	IncPluginFailure      func()
}

func (d Dispatcher) OnCommand(e *events.ApplicationCommandInteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	d.incInteraction()

	locale := e.Locale()
	t := commandtext.Translator{Registry: d.I18n, Locale: locale.Code(), UserID: uint64(e.User().ID)}
	isOwner := false
	if services := d.Services(locale); services.IsOwner != nil {
		isOwner = services.IsOwner(uint64(e.User().ID))
	}
	data := e.Data
	cmdName := data.CommandName()

	if !d.preflightSlash(ctx, e, t) {
		return
	}

	guildID := e.GuildID()
	guildName := ""
	if guildID != nil {
		if guild, ok := e.Client().Caches.Guild(*guildID); ok {
			guildName = strings.TrimSpace(guild.Name)
		}
	}
	d.logger().Info(
		"command used",
		slog.String("cmd", cmdName),
		slog.Uint64("user_id", uint64(e.User().ID)),
		slog.String("username", strings.TrimSpace(e.User().Username)),
		slog.String("guild_name", guildName),
		slog.String("guild_id", router.SnowflakePtrToString(guildID)),
	)

	if !d.takeSlashCooldown(e, t, cmdName, time.Now()) {
		return
	}

	if d.handleRegisteredSlash(ctx, e, t, locale, cmdName) {
		return
	}

	switch data.Type() {
	case discord.ApplicationCommandTypeUser:
		d.handlePluginUserCommand(ctx, e, t, locale, cmdName, isOwner, e.UserCommandInteractionData())
	case discord.ApplicationCommandTypeMessage:
		d.handlePluginMessageCommand(ctx, e, t, locale, cmdName, isOwner, e.MessageCommandInteractionData())
	default:
		d.handlePluginSlash(ctx, e, t, locale, cmdName, isOwner, e.SlashCommandInteractionData())
	}
}

func (d Dispatcher) OnAutocomplete(e *events.AutocompleteInteractionCreate) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	d.incInteraction()
	data := e.Data
	cmdName := data.CommandName
	route, ok := d.PluginCommands[cmdName]
	if !ok {
		_ = e.AutocompleteResult(nil)
		return
	}
	if disabled, err := d.commandDisabled(ctx, e.GuildID(), route.PluginID, cmdName); err != nil || disabled {
		if err != nil {
			d.incInteractionFailure()
			d.logger().ErrorContext(ctx, "plugin autocomplete permission check failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
		}
		_ = e.AutocompleteResult(nil)
		return
	}
	isOwner := false
	if services := d.Services(e.Locale()); services.IsOwner != nil {
		isOwner = services.IsOwner(uint64(e.User().ID))
	}
	input := router.AutocompleteInput(data)
	plan, err := route.Host.PlanAutocomplete("slash", cmdName, input.Path, input.Option)
	if err != nil {
		d.pluginAutocompleteError(ctx, e, cmdName, err)
		return
	}
	invocation := pluginInteractionInvocation(e.User(), e.Member(), e.GuildID(), e.Channel(), e.Guild, e.Client().Caches.SelfUser, e.Locale().Code(), isOwner)
	invocation.Route = plan.Route
	invocation.Kind = contract.InvocationAutocomplete
	invocation.Autocomplete = &input
	admission, denial, err := route.Host.Admit(ctx, route.PluginID, invocation)
	if err != nil {
		d.pluginAutocompleteError(ctx, e, cmdName, err)
		return
	}
	terminal := denial
	if admission != nil {
		terminal, err = admission.Run(ctx, contract.ResponseUnacknowledged)
		if err != nil {
			d.pluginAutocompleteError(ctx, e, cmdName, err)
			return
		}
	}
	choices, ok := terminal.(*contract.AutocompleteChoicesOperation)
	if !ok {
		d.pluginAutocompleteError(ctx, e, cmdName, errors.New("autocomplete did not return choices"))
		return
	}
	converted, err := discordpluginbridge.ContractAutocompleteChoices(choices.Choices)
	if err != nil {
		d.pluginAutocompleteError(ctx, e, cmdName, err)
		return
	}
	_ = e.AutocompleteResult(converted)
}
func (d Dispatcher) pluginAutocompleteError(ctx context.Context, e *events.AutocompleteInteractionCreate, cmdName string, err error) {
	d.incInteractionFailure()
	d.incPluginFailure()
	d.logger().ErrorContext(ctx, "plugin autocomplete failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
	_ = e.AutocompleteResult(nil)
}

func (d Dispatcher) preflightSlash(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
) bool {
	if d.CheckRestrictions == nil {
		return true
	}
	restricted, err := d.CheckRestrictions(ctx, e, t)
	if err != nil {
		d.logger().ErrorContext(ctx, "restriction check failed", slog.String("err", err.Error()))
		_ = e.CreateMessage(discord.NewMessageCreate().WithEphemeral(true).WithContent(t.S("err.generic", nil)))
		return false
	}
	return !restricted
}

func (d Dispatcher) takeSlashCooldown(
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	cmdName string,
	now time.Time,
) bool {
	if d.TakeSlashCooldown == nil {
		return true
	}
	remaining, ok := d.TakeSlashCooldown(e, cmdName, now)
	if ok {
		return true
	}
	msg := interactions.NoticeMessage(
		interactions.KindWarning,
		"",
		t.S("err.cooldown", map[string]any{"Seconds": remaining}),
		true,
	)
	_ = e.CreateMessage(msg)
	return false
}

func (d Dispatcher) handleRegisteredSlash(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	locale discord.Locale,
	cmdName string,
) bool {
	cmd, ok := d.Commands[cmdName]
	if !ok {
		return false
	}

	action, err := cmd.Handle(ctx, e, t, d.services(locale))
	if err != nil {
		d.incInteractionFailure()
		d.logger().ErrorContext(ctx, "command failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
		return true
	}
	if action == nil {
		_ = e.Acknowledge()
		return true
	}
	if execErr := action.Execute(e); execErr != nil {
		d.incInteractionFailure()
		d.logger().ErrorContext(
			ctx,
			"command action failed",
			slog.String("cmd", cmdName),
			slog.String("err", execErr.Error()),
		)
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
	}
	return true
}

func (d Dispatcher) handlePluginSlash(ctx context.Context, e *events.ApplicationCommandInteractionCreate, t commandtext.Translator, locale discord.Locale, cmdName string, isOwner bool, data discord.SlashCommandInteractionData) {
	route, ok := d.PluginCommands[cmdName]
	if !ok {
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
		return
	}
	input := router.CommandInput(data)
	d.runPluginCommand(ctx, e, t, locale, cmdName, isOwner, route, input)
}
func (d Dispatcher) handlePluginUserCommand(ctx context.Context, e *events.ApplicationCommandInteractionCreate, t commandtext.Translator, locale discord.Locale, cmdName string, isOwner bool, data discord.UserCommandInteractionData) {
	route, ok := d.PluginUserCommands[cmdName]
	if !ok {
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
		return
	}
	input := router.UserCommandInput(data)
	d.runPluginCommand(ctx, e, t, locale, cmdName, isOwner, route, input)
}
func (d Dispatcher) handlePluginMessageCommand(ctx context.Context, e *events.ApplicationCommandInteractionCreate, t commandtext.Translator, locale discord.Locale, cmdName string, isOwner bool, data discord.MessageCommandInteractionData) {
	route, ok := d.PluginMessageCommands[cmdName]
	if !ok {
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
		return
	}
	input := router.MessageCommandInput(data)
	d.runPluginCommand(ctx, e, t, locale, cmdName, isOwner, route, input)
}
func (d Dispatcher) runPluginCommand(ctx context.Context, e *events.ApplicationCommandInteractionCreate, t commandtext.Translator, locale discord.Locale, cmdName string, isOwner bool, route discordpluginbridge.Route, input contract.CommandInput) {
	if disabled, err := d.commandDisabled(ctx, e.GuildID(), route.PluginID, cmdName); err != nil {
		d.pluginCommandError(ctx, e, t, cmdName, false, err)
		return
	} else if disabled {
		d.respondCommandDisabled(e)
		return
	}
	plan, err := route.Host.PlanCommand(string(input.Kind), cmdName, input.Path)
	if err != nil {
		d.pluginCommandError(ctx, e, t, cmdName, false, err)
		return
	}
	invocation := pluginInteractionInvocation(e.User(), e.Member(), e.GuildID(), e.Channel(), e.Guild, e.Client().Caches.SelfUser, locale.Code(), isOwner)
	invocation.Route = plan.Route
	invocation.Kind = contract.InvocationCommand
	invocation.Command = &input
	invocation.ResponseState = contract.ResponseUnacknowledged
	admission, denial, err := route.Host.Admit(ctx, route.PluginID, invocation)
	if err != nil {
		d.pluginCommandError(ctx, e, t, cmdName, false, err)
		return
	}
	if admission == nil {
		if err = d.respondPluginCommand(e, route.PluginID, denial, contract.ResponseUnacknowledged); err != nil {
			d.pluginCommandError(ctx, e, t, cmdName, false, err)
		}
		return
	}
	state := contract.ResponseUnacknowledged
	deferred := false
	switch plan.Defer {
	case contract.DeferCreate:
		if err = e.DeferCreateMessage(plan.Ephemeral); err == nil {
			state = contract.ResponseDeferredCreate
			deferred = true
		}
	case contract.DeferUpdate:
		err = errors.New("commands cannot defer a message update")
	}
	if err != nil {
		d.pluginCommandError(ctx, e, t, cmdName, false, err)
		return
	}
	terminal, err := admission.Run(ctx, state)
	if err != nil {
		d.pluginCommandError(ctx, e, t, cmdName, deferred, err)
		return
	}
	if err = d.respondPluginCommand(e, route.PluginID, terminal, state); err != nil {
		d.pluginCommandError(ctx, e, t, cmdName, deferred, err)
	}
}
func (d Dispatcher) respondPluginCommand(e *events.ApplicationCommandInteractionCreate, pluginID string, terminal contract.Operation, state contract.ResponseState) error {
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
			update := discord.MessageUpdate{Content: &message.Content, Embeds: &message.Embeds, Components: &message.Components, AllowedMentions: &discord.AllowedMentions{}}
			return interactions.SlashUpdateInteractionResponse{Update: update}.Execute(e)
		}
		return e.CreateMessage(message)
	case *contract.EditResponseOperation:
		update, err := discordpluginbridge.ContractMessageUpdate(pluginID, value.Patch)
		if err != nil {
			return err
		}
		return interactions.SlashUpdateInteractionResponse{Update: update}.Execute(e)
	case *contract.UpdateOperation:
		return errors.New("command cannot update a source message")
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
		return fmt.Errorf("unsupported command terminal operation %T", terminal)
	}
}
func (d Dispatcher) pluginCommandError(ctx context.Context, e *events.ApplicationCommandInteractionCreate, t commandtext.Translator, cmdName string, deferred bool, err error) {
	d.incInteractionFailure()
	d.incPluginFailure()
	d.logger().ErrorContext(ctx, "plugin command failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
	if deferred {
		content := t.S("err.generic", nil)
		_ = interactions.SlashUpdateInteractionResponse{Update: discord.MessageUpdate{Content: &content, AllowedMentions: &discord.AllowedMentions{}, Embeds: &[]discord.Embed{}}}.Execute(e)
		return
	}
	_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
}
func pluginInteractionInvocation(user discord.User, member *discord.ResolvedMember, guildID *snowflake.ID, channel discord.InteractionChannel, guild func() (discord.Guild, bool), self func() (discord.OAuth2User, bool), locale string, isOwner bool) contract.Invocation {
	author := router.UserRef(user)
	channelRef := router.InteractionChannelRef(channel, router.SnowflakePtrToString(guildID))
	invocation := contract.Invocation{Author: &author, Channel: &channelRef, Locale: locale, IsOwner: isOwner, ResponseState: contract.ResponseUnacknowledged}
	if member != nil {
		value := router.MemberRef(*member, router.SnowflakePtrToString(guildID))
		invocation.Member = &value
	}
	if value, ok := guild(); ok {
		ref := router.GuildRef(value)
		invocation.Guild = &ref
	} else if guildID != nil {
		invocation.Guild = &contract.GuildRef{ID: guildID.String()}
	}
	if value, ok := self(); ok {
		ref := router.UserRef(value.User)
		invocation.BotUser = &ref
	}
	return invocation
}

func (d Dispatcher) commandDisabled(
	ctx context.Context,
	guildID *snowflake.ID,
	pluginID, commandName string,
) (bool, error) {
	if guildID == nil || d.GuildCommandEnabled == nil {
		return false, nil
	}
	enabled, err := d.GuildCommandEnabled(ctx, uint64(*guildID), pluginID, commandName)
	if err != nil {
		return false, err
	}
	return !enabled, nil
}

func (d Dispatcher) respondCommandDisabled(e *events.ApplicationCommandInteractionCreate) {
	_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindWarning, "", "This command is disabled in this server.", true))
}

func (d Dispatcher) services(locale discord.Locale) commandruntime.Services {
	if d.Services == nil {
		return commandruntime.Services{}
	}
	return d.Services(locale)
}

func (d Dispatcher) logger() *slog.Logger {
	if d.Logger == nil {
		return slog.Default()
	}
	return d.Logger
}

func (d Dispatcher) incInteraction() {
	if d.IncInteraction != nil {
		d.IncInteraction()
	}
}

func (d Dispatcher) incInteractionFailure() {
	if d.IncInteractionFailure != nil {
		d.IncInteractionFailure()
	}
}

func (d Dispatcher) incPluginFailure() {
	if d.IncPluginFailure != nil {
		d.IncPluginFailure()
	}
}
