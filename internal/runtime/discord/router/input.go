package router

import (
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/customid"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
)

func SnowflakePtrToString(id *snowflake.ID) string {
	if id == nil {
		return ""
	}
	return id.String()
}
func CommandInput(data discord.SlashCommandInteractionData) contract.CommandInput {
	path := []string{strings.TrimSpace(data.CommandName())}
	if data.SubCommandGroupName != nil {
		path = append(path, strings.TrimSpace(*data.SubCommandGroupName))
	}
	if data.SubCommandName != nil {
		path = append(path, strings.TrimSpace(*data.SubCommandName))
	}
	return contract.CommandInput{Kind: contract.CommandSlash, Path: path, Options: slashOptions(data)}
}
func UserCommandInput(data discord.UserCommandInteractionData) contract.CommandInput {
	user := UserRef(data.TargetUser())
	member := MemberRef(data.TargetMember(), SnowflakePtrToString(data.GuildID()))
	return contract.CommandInput{Kind: contract.CommandUser, Path: []string{strings.TrimSpace(data.CommandName())}, TargetUser: &user, TargetMember: &member}
}
func MessageCommandInput(data discord.MessageCommandInteractionData) contract.CommandInput {
	message := data.TargetMessage()
	target := contract.MessageRef{ID: message.ID.String(), ChannelID: message.ChannelID.String(), Author: UserRef(message.Author), Content: message.Content}
	if message.GuildID != nil {
		target.GuildID = message.GuildID.String()
	}
	return contract.CommandInput{Kind: contract.CommandMessage, Path: []string{strings.TrimSpace(data.CommandName())}, TargetMessage: &target}
}
func AutocompleteInput(data discord.AutocompleteInteractionData) contract.AutocompleteInput {
	path := []string{strings.TrimSpace(data.CommandName)}
	if data.SubCommandGroupName != nil {
		path = append(path, strings.TrimSpace(*data.SubCommandGroupName))
	}
	if data.SubCommandName != nil {
		path = append(path, strings.TrimSpace(*data.SubCommandName))
	}
	all := make([]contract.OptionValue, 0, len(data.All()))
	focused := contract.OptionValue{}
	for _, option := range data.All() {
		converted := autocompleteOption(option)
		if option.Focused {
			focused = converted
		}
		all = append(all, converted)
	}
	return contract.AutocompleteInput{Path: path, Option: focused.Name, Focused: focused, Options: all}
}
func slashOptions(data discord.SlashCommandInteractionData) []contract.OptionValue {
	out := make([]contract.OptionValue, 0, len(data.All()))
	guildID := SnowflakePtrToString(data.GuildID())
	for _, option := range data.All() {
		value := contract.OptionValue{Name: strings.TrimSpace(option.Name)}
		switch option.Type {
		case discord.ApplicationCommandOptionTypeString:
			value.Kind = contract.OptionString
			value.String = option.String()
		case discord.ApplicationCommandOptionTypeInt:
			value.Kind = contract.OptionInteger
			value.Integer = int64(option.Int())
		case discord.ApplicationCommandOptionTypeBool:
			value.Kind = contract.OptionBoolean
			value.Boolean = option.Bool()
		case discord.ApplicationCommandOptionTypeFloat:
			value.Kind = contract.OptionNumber
			value.Number = option.Float()
		case discord.ApplicationCommandOptionTypeUser:
			value.Kind = contract.OptionUser
			user := UserRef(data.User(option.Name))
			value.User = &user
		case discord.ApplicationCommandOptionTypeChannel:
			value.Kind = contract.OptionChannel
			channel := ChannelRef(data.Channel(option.Name), guildID)
			value.Channel = &channel
		case discord.ApplicationCommandOptionTypeRole:
			value.Kind = contract.OptionRole
			role := RoleRef(data.Role(option.Name), guildID)
			value.Role = &role
		case discord.ApplicationCommandOptionTypeMentionable:
			value.Kind = contract.OptionMentionable
			id := option.Snowflake()
			if user, ok := data.OptUser(option.Name); ok {
				ref := UserRef(user)
				value.Mentionable = &contract.MentionableRef{Kind: contract.MentionableUser, User: &ref}
			} else if role, ok := data.OptRole(option.Name); ok {
				ref := RoleRef(role, guildID)
				value.Mentionable = &contract.MentionableRef{Kind: contract.MentionableRole, Role: &ref}
			} else {
				_ = id
			}
		case discord.ApplicationCommandOptionTypeAttachment:
			value.Kind = contract.OptionAttachment
			attachment := data.Attachment(option.Name)
			ref := AttachmentRef(attachment)
			value.Attachment = &ref
		default:
			continue
		}
		out = append(out, value)
	}
	return out
}
func autocompleteOption(option discord.AutocompleteOption) contract.OptionValue {
	value := contract.OptionValue{Name: strings.TrimSpace(option.Name)}
	switch option.Type {
	case discord.ApplicationCommandOptionTypeString:
		value.Kind = contract.OptionString
		value.String = option.String()
	case discord.ApplicationCommandOptionTypeInt:
		value.Kind = contract.OptionInteger
		value.Integer = int64(option.Int())
	case discord.ApplicationCommandOptionTypeFloat:
		value.Kind = contract.OptionNumber
		value.Number = option.Float()
	case discord.ApplicationCommandOptionTypeBool:
		value.Kind = contract.OptionBoolean
		value.Boolean = option.Bool()
	case discord.ApplicationCommandOptionTypeUser:
		value.Kind = contract.OptionUser
		value.User = &contract.UserRef{ID: option.Snowflake().String()}
	case discord.ApplicationCommandOptionTypeChannel:
		value.Kind = contract.OptionChannel
		value.Channel = &contract.ChannelRef{ID: option.Snowflake().String(), Kind: contract.ChannelText}
	case discord.ApplicationCommandOptionTypeRole:
		value.Kind = contract.OptionRole
		value.Role = &contract.RoleRef{RoleIdentity: contract.RoleIdentity{ID: option.Snowflake().String()}}
	case discord.ApplicationCommandOptionTypeMentionable:
		value.Kind = contract.OptionMentionable
		value.Mentionable = &contract.MentionableRef{Kind: contract.MentionableUser, User: &contract.UserRef{ID: option.Snowflake().String()}}
	}
	return value
}
func ComponentInput(e *events.ComponentInteractionCreate, localID string) contract.ComponentInput {
	input := contract.ComponentInput{ID: localID}
	switch e.Data.Type() {
	case discord.ComponentTypeButton:
		input.Kind = contract.ComponentButton
	case discord.ComponentTypeStringSelectMenu:
		input.Kind = contract.ComponentStringSelect
		for _, item := range e.StringSelectMenuInteractionData().Values {
			input.Values = append(input.Values, contract.OptionValue{Kind: contract.OptionString, ScalarOptionValue: contract.ScalarOptionValue{String: item}})
		}
	case discord.ComponentTypeUserSelectMenu:
		input.Kind = contract.ComponentUserSelect
		data := e.UserSelectMenuInteractionData()
		for _, id := range data.Values {
			user := UserRef(data.Resolved.Users[id])
			input.Values = append(input.Values, contract.OptionValue{Kind: contract.OptionUser, ReferenceOptionValue: contract.ReferenceOptionValue{User: &user}})
		}
	case discord.ComponentTypeRoleSelectMenu:
		input.Kind = contract.ComponentRoleSelect
		data := e.RoleSelectMenuInteractionData()
		guildID := SnowflakePtrToString(e.GuildID())
		for _, id := range data.Values {
			role := RoleRef(data.Resolved.Roles[id], guildID)
			input.Values = append(input.Values, contract.OptionValue{Kind: contract.OptionRole, ReferenceOptionValue: contract.ReferenceOptionValue{Role: &role}})
		}
	case discord.ComponentTypeMentionableSelectMenu:
		input.Kind = contract.ComponentMentionableSelect
		data := e.MentionableSelectMenuInteractionData()
		guildID := SnowflakePtrToString(e.GuildID())
		for _, id := range data.Values {
			if user, ok := data.Resolved.Users[id]; ok {
				ref := UserRef(user)
				input.Values = append(input.Values, contract.OptionValue{Kind: contract.OptionMentionable, ReferenceOptionValue: contract.ReferenceOptionValue{Mentionable: &contract.MentionableRef{Kind: contract.MentionableUser, User: &ref}}})
			} else if role, ok := data.Resolved.Roles[id]; ok {
				ref := RoleRef(role, guildID)
				input.Values = append(input.Values, contract.OptionValue{Kind: contract.OptionMentionable, ReferenceOptionValue: contract.ReferenceOptionValue{Mentionable: &contract.MentionableRef{Kind: contract.MentionableRole, Role: &ref}}})
			}
		}
	case discord.ComponentTypeChannelSelectMenu:
		input.Kind = contract.ComponentChannelSelect
		data := e.ChannelSelectMenuInteractionData()
		guildID := SnowflakePtrToString(e.GuildID())
		for _, id := range data.Values {
			ref := ChannelRef(data.Resolved.Channels[id], guildID)
			input.Values = append(input.Values, contract.OptionValue{Kind: contract.OptionChannel, ReferenceOptionValue: contract.ReferenceOptionValue{Channel: &ref}})
		}
	}
	return input
}
func ModalInput(e *events.ModalSubmitInteractionCreate, pluginID, localID string) contract.ModalInput {
	input := contract.ModalInput{ID: localID}
	for component := range e.Data.AllComponents() {
		var id, value string
		switch field := component.(type) {
		case discord.TextInputComponent:
			id, value = field.CustomID, field.Value
		case *discord.TextInputComponent:
			if field != nil {
				id, value = field.CustomID, field.Value
			}
		}
		pid, fieldID, ok := customid.Parse(id)
		if ok && pid == pluginID {
			input.Fields = append(input.Fields, contract.NamedString{Name: fieldID, Value: value})
		}
	}
	return input
}
func UserRef(user discord.User) contract.UserRef {
	return contract.UserRef{ID: user.ID.String(), Username: strings.TrimSpace(user.Username), Name: strings.TrimSpace(user.EffectiveName()), AvatarURL: strings.TrimSpace(user.EffectiveAvatarURL()), Bot: user.Bot, System: user.System}
}
func MemberRef(member discord.ResolvedMember, guildID string) contract.MemberRef {
	roles := make([]string, len(member.RoleIDs))
	for i, id := range member.RoleIDs {
		roles[i] = id.String()
	}
	return contract.MemberRef{GuildID: guildID, User: UserRef(member.User), DisplayName: strings.TrimSpace(member.EffectiveName()), RoleIDs: roles, Permissions: ContractPermissions(member.Permissions)}
}
func GuildRef(guild discord.Guild) contract.GuildRef {
	return contract.GuildRef{ID: guild.ID.String(), Name: strings.TrimSpace(guild.Name)}
}
func ChannelRef(channel discord.ResolvedChannel, guildID string) contract.ChannelRef {
	return contract.ChannelRef{ID: channel.ID.String(), GuildID: guildID, Name: strings.TrimSpace(channel.Name), Kind: contractChannelKind(channel.Type), ParentID: channel.ParentID.String(), Mention: discord.ChannelMention(channel.ID), PermissionBits: uint64(channel.Permissions), CreatedAt: channel.ID.Time().UTC().Unix()}
}
func InteractionChannelRef(channel discord.InteractionChannel, guildID string) contract.ChannelRef {
	return contract.ChannelRef{ID: channel.ID().String(), GuildID: guildID, Name: strings.TrimSpace(channel.Name()), Kind: contractChannelKind(channel.Type()), Mention: discord.ChannelMention(channel.ID()), PermissionBits: uint64(channel.Permissions), CreatedAt: channel.ID().Time().UTC().Unix()}
}
func RoleRef(role discord.Role, guildID string) contract.RoleRef {
	return contract.RoleRef{RoleIdentity: contract.RoleIdentity{ID: role.ID.String(), GuildID: guildID, Name: role.Name}, RoleAuthority: contract.RoleAuthority{Position: role.Position, Permissions: ContractPermissions(role.Permissions)}, RolePresentation: contract.RolePresentation{Mention: discord.RoleMention(role.ID), Color: role.Color, Hoist: role.Hoist, Mentionable: role.Mentionable}, RoleManagement: contract.RoleManagement{Managed: role.Managed, PermissionBits: uint64(role.Permissions)}, CreatedAt: role.CreatedAt().UTC().Unix()}
}
func AttachmentRef(value discord.Attachment) contract.AttachmentRef {
	contentType := ""
	if value.ContentType != nil {
		contentType = *value.ContentType
	}
	width, height := 0, 0
	if value.Width != nil {
		width = *value.Width
	}
	if value.Height != nil {
		height = *value.Height
	}
	return contract.AttachmentRef{ID: value.ID.String(), Filename: value.Filename, ContentType: contentType, URL: value.URL, Size: int64(value.Size), Width: width, Height: height}
}
func ContractPermissions(value discord.Permissions) []contract.MemberPermission {
	mapping := []struct {
		bit   discord.Permissions
		value contract.MemberPermission
	}{{discord.PermissionAdministrator, contract.PermissionAdministrator}, {discord.PermissionManageGuild, contract.PermissionManageGuild}, {discord.PermissionManageRoles, contract.PermissionManageRoles}, {discord.PermissionManageGuildExpressions, contract.PermissionManageExpressions}, {discord.PermissionCreateGuildExpressions, contract.PermissionCreateExpressions}, {discord.PermissionManageMessages, contract.PermissionManageMessages}, {discord.PermissionManageNicknames, contract.PermissionManageNicknames}, {discord.PermissionManageChannels, contract.PermissionManageChannels}, {discord.PermissionKickMembers, contract.PermissionKickMembers}, {discord.PermissionBanMembers, contract.PermissionBanMembers}, {discord.PermissionModerateMembers, contract.PermissionModerateMembers}}
	out := []contract.MemberPermission{}
	for _, entry := range mapping {
		if value.Has(entry.bit) {
			out = append(out, entry.value)
		}
	}
	return out
}
func contractChannelKind(value discord.ChannelType) contract.ChannelKind {
	switch value {
	case discord.ChannelTypeGuildVoice:
		return contract.ChannelVoice
	case discord.ChannelTypeGuildCategory:
		return contract.ChannelCategory
	case discord.ChannelTypeGuildNews:
		return contract.ChannelAnnouncement
	case discord.ChannelTypeGuildStageVoice:
		return contract.ChannelStage
	case discord.ChannelTypeGuildForum:
		return contract.ChannelForum
	case discord.ChannelTypeGuildMedia:
		return contract.ChannelMedia
	default:
		return contract.ChannelText
	}
}
func OptionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}
