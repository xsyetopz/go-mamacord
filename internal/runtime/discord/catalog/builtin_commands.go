package catalog

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"

	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	"github.com/xsyetopz/go-mamacord/internal/commands"
	commandspec "github.com/xsyetopz/go-mamacord/internal/commands/spec"
	commandtext "github.com/xsyetopz/go-mamacord/internal/commandtext"
	"github.com/xsyetopz/go-mamacord/internal/marketplace"
	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

func BuiltinCommands(desc commands.ModuleDescriptor) []slashcmd.Command {
	if desc.Definitions != nil {
		return builtinCommandsFromDefinitions(desc.ID, desc.Definitions())
	}
	return nil
}

func builtinCommandsFromDefinitions(moduleID string, defs []commandspec.SlashCommand) []slashcmd.Command {
	out := make([]slashcmd.Command, 0, len(defs))
	for _, def := range defs {
		switch strings.TrimSpace(moduleID) {
		case "core":
			if cmd, ok := coreBuiltinCommand(def); ok {
				out = append(out, cmd)
			}
		case "admin":
			if cmd, ok := adminBuiltinCommand(def); ok {
				out = append(out, cmd)
			}
		}
	}
	return out
}

func coreBuiltinCommand(def commandspec.SlashCommand) (slashcmd.Command, bool) {
	switch strings.TrimSpace(def.Name) {
	case "ping":
		return slashcmd.Command{
			Name:   def.Name,
			NameID: def.NameID,
			DescID: def.DescID,
			Handle: func(ctx context.Context, e *events.ApplicationCommandInteractionCreate, t commandtext.Translator, s commandruntime.Services) (interactions.SlashAction, error) {
				_ = ctx
				_ = e
				_ = s
				return interactions.SlashMessage{
					Create: discord.NewMessageCreate().
						WithEphemeral(true).
						WithContent(t.S("ok.pong", nil)),
				}, nil
			},
		}, true
	case "help":
		return slashcmd.Command{
			Name:   def.Name,
			NameID: def.NameID,
			DescID: def.DescID,
			Handle: func(ctx context.Context, e *events.ApplicationCommandInteractionCreate, t commandtext.Translator, s commandruntime.Services) (interactions.SlashAction, error) {
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
		}, true
	default:
		return slashcmd.Command{}, false
	}
}

func adminBuiltinCommand(def commandspec.SlashCommand) (slashcmd.Command, bool) {
	switch strings.TrimSpace(def.Name) {
	case "block":
		return slashcmd.Command{
			Name:   def.Name,
			NameID: def.NameID,
			DescID: def.DescID,
			Handle: blockBuiltinHandle,
		}, true
	case "modules":
		return slashcmd.Command{
			Name:   def.Name,
			NameID: def.NameID,
			DescID: def.DescID,
			Handle: modulesBuiltinHandle,
		}, true
	case "plugins":
		return slashcmd.Command{
			Name:   def.Name,
			NameID: def.NameID,
			DescID: def.DescID,
			Handle: pluginsBuiltinHandle,
		}, true
	case "unblock":
		return slashcmd.Command{
			Name:   def.Name,
			NameID: def.NameID,
			DescID: def.DescID,
			Handle: unblockBuiltinHandle,
		}, true
	default:
		return slashcmd.Command{}, false
	}
}

func parseUint64Base10(raw string) (uint64, bool) {
	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func blockBuiltinHandle(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	s commandruntime.Services,
) (interactions.SlashAction, error) {
	actorID := uint64(e.User().ID)
	if s.IsOwner == nil || !s.IsOwner(actorID) {
		msg := discord.NewMessageCreate().WithEphemeral(true).WithContent(t.S("admin.not_owner", nil))
		return interactions.SlashMessage{Create: msg}, nil
	}
	if s.Store == nil {
		return nil, errors.New("store not configured")
	}

	data := e.SlashCommandInteractionData()
	sub := data.SubCommandName
	if sub == nil {
		msg := discord.NewMessageCreate().WithEphemeral(true).WithContent(t.S("err.generic", nil))
		return interactions.SlashMessage{Create: msg}, nil
	}

	restrictions := s.Store.Restrictions()
	switch *sub {
	case "user":
		return blockBuiltinUser(ctx, t, restrictions, actorID, data)
	case "guild":
		return blockBuiltinGuild(ctx, e, t, restrictions, actorID, data)
	default:
		msg := discord.NewMessageCreate().WithEphemeral(true).WithContent(t.S("err.generic", nil))
		return interactions.SlashMessage{Create: msg}, nil
	}
}

func blockBuiltinUser(
	ctx context.Context,
	t commandtext.Translator,
	restrictions store.RestrictionStore,
	actorID uint64,
	data discord.SlashCommandInteractionData,
) (interactions.SlashAction, error) {
	user := data.User("user")
	reason := strings.TrimSpace(data.String("reason"))

	if err := restrictions.PutRestriction(ctx, store.Restriction{
		TargetType: store.TargetTypeUser,
		TargetID:   uint64(user.ID),
		Reason:     reason,
		CreatedBy:  actorID,
		CreatedAt:  time.Now().UTC(),
	}); err != nil {
		return nil, err
	}
	msg := discord.NewMessageCreate().
		WithEphemeral(true).
		WithContent(t.S("admin.block.user.success", map[string]any{"User": user.Mention()}))
	return interactions.SlashMessage{Create: msg}, nil
}

func blockBuiltinGuild(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	restrictions store.RestrictionStore,
	actorID uint64,
	data discord.SlashCommandInteractionData,
) (interactions.SlashAction, error) {
	raw := strings.TrimSpace(data.String("guild_id"))
	guildID, ok := parseUint64Base10(raw)
	if !ok {
		return interactions.SlashMessage{
			Create: discord.NewMessageCreate().
				WithEphemeral(true).
				WithContent(t.S("admin.block.guild.invalid", map[string]any{"GuildID": raw})),
		}, nil
	}

	reason := strings.TrimSpace(data.String("reason"))
	if err := restrictions.PutRestriction(ctx, store.Restriction{
		TargetType: store.TargetTypeGuild,
		TargetID:   guildID,
		Reason:     reason,
		CreatedBy:  actorID,
		CreatedAt:  time.Now().UTC(),
	}); err != nil {
		return nil, err
	}

	if currentGuild := e.GuildID(); currentGuild != nil && uint64(*currentGuild) == guildID {
		go func() { _ = e.Client().Rest.LeaveGuild(snowflake.ID(guildID)) }()
	}

	msg := discord.NewMessageCreate().
		WithEphemeral(true).
		WithContent(t.S("admin.block.guild.success", map[string]any{"GuildID": raw}))
	return interactions.SlashMessage{Create: msg}, nil
}

func modulesBuiltinHandle(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	s commandruntime.Services,
) (interactions.SlashAction, error) {
	actorID := uint64(e.User().ID)
	if s.IsOwner == nil || !s.IsOwner(actorID) {
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{
				interactions.Embed("", t.S("admin.not_owner", nil), interactions.ThemeColorError),
			}, true),
		}, nil
	}
	if s.Modules == nil || !s.Modules.Configured() {
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{
				interactions.Embed("Modules", "Modules are not configured.", interactions.ThemeColorWarning),
			}, true),
		}, nil
	}

	data := e.SlashCommandInteractionData()
	sub := data.SubCommandName
	if sub == nil {
		return interactions.SlashMessage{
			Create: discord.NewMessageCreate().WithEphemeral(true).WithContent("Missing subcommand."),
		}, nil
	}

	switch *sub {
	case "list":
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{modulesListEmbed(s.Modules.Infos())}, true),
		}, nil
	case "info":
		moduleID := strings.TrimSpace(data.String("module"))
		for _, info := range s.Modules.Infos() {
			if info.ID != moduleID {
				continue
			}
			return interactions.SlashMessage{
				Create: interactions.MessageEmbeds([]discord.Embed{moduleInfoEmbed(info)}, true),
			}, nil
		}
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{
				interactions.Embed("Modules", "Unknown module: `"+moduleID+"`", interactions.ThemeColorWarning),
			}, true),
		}, nil
	case "enable":
		moduleID := strings.TrimSpace(data.String("module"))
		if err := s.Modules.SetEnabled(ctx, moduleID, true, actorID); err != nil {
			return interactions.SlashMessage{
				Create: interactions.MessageEmbeds([]discord.Embed{
					interactions.Embed("Modules", "Enable failed: "+err.Error(), interactions.ThemeColorError),
				}, true),
			}, nil
		}
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{
				interactions.Embed("Modules", "Enabled: `"+moduleID+"`", interactions.ThemeColorSuccess),
			}, true),
		}, nil
	case "disable":
		moduleID := strings.TrimSpace(data.String("module"))
		if err := s.Modules.SetEnabled(ctx, moduleID, false, actorID); err != nil {
			return interactions.SlashMessage{
				Create: interactions.MessageEmbeds([]discord.Embed{
					interactions.Embed("Modules", "Disable failed: "+err.Error(), interactions.ThemeColorError),
				}, true),
			}, nil
		}
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{
				interactions.Embed("Modules", "Disabled: `"+moduleID+"`", interactions.ThemeColorWarning),
			}, true),
		}, nil
	case "reset":
		moduleID := strings.TrimSpace(data.String("module"))
		if err := s.Modules.Reset(ctx, moduleID); err != nil {
			return interactions.SlashMessage{
				Create: interactions.MessageEmbeds([]discord.Embed{
					interactions.Embed("Modules", "Reset failed: "+err.Error(), interactions.ThemeColorError),
				}, true),
			}, nil
		}
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{
				interactions.Embed("Modules", "Reset to default: `"+moduleID+"`", interactions.ThemeColorSuccess),
			}, true),
		}, nil
	case "reload":
		if err := s.Modules.Reload(ctx); err != nil {
			return interactions.SlashMessage{
				Create: interactions.MessageEmbeds([]discord.Embed{
					interactions.Embed("Modules", "Reload failed: "+err.Error(), interactions.ThemeColorError),
				}, true),
			}, nil
		}
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{
				interactions.Embed("Modules", "Reloaded.", interactions.ThemeColorSuccess),
			}, true),
		}, nil
	default:
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{
				interactions.Embed("Modules", "Unknown subcommand.", interactions.ThemeColorError),
			}, true),
		}, nil
	}
}

func pluginsBuiltinHandle(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	s commandruntime.Services,
) (interactions.SlashAction, error) {
	actorID := uint64(e.User().ID)
	if s.IsOwner == nil || !s.IsOwner(actorID) {
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{
				interactions.Embed("", t.S("admin.not_owner", nil), interactions.ThemeColorError),
			}, true),
		}, nil
	}
	if s.Plugins == nil || !s.Plugins.Configured() {
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{
				interactions.Embed("Plugins", t.S("admin.plugins.not_configured", nil), interactions.ThemeColorWarning),
			}, true),
		}, nil
	}

	data := e.SlashCommandInteractionData()
	sub := data.SubCommandName
	if sub == nil {
		return interactions.SlashMessage{
			Create: discord.NewMessageCreate().WithEphemeral(true).WithContent(t.S("err.generic", nil)),
		}, nil
	}

	switch *sub {
	case "list":
		return pluginsBuiltinHandleList(t, s.Plugins), nil
	case "reload":
		if err := s.Plugins.Reload(ctx); err != nil {
			return nil, err
		}
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{
				interactions.Embed("Plugins", t.S("admin.plugins.reloaded", nil), interactions.ThemeColorSuccess),
			}, true),
		}, nil
	case "search":
		if s.Marketplace == nil || !s.Marketplace.Configured() {
			return marketplaceUnavailableAction(), nil
		}
		results, err := s.Marketplace.Search(ctx, marketplace.SearchQuery{
			Term:     strings.TrimSpace(data.String("term")),
			SourceID: strings.TrimSpace(data.String("source_id")),
			Refresh:  data.Bool("refresh"),
		})
		if err != nil {
			return nil, err
		}
		return marketplaceSearchAction(results), nil
	case "install":
		if s.Marketplace == nil || !s.Marketplace.Configured() {
			return marketplaceUnavailableAction(), nil
		}
		actor := actorID
		result, err := s.Marketplace.Install(ctx, marketplace.InstallRequest{
			SourceID: strings.TrimSpace(data.String("source_id")),
			PluginID: strings.TrimSpace(data.String("plugin_id")),
			Force:    data.Bool("force"),
			ActorID:  &actor,
		})
		if err != nil {
			return nil, err
		}
		return marketplaceResultAction("Installed", result.PluginID+" from "+result.SourceID+" at "+result.GitRevision, interactions.ThemeColorSuccess), nil
	case "update":
		if s.Marketplace == nil || !s.Marketplace.Configured() {
			return marketplaceUnavailableAction(), nil
		}
		actor := actorID
		result, err := s.Marketplace.Update(ctx, marketplace.UpdateRequest{
			PluginID: strings.TrimSpace(data.String("plugin_id")),
			Force:    data.Bool("force"),
			ActorID:  &actor,
		})
		if err != nil {
			return nil, err
		}
		return marketplaceResultAction("Updated", result.PluginID+" to "+result.GitRevision, interactions.ThemeColorSuccess), nil
	case "uninstall":
		if s.Marketplace == nil || !s.Marketplace.Configured() {
			return marketplaceUnavailableAction(), nil
		}
		pluginID := strings.TrimSpace(data.String("plugin_id"))
		if err := s.Marketplace.Uninstall(ctx, marketplace.UninstallRequest{PluginID: pluginID}); err != nil {
			return nil, err
		}
		return marketplaceResultAction("Uninstalled", pluginID, interactions.ThemeColorWarning), nil
	case "trust-signer":
		if s.Marketplace == nil || !s.Marketplace.Configured() {
			return marketplaceUnavailableAction(), nil
		}
		if err := s.Marketplace.TrustSigner(ctx, marketplace.TrustSignerRequest{
			KeyID:        strings.TrimSpace(data.String("key_id")),
			PublicKeyB64: strings.TrimSpace(data.String("public_key_b64")),
			VendorID:     strings.TrimSpace(data.String("vendor_id")),
		}); err != nil {
			return nil, err
		}
		return marketplaceResultAction("Trusted signer", strings.TrimSpace(data.String("key_id")), interactions.ThemeColorSuccess), nil
	case "trust-vendor":
		if s.Marketplace == nil || !s.Marketplace.Configured() {
			return marketplaceUnavailableAction(), nil
		}
		resp, err := s.Marketplace.TrustVendor(ctx, marketplace.TrustVendorRequest{
			VendorID:        strings.TrimSpace(data.String("vendor_id")),
			Name:            strings.TrimSpace(data.String("name")),
			SourceID:        strings.TrimSpace(data.String("source_id")),
			TrustedKeysPath: strings.TrimSpace(data.String("trusted_keys_path")),
			WebsiteURL:      strings.TrimSpace(data.String("website_url")),
			SupportURL:      strings.TrimSpace(data.String("support_url")),
		})
		if err != nil {
			return nil, err
		}
		return marketplaceResultAction("Trusted vendor", resp.VendorID+" keys: "+strings.Join(resp.KeyIDs, ", "), interactions.ThemeColorSuccess), nil
	default:
		return interactions.SlashMessage{
			Create: discord.NewMessageCreate().WithEphemeral(true).WithContent(t.S("err.generic", nil)),
		}, nil
	}
}

func pluginsBuiltinHandleList(t commandtext.Translator, p commandruntime.PluginAdmin) interactions.SlashAction {
	infos := p.Infos()
	if len(infos) == 0 {
		return interactions.SlashMessage{
			Create: interactions.MessageEmbeds([]discord.Embed{
				interactions.Embed("Plugins", t.S("admin.plugins.none", nil), interactions.ThemeColorBrand),
			}, true),
		}
	}

	lines := make([]string, 0, len(infos))
	for _, info := range infos {
		lines = append(lines, formatPluginInfoLine(info))
	}

	return interactions.SlashMessage{
		Create: interactions.MessageEmbeds([]discord.Embed{
			pluginsListEmbed(t, lines, len(infos)),
		}, true),
	}
}

func formatPluginInfoLine(info pluginhost.PluginInfo) string {
	sig := "⚠️"
	if info.Signed {
		sig = "🔏"
	}

	version := strings.TrimSpace(info.Version)
	if version == "" {
		version = "dev"
	}
	return sig + " " + info.ID + " " + version + " · " + strconv.Itoa(len(info.Commands)) + " cmds"
}

func pluginsListEmbed(t commandtext.Translator, lines []string, count int) discord.Embed {
	sort.Strings(lines)
	desc := strings.TrimSpace(t.S("admin.plugins.list", map[string]any{
		"Count": count,
		"List":  "",
	}))
	e := interactions.Embed("Plugins", desc, interactions.ThemeColorBrand)
	e.Fields = interactions.EmbedFieldChunked("\u200b", lines, false)
	return e
}

func marketplaceUnavailableAction() interactions.SlashAction {
	return interactions.SlashMessage{
		Create: interactions.MessageEmbeds([]discord.Embed{
			interactions.Embed("Marketplace", "Marketplace is not configured.", interactions.ThemeColorWarning),
		}, true),
	}
}

func marketplaceSearchAction(results []marketplace.PluginCandidate) interactions.SlashAction {
	if len(results) == 0 {
		return marketplaceResultAction("Marketplace", "No matching plugins found.", interactions.ThemeColorBrand)
	}
	lines := make([]string, 0, len(results))
	for _, item := range results {
		label := item.PluginID + "@" + item.SourceID
		if item.Version != "" {
			label += " " + item.Version
		}
		if item.SignatureState != "" {
			label += " [" + string(item.SignatureState) + "]"
		}
		lines = append(lines, label)
	}
	return interactions.SlashMessage{
		Create: interactions.MessageEmbeds([]discord.Embed{
			func() discord.Embed {
				e := interactions.Embed("Marketplace", fmt.Sprintf("%d result(s)", len(results)), interactions.ThemeColorBrand)
				e.Fields = interactions.EmbedFieldChunked("\u200b", lines, false)
				return e
			}(),
		}, true),
	}
}

func marketplaceResultAction(title, description string, color int) interactions.SlashAction {
	return interactions.SlashMessage{
		Create: interactions.MessageEmbeds([]discord.Embed{
			interactions.Embed(title, description, color),
		}, true),
	}
}

func unblockBuiltinHandle(
	ctx context.Context,
	e *events.ApplicationCommandInteractionCreate,
	t commandtext.Translator,
	s commandruntime.Services,
) (interactions.SlashAction, error) {
	actorID := uint64(e.User().ID)
	if s.IsOwner == nil || !s.IsOwner(actorID) {
		return interactions.SlashMessage{
			Create: discord.NewMessageCreate().WithEphemeral(true).WithContent(t.S("admin.not_owner", nil)),
		}, nil
	}
	if s.Store == nil {
		return nil, errors.New("store not configured")
	}

	data := e.SlashCommandInteractionData()
	sub := data.SubCommandName
	if sub == nil {
		return interactions.SlashMessage{
			Create: discord.NewMessageCreate().WithEphemeral(true).WithContent(t.S("err.generic", nil)),
		}, nil
	}

	restrictions := s.Store.Restrictions()

	switch *sub {
	case "user":
		user := data.User("user")
		if err := restrictions.DeleteRestriction(ctx, store.TargetTypeUser, uint64(user.ID)); err != nil {
			return nil, err
		}
		return interactions.SlashMessage{
			Create: discord.NewMessageCreate().
				WithEphemeral(true).
				WithContent(t.S("admin.unblock.user.success", map[string]any{"User": user.Mention()})),
		}, nil

	case "guild":
		raw := strings.TrimSpace(data.String("guild_id"))
		guildID, ok := parseUint64Base10(raw)
		if !ok {
			return interactions.SlashMessage{
				Create: discord.NewMessageCreate().
					WithEphemeral(true).
					WithContent(t.S("admin.unblock.guild.invalid", map[string]any{"GuildID": raw})),
			}, nil
		}
		if err := restrictions.DeleteRestriction(ctx, store.TargetTypeGuild, guildID); err != nil {
			return nil, err
		}
		return interactions.SlashMessage{
			Create: discord.NewMessageCreate().
				WithEphemeral(true).
				WithContent(t.S("admin.unblock.guild.success", map[string]any{"GuildID": raw})),
		}, nil
	default:
		return interactions.SlashMessage{
			Create: discord.NewMessageCreate().WithEphemeral(true).WithContent(t.S("err.generic", nil)),
		}, nil
	}
}

func modulesListEmbed(infos []moduleapi.Info) discord.Embed {
	if len(infos) == 0 {
		return interactions.Embed("Modules", "No modules are loaded.", interactions.ThemeColorBrand)
	}

	sort.Slice(infos, func(i, j int) bool { return infos[i].ID < infos[j].ID })

	builtins := []string{}
	plugins := []string{}

	for _, info := range infos {
		prefix := "⛔"
		if info.Enabled {
			prefix = "✅"
		}
		lock := ""
		if !info.Toggleable {
			lock = " 🔒"
		}
		line := prefix + " " + info.ID + lock

		switch info.Kind {
		case moduleapi.KindCoreBuiltin:
			builtins = append(builtins, line)
		case moduleapi.KindPlugin:
			plugins = append(plugins, line)
		default:
			plugins = append(plugins, line)
		}
	}

	e := interactions.Embed("Modules", fmt.Sprintf("Loaded: %d", len(infos)), interactions.ThemeColorBrand)
	e.Fields = append(e.Fields, interactions.EmbedFieldChunked("Built-ins", builtins, false)...)
	e.Fields = append(e.Fields, interactions.EmbedFieldChunked("Plugins", plugins, false)...)
	return e
}

func moduleInfoEmbed(info moduleapi.Info) discord.Embed {
	title := strings.TrimSpace(info.Name)
	if title == "" {
		title = info.ID
	}
	e := interactions.Embed(title, "Module: `"+info.ID+"`", interactions.ThemeColorBrand)

	state := "⛔ disabled"
	color := interactions.ThemeColorWarning
	if info.Enabled {
		state = "✅ enabled"
		color = interactions.ThemeColorSuccess
	}
	e.Color = color

	sig := "⚠️ unsigned"
	if info.Signed {
		sig = "🔏 signed"
	}

	e.Fields = []discord.EmbedField{
		{Name: "State", Value: state, Inline: interactions.Bool(true)},
		{Name: "Kind", Value: string(info.Kind), Inline: interactions.Bool(true)},
		{Name: "Runtime", Value: string(info.Runtime), Inline: interactions.Bool(true)},
		{Name: "Signed", Value: sig, Inline: interactions.Bool(true)},
		{Name: "Toggleable", Value: strconv.FormatBool(info.Toggleable), Inline: interactions.Bool(true)},
		{Name: "Default", Value: strconv.FormatBool(info.DefaultEnabled), Inline: interactions.Bool(true)},
	}

	if len(info.Commands) == 0 {
		e.Fields = append(e.Fields, discord.EmbedField{Name: "Commands", Value: "none"})
	} else {
		cmdLines := make([]string, 0, len(info.Commands))
		for _, cmd := range info.Commands {
			cmd = strings.TrimSpace(cmd)
			if cmd == "" {
				continue
			}
			if strings.HasPrefix(cmd, "/") {
				cmdLines = append(cmdLines, "`"+cmd+"`")
			} else {
				cmdLines = append(cmdLines, "`/"+cmd+"`")
			}
		}
		e.Fields = append(e.Fields, interactions.EmbedFieldChunked("Commands", cmdLines, false)...)
	}

	if src := strings.TrimSpace(info.Source); src != "" {
		e.Footer = &discord.EmbedFooter{Text: src}
	}
	return e
}
