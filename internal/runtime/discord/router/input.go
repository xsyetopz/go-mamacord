package router

import (
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"

	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

func SnowflakePtrToString(id *snowflake.ID) string {
	if id == nil {
		return ""
	}
	return id.String()
}

func PluginOptions(data discord.SlashCommandInteractionData) map[string]any {
	opts := map[string]any{}

	if data.SubCommandGroupName != nil {
		name := strings.TrimSpace(*data.SubCommandGroupName)
		if name != "" {
			opts["__group"] = name
		}
	}
	if data.SubCommandName != nil {
		name := strings.TrimSpace(*data.SubCommandName)
		if name != "" {
			opts["__subcommand"] = name
		}
	}

	for _, opt := range data.All() {
		name := strings.TrimSpace(opt.Name)
		if name == "" {
			continue
		}

		if opt.Type == discord.ApplicationCommandOptionTypeString {
			opts[name] = opt.String()
			continue
		}
		if opt.Type == discord.ApplicationCommandOptionTypeInt {
			opts[name] = opt.Int()
			continue
		}
		if opt.Type == discord.ApplicationCommandOptionTypeBool {
			opts[name] = opt.Bool()
			continue
		}
		if opt.Type == discord.ApplicationCommandOptionTypeFloat {
			opts[name] = opt.Float()
			continue
		}
		if opt.Type == discord.ApplicationCommandOptionTypeUser ||
			opt.Type == discord.ApplicationCommandOptionTypeMentionable {
			opts[name] = opt.Snowflake().String()
			if opt.Type == discord.ApplicationCommandOptionTypeUser {
				user := data.User(name)
				opts["__resolved:"+name] = map[string]any{
					"id":      user.ID.String(),
					"bot":     user.Bot,
					"system":  user.System,
					"mention": user.Mention(),
				}
			}
			continue
		}
		if opt.Type == discord.ApplicationCommandOptionTypeChannel {
			opts[name] = opt.Snowflake().String()
			channel := data.Channel(name)
			resolved := map[string]any{
				"id":          channel.ID.String(),
				"name":        channel.Name,
				"mention":     discord.ChannelMention(channel.ID),
				"type":        channelTypeName(channel.Type),
				"permissions": channel.Permissions.String(),
				"created_at":  channel.ID.Time().UTC().Unix(),
			}
			if channel.ParentID != 0 {
				resolved["parent_id"] = channel.ParentID.String()
			}
			opts["__resolved:"+name] = resolved
			continue
		}
		if opt.Type == discord.ApplicationCommandOptionTypeRole {
			opts[name] = opt.Snowflake().String()
			role := data.Role(name)
			opts["__resolved:"+name] = map[string]any{
				"id":          role.ID.String(),
				"name":        role.Name,
				"mention":     discord.RoleMention(role.ID),
				"color":       role.Color,
				"hoist":       role.Hoist,
				"mentionable": role.Mentionable,
				"managed":     role.Managed,
				"position":    role.Position,
				"permissions": role.Permissions.String(),
				"created_at":  role.CreatedAt().UTC().Unix(),
			}
			continue
		}
		if opt.Type == discord.ApplicationCommandOptionTypeAttachment {
			attachment := data.Attachment(name)
			opts[name] = attachment.ID.String()
			resolved := map[string]any{
				"id":       attachment.ID.String(),
				"filename": attachment.Filename,
				"url":      strings.TrimSpace(attachment.URL),
				"size":     attachment.Size,
			}
			if attachment.Width != nil {
				resolved["width"] = *attachment.Width
			}
			if attachment.Height != nil {
				resolved["height"] = *attachment.Height
			}
			if attachment.ContentType != nil && strings.TrimSpace(*attachment.ContentType) != "" {
				resolved["content_type"] = strings.TrimSpace(*attachment.ContentType)
			}
			opts["__resolved:"+name] = resolved
			continue
		}
	}

	return opts
}

func PluginAutocompleteOptions(data discord.AutocompleteInteractionData) map[string]any {
	opts := map[string]any{}

	if data.SubCommandGroupName != nil {
		name := strings.TrimSpace(*data.SubCommandGroupName)
		if name != "" {
			opts["__group"] = name
		}
	}
	if data.SubCommandName != nil {
		name := strings.TrimSpace(*data.SubCommandName)
		if name != "" {
			opts["__subcommand"] = name
		}
	}
	opts["__command"] = strings.TrimSpace(data.CommandName)

	for _, opt := range data.All() {
		name := strings.TrimSpace(opt.Name)
		if name == "" {
			continue
		}

		value := autocompleteOptionValue(opt)
		opts[name] = value
		if opt.Focused {
			opts["__option"] = name
			opts["__value"] = value
		}
	}

	return opts
}

func autocompleteOptionValue(opt discord.AutocompleteOption) any {
	switch opt.Type {
	case discord.ApplicationCommandOptionTypeString:
		return opt.String()
	case discord.ApplicationCommandOptionTypeInt:
		return opt.Int()
	case discord.ApplicationCommandOptionTypeFloat:
		return opt.Float()
	case discord.ApplicationCommandOptionTypeBool:
		return opt.Bool()
	case discord.ApplicationCommandOptionTypeUser,
		discord.ApplicationCommandOptionTypeChannel,
		discord.ApplicationCommandOptionTypeRole,
		discord.ApplicationCommandOptionTypeMentionable:
		return opt.Snowflake().String()
	default:
		return nil
	}
}

func PluginUserContextOptions(data discord.UserCommandInteractionData) map[string]any {
	opts := map[string]any{}

	user := data.TargetUser()
	if user.ID != 0 {
		opts["__target_user"] = map[string]any{
			"id":           user.ID.String(),
			"username":     strings.TrimSpace(user.Username),
			"display_name": strings.TrimSpace(user.EffectiveName()),
			"mention":      user.Mention(),
			"bot":          user.Bot,
			"system":       user.System,
			"created_at":   user.CreatedAt().UTC().Unix(),
		}
	}

	member := data.TargetMember()
	if member.User.ID != 0 {
		roleIDs := make([]any, 0, len(member.RoleIDs))
		for _, roleID := range member.RoleIDs {
			roleIDs = append(roleIDs, roleID.String())
		}
		target := map[string]any{
			"user_id":  member.User.ID.String(),
			"guild_id": SnowflakePtrToString(data.GuildID()),
			"role_ids": roleIDs,
		}
		if member.JoinedAt != nil && !member.JoinedAt.IsZero() {
			target["joined_at"] = member.JoinedAt.UTC().Unix()
		}
		if avatar := strings.TrimSpace(member.EffectiveAvatarURL()); avatar != "" {
			target["avatar_url"] = avatar
		}
		if banner := strings.TrimSpace(member.EffectiveBannerURL()); banner != "" {
			target["banner_url"] = banner
		}
		opts["__target_member"] = target
	}

	return opts
}

func PluginMessageContextOptions(data discord.MessageCommandInteractionData) map[string]any {
	opts := map[string]any{}

	message := data.TargetMessage()
	if message.ID != 0 {
		target := map[string]any{
			"id":         message.ID.String(),
			"channel_id": message.ChannelID.String(),
			"author_id":  message.Author.ID.String(),
			"content":    message.Content,
			"created_at": message.CreatedAt.UTC().Unix(),
			"pinned":     message.Pinned,
		}
		if message.GuildID != nil {
			target["guild_id"] = message.GuildID.String()
		}
		if message.EditedTimestamp != nil && !message.EditedTimestamp.IsZero() {
			target["edited_at"] = message.EditedTimestamp.UTC().Unix()
		}
		opts["__target_message"] = target
	}

	return opts
}

func ComponentOptions(e *events.ComponentInteractionCreate) map[string]any {
	opts := map[string]any{}

	if e.Data.Type() == discord.ComponentTypeButton {
		opts["type"] = "button"
		return opts
	}
	if e.Data.Type() == discord.ComponentTypeStringSelectMenu {
		opts["type"] = "string_select"
		data := e.StringSelectMenuInteractionData()
		vals := make([]any, 0, len(data.Values))
		for _, v := range data.Values {
			vals = append(vals, v)
		}
		opts["values"] = vals
		return opts
	}
	if e.Data.Type() == discord.ComponentTypeUserSelectMenu {
		opts["type"] = "user_select"
		data := e.UserSelectMenuInteractionData()
		vals := make([]any, 0, len(data.Values))
		for _, v := range data.Values {
			vals = append(vals, v.String())
		}
		opts["values"] = vals
		return opts
	}
	if e.Data.Type() == discord.ComponentTypeRoleSelectMenu {
		opts["type"] = "role_select"
		data := e.RoleSelectMenuInteractionData()
		vals := make([]any, 0, len(data.Values))
		for _, v := range data.Values {
			vals = append(vals, v.String())
		}
		opts["values"] = vals
		return opts
	}
	if e.Data.Type() == discord.ComponentTypeMentionableSelectMenu {
		opts["type"] = "mentionable_select"
		data := e.MentionableSelectMenuInteractionData()
		vals := make([]any, 0, len(data.Values))
		for _, v := range data.Values {
			vals = append(vals, v.String())
		}
		opts["values"] = vals
		return opts
	}
	if e.Data.Type() == discord.ComponentTypeChannelSelectMenu {
		opts["type"] = "channel_select"
		data := e.ChannelSelectMenuInteractionData()
		vals := make([]any, 0, len(data.Values))
		for _, v := range data.Values {
			vals = append(vals, v.String())
		}
		opts["values"] = vals
		return opts
	}

	return opts
}

func ModalOptions(e *events.ModalSubmitInteractionCreate, pluginID string) map[string]any {
	opts := map[string]any{}

	fields := map[string]any{}
	for component := range e.Data.AllComponents() {
		var customID, value string
		switch ti := component.(type) {
		case discord.TextInputComponent:
			customID = ti.CustomID
			value = ti.Value
		case *discord.TextInputComponent:
			if ti == nil {
				continue
			}
			customID = ti.CustomID
			value = ti.Value
		default:
			continue
		}

		cid := strings.TrimSpace(customID)
		if cid == "" {
			continue
		}
		pid, localID, ok := pluginhost.ParseCustomID(cid)
		if !ok || pid != pluginID {
			continue
		}
		fields[localID] = value
	}
	opts["fields"] = fields
	return opts
}

func OptionalString(value *string) string {
	if value == nil {
		return ""
	}
	return strings.TrimSpace(*value)
}

func channelTypeName(t discord.ChannelType) string {
	switch t {
	case discord.ChannelTypeGuildText:
		return "guild_text"
	case discord.ChannelTypeDM:
		return "dm"
	case discord.ChannelTypeGuildVoice:
		return "guild_voice"
	case discord.ChannelTypeGroupDM:
		return "group_dm"
	case discord.ChannelTypeGuildCategory:
		return "guild_category"
	case discord.ChannelTypeGuildNews:
		return "guild_news"
	case discord.ChannelTypeGuildNewsThread:
		return "guild_news_thread"
	case discord.ChannelTypeGuildPublicThread:
		return "guild_public_thread"
	case discord.ChannelTypeGuildPrivateThread:
		return "guild_private_thread"
	case discord.ChannelTypeGuildStageVoice:
		return "guild_stage_voice"
	case discord.ChannelTypeGuildDirectory:
		return "guild_directory"
	case discord.ChannelTypeGuildForum:
		return "guild_forum"
	case discord.ChannelTypeGuildMedia:
		return "guild_media"
	default:
		return "unknown"
	}
}
