package corecmd

import (
	"context"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	commandapi "github.com/xsyetopz/go-mamacord/internal/commands/api"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
)

func Commands() []commandapi.SlashCommand {
	return []commandapi.SlashCommand{
		ping(),
		help(),
	}
}

func ping() commandapi.SlashCommand {
	return commandapi.SlashCommand{
		Name:   "ping",
		NameID: "cmd.ping.name",
		DescID: "cmd.ping.desc",
		CreateCommand: func(locales []string, t commandapi.Translator) discord.ApplicationCommandCreate {
			return discord.SlashCommandCreate{
				Name: "ping",
				NameLocalizations: commandapi.LocalizeMap(locales, func(locale string) string {
					return t.Registry.MustLocalize(i18n.Config{Locale: locale, MessageID: "cmd.ping.name"})
				}),
				Description: t.S("cmd.ping.desc", nil),
				DescriptionLocalizations: commandapi.LocalizeMap(locales, func(locale string) string {
					return t.Registry.MustLocalize(i18n.Config{Locale: locale, MessageID: "cmd.ping.desc"})
				}),
			}
		},
		Handle: func(ctx context.Context, e *events.ApplicationCommandInteractionCreate, t commandapi.Translator, s commandapi.Services) (interactions.SlashAction, error) {
			_ = ctx
			_ = e
			_ = s
			return interactions.SlashMessage{
				Create: discord.NewMessageCreate().
					WithEphemeral(true).
					WithContent(t.S("ok.pong", nil)),
			}, nil
		},
	}
}

func help() commandapi.SlashCommand {
	return commandapi.SlashCommand{
		Name:   "help",
		NameID: "cmd.help.name",
		DescID: "cmd.help.desc",
		CreateCommand: func(locales []string, t commandapi.Translator) discord.ApplicationCommandCreate {
			return discord.SlashCommandCreate{
				Name: "help",
				NameLocalizations: commandapi.LocalizeMap(locales, func(locale string) string {
					return t.Registry.MustLocalize(i18n.Config{Locale: locale, MessageID: "cmd.help.name"})
				}),
				Description: t.S("cmd.help.desc", nil),
				DescriptionLocalizations: commandapi.LocalizeMap(locales, func(locale string) string {
					return t.Registry.MustLocalize(i18n.Config{Locale: locale, MessageID: "cmd.help.desc"})
				}),
			}
		},
		Handle: func(ctx context.Context, e *events.ApplicationCommandInteractionCreate, t commandapi.Translator, s commandapi.Services) (interactions.SlashAction, error) {
			_ = ctx
			_ = e
			names := []string{}
			if s.HelpNames != nil {
				names = s.HelpNames(t.Locale)
			}

			lines := make([]string, 0, len(names))
			for _, name := range names {
				name = strings.TrimSpace(name)
				if name == "" {
					continue
				}
				if strings.HasPrefix(name, "/") {
					lines = append(lines, "• "+name)
				} else {
					lines = append(lines, "• /"+name)
				}
			}

			content := t.S("cmd.help.content", map[string]any{
				"Commands": strings.Join(lines, "\n"),
			})
			embed := interactions.Embed("Help", content, interactions.ThemeColorBrand)
			return interactions.SlashMessage{
				Create: interactions.MessageEmbeds([]discord.Embed{embed}, true),
			}, nil
		},
	}
}
