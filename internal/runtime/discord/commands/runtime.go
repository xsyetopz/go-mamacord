package commands

import (
	"context"
	"log/slog"
	"sort"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	"github.com/xsyetopz/go-mamacord/internal/buildinfo"
	commandapi "github.com/xsyetopz/go-mamacord/internal/commands/api"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

type Runtime struct {
	Logger        *slog.Logger
	Registry      i18n.Registry
	Store         commandapi.Store
	ProdMode      bool
	SlashCommands map[string]commandapi.SlashCommand
	HelpNames     func(locale discord.Locale) []string
	IsOwner       func(uint64) bool
	Plugins       commandapi.PluginAdmin
	Modules       commandapi.ModuleAdmin
	IncFailure    func()
}

func (r Runtime) Services(locale discord.Locale) commandapi.Services {
	helpNames := r.HelpNames
	if helpNames == nil {
		helpNames = r.fallbackHelpNames
	}
	return commandapi.Services{
		Logger:    r.Logger,
		Store:     r.Store,
		ProdMode:  r.ProdMode,
		IsOwner:   r.IsOwner,
		Plugins:   r.Plugins,
		Modules:   r.Modules,
		HelpNames: helpNames,
	}
}

func (r Runtime) CheckRestrictions(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandapi.Translator,
	build buildinfo.Info,
) (bool, error) {
	restrictions := r.Store.Restrictions()

	msgID := "err.restricted"
	var msgData map[string]any
	dev := build.DeveloperURL
	support := build.SupportServerURL
	if dev != "" && support != "" {
		msgID = "err.restricted_links"
		msgData = map[string]any{
			"DeveloperURL":     dev,
			"SupportServerURL": support,
		}
	}
	msgText := t.S(msgID, msgData)

	userID := uint64(e.User().ID)
	if _, ok, err := restrictions.GetRestriction(ctx, store.TargetTypeUser, userID); err != nil {
		return false, err
	} else if ok {
		return true, e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", msgText, true))
	}

	guildID := e.GuildID()
	if guildID == nil {
		return false, nil
	}

	if _, ok, err := restrictions.GetRestriction(ctx, store.TargetTypeGuild, uint64(*guildID)); err != nil {
		return false, err
	} else if ok {
		return true, e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", msgText, true))
	}

	return false, nil
}

func (r Runtime) HandleSlash(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandapi.Translator,
	locale discord.Locale,
	cmdName string,
) bool {
	cmd, ok := r.SlashCommands[cmdName]
	if !ok {
		return false
	}

	action, err := cmd.Handle(ctx, e, t, r.Services(locale))
	if err != nil {
		r.incFailure()
		if r.Logger != nil {
			r.Logger.ErrorContext(ctx, "command failed", slog.String("cmd", cmdName), slog.String("err", err.Error()))
		}
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
		return true
	}
	if action == nil {
		_ = e.Acknowledge()
		return true
	}
	if execErr := action.Execute(e); execErr != nil {
		r.incFailure()
		if r.Logger != nil {
			r.Logger.ErrorContext(
				ctx,
				"command action failed",
				slog.String("cmd", cmdName),
				slog.String("err", execErr.Error()),
			)
		}
		_ = e.CreateMessage(interactions.NoticeMessage(interactions.KindError, "", t.S("err.generic", nil), true))
	}
	return true
}

func (r Runtime) fallbackHelpNames(locale discord.Locale) []string {
	t := commandapi.Translator{Registry: r.Registry, Locale: locale}
	out := make([]string, 0, len(r.SlashCommands))
	for _, cmd := range r.SlashCommands {
		name := strings.TrimSpace(cmd.Name)
		if strings.TrimSpace(cmd.NameID) != "" {
			name = t.S(cmd.NameID, nil)
		}
		if name != "" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func (r Runtime) incFailure() {
	if r.IncFailure != nil {
		r.IncFailure()
	}
}
