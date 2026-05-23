package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"

	commandapi "github.com/xsyetopz/go-mamacord/internal/commands/api"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/catalog"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

type Registrar struct {
	Client           *bot.Client
	Builtins         []commandapi.SlashCommand
	PluginHost       *pluginhost.Host
	EnabledPluginIDs map[string]struct{}
	I18n             i18n.Registry
}

func (r Registrar) Creates() []discord.ApplicationCommandCreate {
	return catalog.CommandCreates(catalog.CommandCreateOptions{
		Builtins:         r.Builtins,
		PluginHost:       r.PluginHost,
		EnabledPluginIDs: r.EnabledPluginIDs,
		I18n:             r.I18n,
		Locales:          r.I18n.SupportedLocales(),
	})
}

func (r Registrar) Register(ctx context.Context, mode string, devGuildID *uint64, guildIDs []uint64) error {
	creates := r.Creates()
	if devGuildID != nil {
		_, err := r.client().Rest.SetGuildCommands(r.client().ApplicationID, snowflake.ID(*devGuildID), creates)
		return err
	}

	switch mode {
	case "global":
		_, err := r.client().Rest.SetGlobalCommands(r.client().ApplicationID, creates)
		return err
	case "guilds":
		return r.SetCommandsInGuilds(ctx, creates, guildIDs)
	case "hybrid":
		if _, err := r.client().Rest.SetGlobalCommands(r.client().ApplicationID, creates); err != nil {
			return err
		}
		return r.SetCommandsInGuilds(ctx, creates, guildIDs)
	default:
		return errors.New("invalid command registration mode")
	}
}

func (r Registrar) SetCommandsInGuilds(
	_ context.Context,
	creates []discord.ApplicationCommandCreate,
	guildIDs []uint64,
) error {
	if r.client() == nil {
		return errors.New("discord client not initialized")
	}
	if len(guildIDs) == 0 {
		return nil
	}

	for _, guildID := range guildIDs {
		if guildID == 0 {
			continue
		}
		_, err := r.client().Rest.SetGuildCommands(r.client().ApplicationID, snowflake.ID(guildID), creates)
		if err != nil {
			return fmt.Errorf("set commands for guild %d: %w", guildID, err)
		}
	}
	return nil
}

func (r Registrar) RegisterInCachedGuilds(_ context.Context) error {
	if r.client() == nil {
		return errors.New("discord client not initialized")
	}

	creates := r.Creates()
	for guild := range r.client().Caches.Guilds() {
		guildID := uint64(guild.ID)
		if guildID == 0 {
			continue
		}
		_, err := r.client().Rest.SetGuildCommands(r.client().ApplicationID, snowflake.ID(guildID), creates)
		if err != nil {
			return fmt.Errorf("set commands for cached guild %d: %w", guildID, err)
		}
	}

	return nil
}

func (r Registrar) client() *bot.Client {
	return r.Client
}
