package appcmd

import (
	"context"
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
	discordplugin "github.com/xsyetopz/go-mamacord/internal/runtime/discord/plugin"
	discordpluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/router"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
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
	ctx := context.Background()
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
	ctx := context.Background()
	d.incInteraction()

	data := e.Data
	cmdName := data.CommandName
	route, ok := d.PluginCommands[cmdName]
	if !ok {
		_ = e.AutocompleteResult(nil)
		return
	}
	if disabled, err := d.commandDisabled(ctx, e.GuildID(), route.PluginID, cmdName); err != nil {
		d.incInteractionFailure()
		d.logger().ErrorContext(ctx, "plugin autocomplete permission check failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
		_ = e.AutocompleteResult(nil)
		return
	} else if disabled {
		_ = e.AutocompleteResult(nil)
		return
	}

	isOwner := false
	if services := d.Services(e.Locale()); services.IsOwner != nil {
		isOwner = services.IsOwner(uint64(e.User().ID))
	}

	res, pluginID, err := route.Host.HandleAutocomplete(ctx, cmdName, router.OptionalString(data.SubCommandGroupName), router.OptionalString(data.SubCommandName), strings.TrimSpace(data.Focused().Name), pluginhost.Payload{
		GuildID:   router.SnowflakePtrToString(e.GuildID()),
		ChannelID: e.Channel().ID().String(),
		UserID:    e.User().ID.String(),
		Locale:    e.Locale().Code(),
		IsOwner:   isOwner,
		Options:   router.PluginAutocompleteOptions(data),
	})
	if err != nil {
		d.incInteractionFailure()
		d.incPluginFailure()
		d.logger().ErrorContext(ctx, "plugin autocomplete failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
		_ = e.AutocompleteResult(nil)
		return
	}

	choices, parseErr := router.ParsePluginAutocompleteChoices(pluginID, res)
	if parseErr != nil {
		d.incInteractionFailure()
		d.incPluginFailure()
		d.logger().ErrorContext(ctx, "plugin autocomplete parse failed", slog.String("cmd", cmdName), slog.String("err", parseErr.Error()))
		_ = e.AutocompleteResult(nil)
		return
	}
	_ = e.AutocompleteResult(choices)
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

func (d Dispatcher) handlePluginSlash(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	locale discord.Locale,
	cmdName string,
	isOwner bool,
	data discord.SlashCommandInteractionData,
) {
	route, ok := d.PluginCommands[cmdName]
	if !ok {
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
		return
	}
	interaction := discordpluginbridge.NewSlashInteraction(e)
	if disabled, err := d.commandDisabled(ctx, e.GuildID(), route.PluginID, cmdName); err != nil {
		d.incInteractionFailure()
		d.incPluginFailure()
		d.logger().ErrorContext(ctx, "plugin command permission check failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
		d.respondPluginSlashError(e, t, interaction)
		return
	} else if disabled {
		d.respondCommandDisabled(e)
		return
	}

	res, defaultEphemeral, pluginID, err := route.Host.HandleSlash(ctx, cmdName, pluginhost.Payload{
		GuildID:     router.SnowflakePtrToString(e.GuildID()),
		ChannelID:   e.Channel().ID().String(),
		UserID:      e.User().ID.String(),
		Locale:      locale.Code(),
		IsOwner:     isOwner,
		Options:     router.PluginOptions(data),
		Interaction: interaction,
	})
	if err != nil {
		d.incInteractionFailure()
		d.incPluginFailure()
		d.logger().ErrorContext(ctx, "plugin command failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
		d.respondPluginSlashError(e, t, interaction)
		return
	}

	action, parseErr := discordplugin.ParseAction(pluginID, res, defaultEphemeral, discordplugin.ResponseSlash)
	if parseErr != nil {
		d.incInteractionFailure()
		d.incPluginFailure()
		d.logger().ErrorContext(ctx, "plugin response parse failed", slog.String("cmd", cmdName), slog.String("err", parseErr.Error()))
		d.respondPluginSlashError(e, t, interaction)
		return
	}

	d.executePluginActionFromSlash(e, t, action)
}

func (d Dispatcher) handlePluginUserCommand(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	locale discord.Locale,
	cmdName string,
	isOwner bool,
	data discord.UserCommandInteractionData,
) {
	route, ok := d.PluginUserCommands[cmdName]
	if !ok {
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
		return
	}
	interaction := discordpluginbridge.NewSlashInteraction(e)
	if disabled, err := d.commandDisabled(ctx, e.GuildID(), route.PluginID, cmdName); err != nil {
		d.incInteractionFailure()
		d.incPluginFailure()
		d.logger().ErrorContext(ctx, "plugin user command permission check failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
		d.respondPluginSlashError(e, t, interaction)
		return
	} else if disabled {
		d.respondCommandDisabled(e)
		return
	}

	res, defaultEphemeral, pluginID, err := route.Host.HandleUserCommand(ctx, cmdName, pluginhost.Payload{
		GuildID:     router.SnowflakePtrToString(e.GuildID()),
		ChannelID:   e.Channel().ID().String(),
		UserID:      e.User().ID.String(),
		Locale:      locale.Code(),
		IsOwner:     isOwner,
		Options:     router.PluginUserContextOptions(data),
		Interaction: interaction,
	})
	if err != nil {
		d.incInteractionFailure()
		d.incPluginFailure()
		d.logger().ErrorContext(ctx, "plugin user command failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
		d.respondPluginSlashError(e, t, interaction)
		return
	}

	action, parseErr := discordplugin.ParseAction(pluginID, res, defaultEphemeral, discordplugin.ResponseSlash)
	if parseErr != nil {
		d.incInteractionFailure()
		d.incPluginFailure()
		d.logger().ErrorContext(ctx, "plugin user command response parse failed", slog.String("cmd", cmdName), slog.String("err", parseErr.Error()))
		d.respondPluginSlashError(e, t, interaction)
		return
	}

	d.executePluginActionFromSlash(e, t, action)
}

func (d Dispatcher) handlePluginMessageCommand(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	locale discord.Locale,
	cmdName string,
	isOwner bool,
	data discord.MessageCommandInteractionData,
) {
	route, ok := d.PluginMessageCommands[cmdName]
	if !ok {
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
		return
	}
	interaction := discordpluginbridge.NewSlashInteraction(e)
	if disabled, err := d.commandDisabled(ctx, e.GuildID(), route.PluginID, cmdName); err != nil {
		d.incInteractionFailure()
		d.incPluginFailure()
		d.logger().ErrorContext(ctx, "plugin message command permission check failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
		d.respondPluginSlashError(e, t, interaction)
		return
	} else if disabled {
		d.respondCommandDisabled(e)
		return
	}

	res, defaultEphemeral, pluginID, err := route.Host.HandleMessageCommand(ctx, cmdName, pluginhost.Payload{
		GuildID:     router.SnowflakePtrToString(e.GuildID()),
		ChannelID:   e.Channel().ID().String(),
		UserID:      e.User().ID.String(),
		Locale:      locale.Code(),
		IsOwner:     isOwner,
		Options:     router.PluginMessageContextOptions(data),
		Interaction: interaction,
	})
	if err != nil {
		d.incInteractionFailure()
		d.incPluginFailure()
		d.logger().ErrorContext(ctx, "plugin message command failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
		d.respondPluginSlashError(e, t, interaction)
		return
	}

	action, parseErr := discordplugin.ParseAction(pluginID, res, defaultEphemeral, discordplugin.ResponseSlash)
	if parseErr != nil {
		d.incInteractionFailure()
		d.incPluginFailure()
		d.logger().ErrorContext(ctx, "plugin message command response parse failed", slog.String("cmd", cmdName), slog.String("err", parseErr.Error()))
		d.respondPluginSlashError(e, t, interaction)
		return
	}

	d.executePluginActionFromSlash(e, t, action)
}

func (d Dispatcher) respondPluginSlashError(
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	interaction *discordpluginbridge.SlashInteraction,
) {
	if interaction != nil && interaction.Deferred() {
		content := t.S("err.generic", nil)
		_ = interactions.SlashUpdateInteractionResponse{Update: discord.MessageUpdate{
			Content:         &content,
			AllowedMentions: &discord.AllowedMentions{},
			Embeds:          &[]discord.Embed{},
		}}.Execute(e)
		return
	}

	_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
}

func (d Dispatcher) executePluginActionFromSlash(
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	action discordplugin.Action,
) {
	switch action.Kind {
	case discordplugin.ActionNone:
		_ = e.Acknowledge()
	case discordplugin.ActionModal:
		_ = e.Modal(action.Modal)
	case discordplugin.ActionUpdate:
		_ = interactions.SlashUpdateInteractionResponse{Update: action.Update}.Execute(e)
	case discordplugin.ActionMessage:
		_ = e.CreateMessage(action.Create)
	default:
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
	}
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
