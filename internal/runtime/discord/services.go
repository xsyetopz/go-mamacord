package discordruntime

import (
	"context"
	"sort"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"

	"github.com/xsyetopz/go-mamacord/internal/buildinfo"
	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	commandtext "github.com/xsyetopz/go-mamacord/internal/commandtext"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

func (b *Bot) services(_ discord.Locale) commandruntime.Services {
	s := commandruntime.Services{
		Logger:   b.logger,
		Store:    b.store,
		ProdMode: b.prodMode,
		IsOwner:  b.isOwner,
		HelpNames: func(locale string) []string {
			t := commandtext.Translator{Registry: b.i18n, Locale: locale}
			out := make([]string, 0, len(b.order)+len(b.pluginCommands))
			for _, cmd := range b.order {
				name := strings.TrimSpace(cmd.Name)
				if strings.TrimSpace(cmd.NameID) != "" {
					name = t.S(cmd.NameID, nil)
				}
				if name != "" {
					out = append(out, name)
				}
			}
			for name := range b.pluginCommands {
				out = append(out, name)
			}
			sort.Strings(out)
			return out
		},
	}

	if b.pluginHost != nil {
		s.Plugins = pluginAdmin{b: b}
	}
	s.Marketplace = b.marketplace
	s.Modules = moduleAdmin{b: b}
	return s
}

func (b *Bot) checkRestrictions(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
) (bool, error) {
	restrictions := b.store.Restrictions()

	msgID := "err.restricted"
	var msgData map[string]any
	currentBuild := buildinfo.Current()
	if currentBuild.DeveloperURL != "" && currentBuild.SupportServerURL != "" {
		msgID = "err.restricted_links"
		msgData = map[string]any{
			"DeveloperURL":     currentBuild.DeveloperURL,
			"SupportServerURL": currentBuild.SupportServerURL,
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
