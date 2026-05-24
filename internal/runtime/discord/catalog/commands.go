package catalog

import (
	"github.com/disgoorg/disgo/discord"

	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

type CommandCreateOptions struct {
	Builtins         []slashcmd.Command
	PluginHost       *pluginhost.Host
	EnabledPluginIDs map[string]struct{}
	I18n             i18n.Registry
	Locales          []string
}

func CommandCreates(opts CommandCreateOptions) []discord.ApplicationCommandCreate {
	const extraCreatesCapacity = 8
	creates := make([]discord.ApplicationCommandCreate, 0, len(opts.Builtins)+extraCreatesCapacity)
	for _, cmd := range opts.Builtins {
		if create, ok := builtinCommandCreate(cmd, opts.Locales, opts.I18n); ok {
			creates = append(creates, create)
		}
	}
	if opts.PluginHost == nil {
		return creates
	}

	return append(
		creates,
		opts.PluginHost.CommandCreatesFiltered(opts.EnabledPluginIDs, opts.Locales, func(pluginID, locale, messageID string) (string, bool) {
			return opts.I18n.TryLocalize(i18n.Config{
				Locale:    locale,
				PluginID:  pluginID,
				MessageID: messageID,
			})
		})...,
	)
}
