package catalog

import (
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/omit"

	commandtext "github.com/xsyetopz/go-mamacord/internal/commandtext"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
)

func builtinCommandCreate(cmd slashcmd.Command, locales []string, registry i18n.Registry) (discord.ApplicationCommandCreate, bool) {
	t := commandtext.Translator{Registry: registry, Locale: discord.LocaleEnglishUS.Code()}

	switch strings.TrimSpace(cmd.Name) {
	case "ping":
		return simpleBuiltinCreate("ping", "cmd.ping.name", "cmd.ping.desc", locales, t), true
	case "help":
		return simpleBuiltinCreate("help", "cmd.help.name", "cmd.help.desc", locales, t), true
	case "block":
		return blockCreate(locales, t), true
	case "unblock":
		return unblockCreate(locales, t), true
	case "modules":
		return modulesCreate(), true
	case "plugins":
		return pluginsCreate(locales, t), true
	default:
		return nil, false
	}
}

func simpleBuiltinCreate(name, nameID, descID string, locales []string, t commandtext.Translator) discord.ApplicationCommandCreate {
	return discord.SlashCommandCreate{
		Name: name,
		NameLocalizations: localizeMap(locales, func(locale string) string {
			return t.Registry.MustLocalize(i18n.Config{Locale: locale, MessageID: nameID})
		}),
		Description: t.S(descID, nil),
		DescriptionLocalizations: localizeMap(locales, func(locale string) string {
			return t.Registry.MustLocalize(i18n.Config{Locale: locale, MessageID: descID})
		}),
	}
}

func createLocalizer(locales []string, t commandtext.Translator) func(id string) map[discord.Locale]string {
	return func(id string) map[discord.Locale]string {
		return localizeMap(locales, func(locale string) string {
			return t.Registry.MustLocalize(i18n.Config{Locale: locale, MessageID: id})
		})
	}
}

func localizeMap(locales []string, fn func(locale string) string) map[discord.Locale]string {
	const baseLocale = "en-US"
	base := strings.TrimSpace(fn(baseLocale))

	out := map[discord.Locale]string{}
	for _, locale := range locales {
		locale = strings.TrimSpace(locale)
		if locale == "" || strings.EqualFold(locale, baseLocale) {
			continue
		}

		translated := strings.TrimSpace(fn(locale))
		if translated == "" {
			continue
		}
		if base != "" && translated == base {
			continue
		}
		out[discord.Locale(locale)] = translated
	}
	return out
}

func blockCreate(locales []string, t commandtext.Translator) discord.ApplicationCommandCreate {
	maxLen := 255
	minLen := 1
	perm := discord.PermissionAdministrator
	loc := createLocalizer(locales, t)

	return discord.SlashCommandCreate{
		Name:                     "block",
		NameLocalizations:        loc("cmd.block.name"),
		Description:              t.S("cmd.block.desc", nil),
		DescriptionLocalizations: loc("cmd.block.desc"),
		DefaultMemberPermissions: omit.NewPtr(perm),
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:                     "user",
				NameLocalizations:        loc("cmd.block.sub.user.name"),
				Description:              t.S("cmd.block.sub.user.desc", nil),
				DescriptionLocalizations: loc("cmd.block.sub.user.desc"),
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionUser{
						Name:                     "user",
						NameLocalizations:        loc("cmd.block.opt.user.name"),
						Description:              t.S("cmd.block.opt.user.desc", nil),
						DescriptionLocalizations: loc("cmd.block.opt.user.desc"),
						Required:                 true,
					},
					discord.ApplicationCommandOptionString{
						Name:                     "reason",
						NameLocalizations:        loc("cmd.block.opt.reason.name"),
						Description:              t.S("cmd.block.opt.reason.desc", nil),
						DescriptionLocalizations: loc("cmd.block.opt.reason.desc"),
						Required:                 true,
						MinLength:                &minLen,
						MaxLength:                &maxLen,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:                     "guild",
				NameLocalizations:        loc("cmd.block.sub.guild.name"),
				Description:              t.S("cmd.block.sub.guild.desc", nil),
				DescriptionLocalizations: loc("cmd.block.sub.guild.desc"),
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:                     "guild_id",
						NameLocalizations:        loc("cmd.block.opt.guild_id.name"),
						Description:              t.S("cmd.block.opt.guild_id.desc", nil),
						DescriptionLocalizations: loc("cmd.block.opt.guild_id.desc"),
						Required:                 true,
						MinLength:                &minLen,
						MaxLength:                &maxLen,
					},
					discord.ApplicationCommandOptionString{
						Name:                     "reason",
						NameLocalizations:        loc("cmd.block.opt.reason.name"),
						Description:              t.S("cmd.block.opt.reason.desc", nil),
						DescriptionLocalizations: loc("cmd.block.opt.reason.desc"),
						Required:                 true,
						MinLength:                &minLen,
						MaxLength:                &maxLen,
					},
				},
			},
		},
	}
}

func unblockCreate(locales []string, t commandtext.Translator) discord.ApplicationCommandCreate {
	maxLen := 255
	minLen := 1
	perm := discord.PermissionAdministrator
	loc := createLocalizer(locales, t)

	return discord.SlashCommandCreate{
		Name:                     "unblock",
		NameLocalizations:        loc("cmd.unblock.name"),
		Description:              t.S("cmd.unblock.desc", nil),
		DescriptionLocalizations: loc("cmd.unblock.desc"),
		DefaultMemberPermissions: omit.NewPtr(perm),
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:                     "user",
				NameLocalizations:        loc("cmd.unblock.sub.user.name"),
				Description:              t.S("cmd.unblock.sub.user.desc", nil),
				DescriptionLocalizations: loc("cmd.unblock.sub.user.desc"),
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionUser{
						Name:                     "user",
						NameLocalizations:        loc("cmd.unblock.opt.user.name"),
						Description:              t.S("cmd.unblock.opt.user.desc", nil),
						DescriptionLocalizations: loc("cmd.unblock.opt.user.desc"),
						Required:                 true,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:                     "guild",
				NameLocalizations:        loc("cmd.unblock.sub.guild.name"),
				Description:              t.S("cmd.unblock.sub.guild.desc", nil),
				DescriptionLocalizations: loc("cmd.unblock.sub.guild.desc"),
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:                     "guild_id",
						NameLocalizations:        loc("cmd.unblock.opt.guild_id.name"),
						Description:              t.S("cmd.unblock.opt.guild_id.desc", nil),
						DescriptionLocalizations: loc("cmd.unblock.opt.guild_id.desc"),
						Required:                 true,
						MinLength:                &minLen,
						MaxLength:                &maxLen,
					},
				},
			},
		},
	}
}

func modulesCreate() discord.ApplicationCommandCreate {
	return discord.SlashCommandCreate{
		Name:        "modules",
		Description: "Inspect and manage modules",
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:        "list",
				Description: "List built-ins and plugins",
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "info",
				Description: "Show one module",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "module",
						Description: "Module ID",
						Required:    true,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "enable",
				Description: "Enable one module",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "module",
						Description: "Module ID",
						Required:    true,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "disable",
				Description: "Disable one module",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "module",
						Description: "Module ID",
						Required:    true,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "reset",
				Description: "Reset one module to its default",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{
						Name:        "module",
						Description: "Module ID",
						Required:    true,
					},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "reload",
				Description: "Reload official and user plugins",
			},
		},
	}
}

func pluginsCreate(locales []string, t commandtext.Translator) discord.ApplicationCommandCreate {
	loc := createLocalizer(locales, t)

	return discord.SlashCommandCreate{
		Name:                     "plugins",
		NameLocalizations:        loc("cmd.plugins.name"),
		Description:              t.S("cmd.plugins.desc", nil),
		DescriptionLocalizations: loc("cmd.plugins.desc"),
		Options: []discord.ApplicationCommandOption{
			discord.ApplicationCommandOptionSubCommand{
				Name:                     "list",
				NameLocalizations:        loc("cmd.plugins.sub.list.name"),
				Description:              t.S("cmd.plugins.sub.list.desc", nil),
				DescriptionLocalizations: loc("cmd.plugins.sub.list.desc"),
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:                     "reload",
				NameLocalizations:        loc("cmd.plugins.sub.reload.name"),
				Description:              t.S("cmd.plugins.sub.reload.desc", nil),
				DescriptionLocalizations: loc("cmd.plugins.sub.reload.desc"),
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "search",
				Description: "Search cached marketplace plugins",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{Name: "term", Description: "Search term", Required: false},
					discord.ApplicationCommandOptionString{Name: "source_id", Description: "Marketplace source id", Required: false},
					discord.ApplicationCommandOptionBool{Name: "refresh", Description: "Sync before searching", Required: false},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "install",
				Description: "Install a marketplace plugin",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{Name: "source_id", Description: "Marketplace source id", Required: true},
					discord.ApplicationCommandOptionString{Name: "plugin_id", Description: "Plugin id", Required: true},
					discord.ApplicationCommandOptionBool{Name: "force", Description: "Replace existing marketplace install", Required: false},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "update",
				Description: "Update a marketplace plugin",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{Name: "plugin_id", Description: "Plugin id", Required: true},
					discord.ApplicationCommandOptionBool{Name: "force", Description: "Replace local modifications", Required: false},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "uninstall",
				Description: "Uninstall a marketplace plugin",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{Name: "plugin_id", Description: "Plugin id", Required: true},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "trust-signer",
				Description: "Trust a signer public key",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{Name: "key_id", Description: "Signer key id", Required: true},
					discord.ApplicationCommandOptionString{Name: "public_key_b64", Description: "Base64 ed25519 public key", Required: true},
					discord.ApplicationCommandOptionString{Name: "vendor_id", Description: "Optional vendor id", Required: false},
				},
			},
			discord.ApplicationCommandOptionSubCommand{
				Name:        "trust-vendor",
				Description: "Trust vendor keys from a source or file",
				Options: []discord.ApplicationCommandOption{
					discord.ApplicationCommandOptionString{Name: "vendor_id", Description: "Vendor id", Required: true},
					discord.ApplicationCommandOptionString{Name: "name", Description: "Vendor display name", Required: true},
					discord.ApplicationCommandOptionString{Name: "source_id", Description: "Marketplace source id", Required: false},
					discord.ApplicationCommandOptionString{Name: "trusted_keys_path", Description: "Path to trusted_keys.json", Required: false},
					discord.ApplicationCommandOptionString{Name: "website_url", Description: "Vendor website", Required: false},
					discord.ApplicationCommandOptionString{Name: "support_url", Description: "Vendor support URL", Required: false},
				},
			},
		},
	}
}
