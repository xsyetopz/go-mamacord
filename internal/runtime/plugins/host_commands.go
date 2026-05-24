package pluginhost

import (
	"sort"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/omit"
)

func (m *Host) CommandCreates() []discord.ApplicationCommandCreate {
	return m.CommandCreatesWithLocalizations(nil, nil)
}

type CommandLocalizer func(pluginID, locale, messageID string) (string, bool)

func (m *Host) CommandCreatesWithLocalizations(
	locales []string,
	localize CommandLocalizer,
) []discord.ApplicationCommandCreate {
	return m.CommandCreatesFiltered(nil, locales, localize)
}

func (m *Host) CommandCreatesFiltered(
	allowedPluginIDs map[string]struct{},
	locales []string,
	localize CommandLocalizer,
) []discord.ApplicationCommandCreate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.commands) == 0 {
		return nil
	}

	names := make([]string, 0, len(m.commands))
	for name := range m.commands {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]discord.ApplicationCommandCreate, 0, len(names))
	for _, key := range names {
		cmd := m.commands[key]
		if len(allowedPluginIDs) != 0 {
			if _, ok := allowedPluginIDs[cmd.PluginID]; !ok {
				continue
			}
		}
		out = append(out, commandToCreate(cmd.PluginID, cmd.Command, locales, localize))
	}
	return out
}
func commandToCreate(
	pluginID string,
	cmd Command,
	locales []string,
	localize CommandLocalizer,
) discord.ApplicationCommandCreate {
	name := cmd.Name
	if NormalizeCommandType(cmd.Type) != CommandTypeSlash {
		name = strings.TrimSpace(cmd.Name)
	}
	var options []discord.ApplicationCommandOption
	if NormalizeCommandType(cmd.Type) == CommandTypeSlash && (len(cmd.Subcommands) > 0 || len(cmd.Groups) > 0) {
		options = append(options, buildSubcommands(pluginID, cmd.Subcommands, locales, localize)...)
		options = append(options, buildGroups(pluginID, cmd.Groups, locales, localize)...)
	} else if NormalizeCommandType(cmd.Type) == CommandTypeSlash {
		options = append(options, buildOptions(pluginID, cmd.Options, locales, localize)...)
	}

	perms, hasPerms := commandPermissions(cmd.DefaultMemberPermissions)
	switch NormalizeCommandType(cmd.Type) {
	case CommandTypeUser:
		create := discord.UserCommandCreate{Name: name}
		if hasPerms {
			create.DefaultMemberPermissions = omit.NewPtr(perms)
		}
		return create
	case CommandTypeMessage:
		create := discord.MessageCommandCreate{Name: name}
		if hasPerms {
			create.DefaultMemberPermissions = omit.NewPtr(perms)
		}
		return create
	default:
		create := discord.SlashCommandCreate{
			Name:        name,
			Description: cmd.Description,
			Options:     options,
		}
		if hasPerms {
			create.DefaultMemberPermissions = omit.NewPtr(perms)
		}
		if locs := descriptionLocalizations(pluginID, cmd.DescriptionID, locales, localize); len(locs) != 0 {
			create.DescriptionLocalizations = locs
		}
		return create
	}
}

func commandPermissions(names []string) (discord.Permissions, bool) {
	if len(names) == 0 {
		return 0, false
	}

	var (
		perms discord.Permissions
		ok    bool
	)
	for _, name := range names {
		perm, found := commandPermissionByName(name)
		if !found {
			continue
		}
		perms |= perm
		ok = true
	}
	return perms, ok
}

func commandPermissionByName(name string) (discord.Permissions, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "administrator":
		return discord.PermissionAdministrator, true
	case "manage_guild":
		return discord.PermissionManageGuild, true
	case "manage_roles":
		return discord.PermissionManageRoles, true
	case "manage_expressions":
		return discord.PermissionManageGuildExpressions, true
	case "create_expressions":
		return discord.PermissionCreateGuildExpressions, true
	case "manage_emojis_and_stickers":
		return discord.PermissionManageGuildExpressions, true
	case "manage_messages":
		return discord.PermissionManageMessages, true
	case "manage_nicknames":
		return discord.PermissionManageNicknames, true
	case "manage_channels":
		return discord.PermissionManageChannels, true
	case "kick_members":
		return discord.PermissionKickMembers, true
	case "ban_members":
		return discord.PermissionBanMembers, true
	case "moderate_members":
		return discord.PermissionModerateMembers, true
	default:
		return 0, false
	}
}

func descriptionLocalizations(
	pluginID string,
	descriptionID string,
	locales []string,
	localize CommandLocalizer,
) map[discord.Locale]string {
	descID := strings.TrimSpace(descriptionID)
	if descID == "" || len(locales) == 0 || localize == nil {
		return nil
	}

	locs := map[discord.Locale]string{}
	for _, locale := range locales {
		s, ok := localize(pluginID, locale, descID)
		if !ok {
			continue
		}
		locs[discord.Locale(locale)] = s
	}
	return locs
}

func buildSubcommands(
	pluginID string,
	cmds []Subcommand,
	locales []string,
	localize CommandLocalizer,
) []discord.ApplicationCommandOption {
	out := make([]discord.ApplicationCommandOption, 0, len(cmds))
	for _, sc := range cmds {
		opt := discord.ApplicationCommandOptionSubCommand{
			Name:        sc.Name,
			Description: sc.Description,
			Options:     buildOptions(pluginID, sc.Options, locales, localize),
		}
		if locs := descriptionLocalizations(pluginID, sc.DescriptionID, locales, localize); len(locs) != 0 {
			opt.DescriptionLocalizations = locs
		}
		out = append(out, opt)
	}
	return out
}

func buildGroups(
	pluginID string,
	groups []CommandGroup,
	locales []string,
	localize CommandLocalizer,
) []discord.ApplicationCommandOption {
	out := make([]discord.ApplicationCommandOption, 0, len(groups))
	for _, g := range groups {
		opt := discord.ApplicationCommandOptionSubCommandGroup{
			Name:        g.Name,
			Description: g.Description,
		}
		if locs := descriptionLocalizations(pluginID, g.DescriptionID, locales, localize); len(locs) != 0 {
			opt.DescriptionLocalizations = locs
		}

		subs := make([]discord.ApplicationCommandOptionSubCommand, 0, len(g.Subcommands))
		for _, sc := range g.Subcommands {
			sub := discord.ApplicationCommandOptionSubCommand{
				Name:        sc.Name,
				Description: sc.Description,
				Options:     buildOptions(pluginID, sc.Options, locales, localize),
			}
			if locs := descriptionLocalizations(pluginID, sc.DescriptionID, locales, localize); len(locs) != 0 {
				sub.DescriptionLocalizations = locs
			}
			subs = append(subs, sub)
		}
		opt.Options = subs

		out = append(out, opt)
	}
	return out
}

func buildOptions(
	pluginID string,
	opts []CommandOption,
	locales []string,
	localize CommandLocalizer,
) []discord.ApplicationCommandOption {
	// Discord requires required options to be listed before non-required options.
	// Plugin authors will naturally write "nice" human ordering; we normalize so
	// that one plugin cannot brick command registration.
	opts = normalizeRequiredOptionsFirst(opts)

	out := make([]discord.ApplicationCommandOption, 0, len(opts))
	for _, opt := range opts {
		if o, ok := buildOption(pluginID, opt, locales, localize); ok {
			out = append(out, o)
		}
	}
	return out
}

func normalizeRequiredOptionsFirst(opts []CommandOption) []CommandOption {
	if len(opts) < 2 {
		return opts
	}

	// Fast-path: already valid ordering.
	seenOptional := false
	needsFix := false
	for _, opt := range opts {
		if !opt.Required {
			seenOptional = true
			continue
		}
		if seenOptional {
			needsFix = true
			break
		}
	}
	if !needsFix {
		return opts
	}

	required := make([]CommandOption, 0, len(opts))
	optional := make([]CommandOption, 0, len(opts))
	for _, opt := range opts {
		if opt.Required {
			required = append(required, opt)
		} else {
			optional = append(optional, opt)
		}
	}
	return append(required, optional...)
}

func buildOption(
	pluginID string,
	opt CommandOption,
	locales []string,
	localize CommandLocalizer,
) (discord.ApplicationCommandOption, bool) {
	typ := strings.ToLower(strings.TrimSpace(opt.Type))
	descLocs := descriptionLocalizations(pluginID, opt.DescriptionID, locales, localize)
	switch typ {
	case "string":
		return buildStringOption(opt, descLocs), true
	case "bool":
		return buildBoolOption(opt, descLocs), true
	case "int":
		return buildIntOption(opt, descLocs), true
	case "float":
		return buildFloatOption(opt, descLocs), true
	case "user":
		return buildUserOption(opt, descLocs), true
	case "channel":
		return buildChannelOption(opt, descLocs), true
	case "role":
		return buildRoleOption(opt, descLocs), true
	case "mentionable":
		return buildMentionableOption(opt, descLocs), true
	case "attachment":
		return buildAttachmentOption(opt, descLocs), true
	default:
		return nil, false
	}
}

func buildStringOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionString {
	choices := buildStringChoices(opt.Choices)
	if strings.TrimSpace(opt.Autocomplete) != "" {
		choices = nil
	}
	o := discord.ApplicationCommandOptionString{
		Name:         opt.Name,
		Description:  opt.Description,
		Required:     opt.Required,
		MinLength:    opt.MinLength,
		MaxLength:    opt.MaxLength,
		Choices:      choices,
		Autocomplete: strings.TrimSpace(opt.Autocomplete) != "",
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildBoolOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionBool {
	o := discord.ApplicationCommandOptionBool{
		Name:        opt.Name,
		Description: opt.Description,
		Required:    opt.Required,
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildIntOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionInt {
	choices := buildIntChoices(opt.Choices)
	if strings.TrimSpace(opt.Autocomplete) != "" {
		choices = nil
	}
	o := discord.ApplicationCommandOptionInt{
		Name:         opt.Name,
		Description:  opt.Description,
		Required:     opt.Required,
		Choices:      choices,
		Autocomplete: strings.TrimSpace(opt.Autocomplete) != "",
		MinValue:     floatToIntPtr(opt.MinValue),
		MaxValue:     floatToIntPtr(opt.MaxValue),
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildFloatOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionFloat {
	choices := buildFloatChoices(opt.Choices)
	if strings.TrimSpace(opt.Autocomplete) != "" {
		choices = nil
	}
	o := discord.ApplicationCommandOptionFloat{
		Name:         opt.Name,
		Description:  opt.Description,
		Required:     opt.Required,
		Choices:      choices,
		Autocomplete: strings.TrimSpace(opt.Autocomplete) != "",
		MinValue:     opt.MinValue,
		MaxValue:     opt.MaxValue,
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildUserOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionUser {
	o := discord.ApplicationCommandOptionUser{
		Name:        opt.Name,
		Description: opt.Description,
		Required:    opt.Required,
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildChannelOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionChannel {
	o := discord.ApplicationCommandOptionChannel{
		Name:         opt.Name,
		Description:  opt.Description,
		Required:     opt.Required,
		ChannelTypes: buildChannelTypes(opt.ChannelTypes),
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildRoleOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionRole {
	o := discord.ApplicationCommandOptionRole{
		Name:        opt.Name,
		Description: opt.Description,
		Required:    opt.Required,
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildMentionableOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionMentionable {
	o := discord.ApplicationCommandOptionMentionable{
		Name:        opt.Name,
		Description: opt.Description,
		Required:    opt.Required,
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildAttachmentOption(
	opt CommandOption,
	descLocs map[discord.Locale]string,
) discord.ApplicationCommandOptionAttachment {
	o := discord.ApplicationCommandOptionAttachment{
		Name:        opt.Name,
		Description: opt.Description,
		Required:    opt.Required,
	}
	if len(descLocs) != 0 {
		o.DescriptionLocalizations = descLocs
	}
	return o
}

func buildStringChoices(in []OptionChoice) []discord.ApplicationCommandOptionChoiceString {
	out := make([]discord.ApplicationCommandOptionChoiceString, 0, len(in))
	for _, c := range in {
		v, ok := c.Value.(string)
		if !ok {
			continue
		}
		out = append(out, discord.ApplicationCommandOptionChoiceString{Name: c.Name, Value: v})
	}
	return out
}

func buildIntChoices(in []OptionChoice) []discord.ApplicationCommandOptionChoiceInt {
	out := make([]discord.ApplicationCommandOptionChoiceInt, 0, len(in))
	for _, c := range in {
		v, ok := floatToInt(c.Value)
		if !ok {
			continue
		}
		out = append(out, discord.ApplicationCommandOptionChoiceInt{Name: c.Name, Value: v})
	}
	return out
}

func buildFloatChoices(in []OptionChoice) []discord.ApplicationCommandOptionChoiceFloat {
	out := make([]discord.ApplicationCommandOptionChoiceFloat, 0, len(in))
	for _, c := range in {
		switch v := c.Value.(type) {
		case float64:
			out = append(out, discord.ApplicationCommandOptionChoiceFloat{Name: c.Name, Value: v})
		case int:
			out = append(out, discord.ApplicationCommandOptionChoiceFloat{Name: c.Name, Value: float64(v)})
		}
	}
	return out
}

func floatToIntPtr(v *float64) *int {
	if v == nil {
		return nil
	}
	if i, ok := floatToInt(*v); ok {
		return &i
	}
	return nil
}

func floatToInt(v any) (int, bool) {
	switch vv := v.(type) {
	case float64:
		if vv != float64(int(vv)) {
			return 0, false
		}
		return int(vv), true
	case int:
		return vv, true
	default:
		return 0, false
	}
}

func buildChannelTypes(in []int) []discord.ChannelType {
	if len(in) == 0 {
		return nil
	}
	out := make([]discord.ChannelType, 0, len(in))
	for _, v := range in {
		out = append(out, discord.ChannelType(v))
	}
	return out
}
