package pluginbridge

import (
	"context"
	"errors"
	"fmt"
	discordexecutor "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge/executor"
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/customid"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/omit"
	"github.com/disgoorg/snowflake/v2"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/host"
)

// StarlarkBridge is the sole Discord adapter exposed to the plugin host.
type StarlarkBridge struct{ Executor discordexecutor.Discord }

func (bridge StarlarkBridge) GetUser(ctx context.Context, id string) (contract.UserDetailsRef, bool, error) {
	userID, err := parseContractID(id)
	if err != nil {
		return contract.UserDetailsRef{}, false, err
	}
	if bridge.Executor.Client() == nil {
		return contract.UserDetailsRef{}, false, errors.New("discord client unavailable")
	}
	user, err := bridge.Executor.Client().Rest.GetUser(snowflake.ID(userID), rest.WithCtx(ctx))
	if err != nil || user == nil {
		return contract.UserDetailsRef{}, false, err
	}
	ref := contract.UserDetailsRef{User: routerUserRef(*user), Mention: user.Mention(), AvatarURL: strings.TrimSpace(user.EffectiveAvatarURL()), CreatedAt: user.CreatedAt().UTC().Unix()}
	if user.AccentColor != nil {
		color := *user.AccentColor
		ref.AccentColor = &color
	}
	if value := user.BannerURL(); value != nil {
		ref.BannerURL = strings.TrimSpace(*value)
	}
	return ref, true, nil
}
func (bridge StarlarkBridge) GetMember(ctx context.Context, guildIDText, userIDText string) (contract.MemberDetailsRef, bool, error) {
	guildID, err := parseContractID(guildIDText)
	if err != nil {
		return contract.MemberDetailsRef{}, false, err
	}
	userID, err := parseContractID(userIDText)
	if err != nil {
		return contract.MemberDetailsRef{}, false, err
	}
	if bridge.Executor.Client() == nil {
		return contract.MemberDetailsRef{}, false, errors.New("discord client unavailable")
	}
	member, err := bridge.Executor.Client().Rest.GetMember(snowflake.ID(guildID), snowflake.ID(userID), rest.WithCtx(ctx))
	if err != nil || member == nil {
		return contract.MemberDetailsRef{}, false, err
	}
	roles := make([]string, len(member.RoleIDs))
	for i, id := range member.RoleIDs {
		roles[i] = id.String()
	}
	ref := contract.MemberDetailsRef{Member: contract.MemberRef{GuildID: guildIDText, User: routerUserRef(member.User), DisplayName: strings.TrimSpace(member.EffectiveName()), RoleIDs: roles}, AvatarURL: strings.TrimSpace(member.EffectiveAvatarURL()), BannerURL: strings.TrimSpace(member.EffectiveBannerURL())}
	if member.JoinedAt != nil {
		ref.JoinedAt = member.JoinedAt.UTC().Unix()
	}
	return ref, true, nil
}
func (bridge StarlarkBridge) GetGuild(ctx context.Context, id string) (contract.GuildDetailsRef, bool, error) {
	guildID, err := parseContractID(id)
	if err != nil {
		return contract.GuildDetailsRef{}, false, err
	}
	if bridge.Executor.Client() == nil {
		return contract.GuildDetailsRef{}, false, errors.New("discord client unavailable")
	}
	guild, err := bridge.Executor.Client().Rest.GetGuild(snowflake.ID(guildID), true, rest.WithCtx(ctx))
	if err != nil || guild == nil {
		return contract.GuildDetailsRef{}, false, err
	}
	channels, err := bridge.Executor.Client().Rest.GetGuildChannels(snowflake.ID(guildID), rest.WithCtx(ctx))
	if err != nil {
		return contract.GuildDetailsRef{}, false, err
	}
	members := guild.ApproximateMemberCount
	if members <= 0 {
		members = guild.MemberCount
	}
	ref := contract.GuildDetailsRef{Guild: contract.GuildRef{ID: guild.ID.String(), Name: strings.TrimSpace(guild.Name)}, GuildProfile: contract.GuildProfile{OwnerID: guild.OwnerID.String()}, GuildResourceCounts: contract.GuildResourceCounts{RolesCount: len(guild.Roles), EmojisCount: len(guild.Emojis), StickersCount: len(guild.Stickers), MemberCount: members, ChannelsCount: len(channels)}, CreatedAt: guild.CreatedAt().UTC().Unix()}
	if guild.Description != nil {
		ref.Description = strings.TrimSpace(*guild.Description)
	}
	if value := guild.IconURL(); value != nil {
		ref.IconURL = strings.TrimSpace(*value)
	}
	if value := guild.BannerURL(); value != nil {
		ref.BannerURL = strings.TrimSpace(*value)
	}
	return ref, true, nil
}
func routerUserRef(user discord.User) contract.UserRef {
	return contract.UserRef{ID: user.ID.String(), Username: strings.TrimSpace(user.Username), Name: strings.TrimSpace(user.EffectiveName()), AvatarURL: strings.TrimSpace(user.EffectiveAvatarURL()), Bot: user.Bot, System: user.System}
}

func (bridge StarlarkBridge) Execute(ctx context.Context, scope pluginhost.EffectScope, operation contract.Operation) error {
	client := bridge.Executor.Client()
	if client == nil {
		return errors.New("discord client unavailable")
	}
	guildID, _ := parseOptionalContractID(scope.GuildID)
	channelID, _ := parseOptionalContractID(scope.ChannelID)
	switch value := operation.(type) {
	case *contract.SendChannelOperation:
		id, err := parseContractID(value.ChannelID)
		if err != nil {
			return err
		}
		message, err := ContractMessage(scope.PluginID, value.Message)
		if err != nil {
			return err
		}
		_, err = client.Rest.CreateMessage(snowflake.ID(id), message, rest.WithCtx(ctx))
		return err
	case *contract.SendDMOperation:
		id, err := parseContractID(value.UserID)
		if err != nil {
			return err
		}
		dm, err := bridge.Executor.EnsureDMChannel(ctx, id)
		if err != nil {
			return err
		}
		message, err := ContractMessage(scope.PluginID, value.Message)
		if err != nil {
			return err
		}
		_, err = client.Rest.CreateMessage(snowflake.ID(dm), message, rest.WithCtx(ctx))
		return err
	case *contract.TimeoutMemberOperation:
		id, err := parseContractID(value.UserID)
		if err != nil {
			return err
		}
		if guildID == 0 {
			return errors.New("timeout requires guild")
		}
		until := time.Unix(value.UntilUnix, 0).UTC()
		_, err = client.Rest.UpdateMember(snowflake.ID(guildID), snowflake.ID(id), discord.MemberUpdate{CommunicationDisabledUntil: omit.NewPtr(until)}, rest.WithCtx(ctx))
		return err
	case *contract.SetSlowmodeOperation:
		id, err := parseContractID(value.ChannelID)
		if err != nil {
			return err
		}
		seconds := value.Seconds
		_, err = client.Rest.UpdateChannel(snowflake.ID(id), discord.GuildTextChannelUpdate{RateLimitPerUser: &seconds}, rest.WithCtx(ctx))
		return err
	case *contract.SetNicknameOperation:
		id, err := parseContractID(value.UserID)
		if err != nil {
			return err
		}
		if guildID == 0 {
			return errors.New("nickname requires guild")
		}
		nickname := ""
		if value.Nickname.Set {
			nickname = value.Nickname.Value
		}
		_, err = client.Rest.UpdateMember(snowflake.ID(guildID), snowflake.ID(id), discord.MemberUpdate{Nick: &nickname}, rest.WithCtx(ctx))
		return err
	case *contract.PurgeMessagesOperation:
		id, err := parseContractID(value.ChannelID)
		if err != nil {
			return err
		}
		var around, before, after snowflake.ID
		if value.AnchorMessageID != "" {
			anchor, parseErr := parseContractID(value.AnchorMessageID)
			if parseErr != nil {
				return parseErr
			}
			switch value.Mode {
			case contract.PurgeAround:
				around = snowflake.ID(anchor)
			case contract.PurgeBefore:
				before = snowflake.ID(anchor)
			case contract.PurgeAfter:
				after = snowflake.ID(anchor)
			}
		}
		messages, err := client.Rest.GetMessages(snowflake.ID(id), around, before, after, value.Count, rest.WithCtx(ctx))
		if err != nil {
			return err
		}
		ids := make([]snowflake.ID, len(messages))
		for i, message := range messages {
			ids[i] = message.ID
		}
		_, err = bridge.Executor.DeleteMessages(ctx, snowflake.ID(id), ids, time.Now())
		return err
	case *contract.CreateRoleOperation:
		if guildID == 0 {
			return errors.New("role create requires guild")
		}
		input := discord.RoleCreate{Name: value.Name}
		if value.Color != nil {
			input.Color = *value.Color
		}
		if value.Hoist != nil {
			input.Hoist = *value.Hoist
		}
		if value.Mentionable != nil {
			input.Mentionable = *value.Mentionable
		}
		_, err := client.Rest.CreateRole(snowflake.ID(guildID), input, rest.WithCtx(ctx))
		return err
	case *contract.EditRoleOperation:
		if guildID == 0 {
			return errors.New("role edit requires guild")
		}
		id, err := parseContractID(value.RoleID)
		if err != nil {
			return err
		}
		_, err = client.Rest.UpdateRole(snowflake.ID(guildID), snowflake.ID(id), discord.RoleUpdate{Name: value.Name, Color: value.Color, Hoist: value.Hoist, Mentionable: value.Mentionable}, rest.WithCtx(ctx))
		return err
	case *contract.DeleteRoleOperation:
		if guildID == 0 {
			return errors.New("role delete requires guild")
		}
		id, err := parseContractID(value.RoleID)
		if err != nil {
			return err
		}
		return client.Rest.DeleteRole(snowflake.ID(guildID), snowflake.ID(id), rest.WithCtx(ctx))
	case *contract.MemberRoleOperation:
		if guildID == 0 {
			return errors.New("member role requires guild")
		}
		userID, err := parseContractID(value.UserID)
		if err != nil {
			return err
		}
		roleID, err := parseContractID(value.RoleID)
		if err != nil {
			return err
		}
		if value.Add {
			return client.Rest.AddMemberRole(snowflake.ID(guildID), snowflake.ID(userID), snowflake.ID(roleID), rest.WithCtx(ctx))
		}
		return client.Rest.RemoveMemberRole(snowflake.ID(guildID), snowflake.ID(userID), snowflake.ID(roleID), rest.WithCtx(ctx))
	case *contract.CreateEmojiOperation:
		return bridge.createEmoji(ctx, guildID, value, scope)
	case *contract.EditEmojiOperation:
		if guildID == 0 {
			return errors.New("emoji edit requires guild")
		}
		emojiID, ok := parseEmojiContractID(value.Emoji)
		if !ok {
			return errors.New("invalid emoji")
		}
		_, err := client.Rest.UpdateEmoji(snowflake.ID(guildID), emojiID, discord.EmojiUpdate{Name: &value.Name}, rest.WithCtx(ctx))
		return err
	case *contract.DeleteEmojiOperation:
		if guildID == 0 {
			return errors.New("emoji delete requires guild")
		}
		emojiID, ok := parseEmojiContractID(value.Emoji)
		if !ok {
			return errors.New("invalid emoji")
		}
		return client.Rest.DeleteEmoji(snowflake.ID(guildID), emojiID, rest.WithCtx(ctx))
	case *contract.CreateStickerOperation:
		return bridge.createSticker(ctx, guildID, value, scope)
	case *contract.EditStickerOperation:
		if guildID == 0 {
			return errors.New("sticker edit requires guild")
		}
		id, err := parseContractID(value.StickerID)
		if err != nil {
			return err
		}
		update := discord.StickerUpdate{Name: value.Name, Description: value.Description, Tags: value.EmojiTag}
		_, err = client.Rest.UpdateSticker(snowflake.ID(guildID), snowflake.ID(id), update, rest.WithCtx(ctx))
		return err
	case *contract.DeleteStickerOperation:
		if guildID == 0 {
			return errors.New("sticker delete requires guild")
		}
		id, err := parseContractID(value.StickerID)
		if err != nil {
			return err
		}
		return client.Rest.DeleteSticker(snowflake.ID(guildID), snowflake.ID(id), rest.WithCtx(ctx))
	default:
		_ = channelID
		return fmt.Errorf("unsupported Discord plugin operation %T", operation)
	}
}

func (bridge StarlarkBridge) createEmoji(ctx context.Context, guildID uint64, value *contract.CreateEmojiOperation, scope pluginhost.EffectScope) error {
	if guildID == 0 {
		return errors.New("emoji create requires guild")
	}
	attachment, err := scopeAttachment(scope, value.AttachmentID)
	if err != nil {
		return err
	}
	body, err := fetchContractAttachment(ctx, attachment, discordexecutor.EmojiMaxFileBytes)
	if err != nil {
		return err
	}
	_, err = bridge.Executor.CreateEmojiUpload(ctx, guildID, value.Name, attachment.Filename, body, attachment.Width, attachment.Height)
	return err
}
func (bridge StarlarkBridge) createSticker(ctx context.Context, guildID uint64, value *contract.CreateStickerOperation, scope pluginhost.EffectScope) error {
	if guildID == 0 {
		return errors.New("sticker create requires guild")
	}
	attachment, err := scopeAttachment(scope, value.AttachmentID)
	if err != nil {
		return err
	}
	body, err := fetchContractAttachment(ctx, attachment, discordexecutor.StickerMaxFileBytes)
	if err != nil {
		return err
	}
	_, err = bridge.Executor.CreateStickerUpload(ctx, guildID, value.Name, value.Description, value.EmojiTag, attachment.Filename, body, attachment.Width, attachment.Height)
	return err
}

// ContractMessage converts a validated neutral message at the Discord boundary.
func ContractMessage(pluginID string, message contract.Message) (discord.MessageCreate, error) {
	embeds := make([]discord.Embed, len(message.Embeds))
	for i, value := range message.Embeds {
		embed := discord.Embed{Title: value.Title, Description: value.Description, URL: value.URL, Color: value.Color}
		for _, field := range value.Fields {
			inline := field.Inline
			embed.Fields = append(embed.Fields, discord.EmbedField{Name: field.Name, Value: field.Value, Inline: &inline})
		}
		if value.Author != nil {
			embed.Author = &discord.EmbedAuthor{Name: value.Author.Name, URL: value.Author.URL, IconURL: value.Author.IconURL}
		}
		if value.Footer != nil {
			embed.Footer = &discord.EmbedFooter{Text: value.Footer.Text, IconURL: value.Footer.IconURL}
		}
		if value.ImageURL != "" {
			embed.Image = &discord.EmbedResource{URL: value.ImageURL}
		}
		if value.ThumbnailURL != "" {
			embed.Thumbnail = &discord.EmbedResource{URL: value.ThumbnailURL}
		}
		embeds[i] = embed
	}
	components, err := contractComponents(pluginID, message.Components)
	if err != nil {
		return discord.MessageCreate{}, err
	}
	return discord.MessageCreate{Content: message.Content, Embeds: embeds, Components: components, AllowedMentions: &discord.AllowedMentions{}}, nil
}
func contractComponents(pluginID string, rows []contract.ComponentRow) ([]discord.LayoutComponent, error) {
	out := make([]discord.LayoutComponent, 0, len(rows))
	for _, row := range rows {
		items := make([]discord.InteractiveComponent, 0, len(row.Components))
		for _, value := range row.Components {
			switch item := value.(type) {
			case *contract.Button:
				button, err := contractButton(pluginID, item)
				if err != nil {
					return nil, err
				}
				items = append(items, button)
			case *contract.Select:
				menu, err := contractSelect(pluginID, item)
				if err != nil {
					return nil, err
				}
				items = append(items, menu)
			default:
				return nil, fmt.Errorf("unsupported component %T", value)
			}
		}
		out = append(out, discord.NewActionRow(items...))
	}
	return out, nil
}
func contractButton(pluginID string, value *contract.Button) (discord.ButtonComponent, error) {
	style := map[contract.ButtonStyle]discord.ButtonStyle{contract.ButtonPrimary: discord.ButtonStylePrimary, contract.ButtonSecondary: discord.ButtonStyleSecondary, contract.ButtonSuccess: discord.ButtonStyleSuccess, contract.ButtonDanger: discord.ButtonStyleDanger, contract.ButtonLink: discord.ButtonStyleLink}[value.Style]
	button := discord.ButtonComponent{Style: style, Label: value.Label, URL: value.URL, Disabled: value.Disabled}
	if value.Style != contract.ButtonLink {
		id, err := customid.Build(pluginID, value.Handler)
		if err != nil {
			return button, err
		}
		button.CustomID = id
	}
	if value.Emoji != nil {
		emoji, err := contractEmoji(*value.Emoji)
		if err != nil {
			return button, err
		}
		button.Emoji = &emoji
	}
	return button, nil
}
func contractSelect(pluginID string, value *contract.Select) (discord.InteractiveComponent, error) {
	id, err := customid.Build(pluginID, value.Handler)
	if err != nil {
		return nil, err
	}
	min := value.MinValues
	switch value.Kind {
	case contract.SelectString:
		options := make([]discord.StringSelectMenuOption, len(value.Options))
		for i, item := range value.Options {
			options[i] = discord.StringSelectMenuOption{Label: item.Label, Value: item.Value, Description: item.Description, Default: item.Default}
			if item.Emoji != nil {
				emoji, e := contractEmoji(*item.Emoji)
				if e != nil {
					return nil, e
				}
				options[i].Emoji = &emoji
			}
		}
		return discord.StringSelectMenuComponent{CustomID: id, Placeholder: value.Placeholder, MinValues: &min, MaxValues: value.MaxValues, Disabled: value.Disabled, Options: options}, nil
	case contract.SelectUser:
		return discord.UserSelectMenuComponent{CustomID: id, Placeholder: value.Placeholder, MinValues: &min, MaxValues: value.MaxValues, Disabled: value.Disabled}, nil
	case contract.SelectRole:
		return discord.RoleSelectMenuComponent{CustomID: id, Placeholder: value.Placeholder, MinValues: &min, MaxValues: value.MaxValues, Disabled: value.Disabled}, nil
	case contract.SelectMentionable:
		return discord.MentionableSelectMenuComponent{CustomID: id, Placeholder: value.Placeholder, MinValues: &min, MaxValues: value.MaxValues, Disabled: value.Disabled}, nil
	case contract.SelectChannel:
		kinds := make([]discord.ChannelType, len(value.ChannelKinds))
		for i, kind := range value.ChannelKinds {
			kinds[i] = contractChannelType(kind)
		}
		return discord.ChannelSelectMenuComponent{CustomID: id, Placeholder: value.Placeholder, MinValues: &min, MaxValues: value.MaxValues, Disabled: value.Disabled, ChannelTypes: kinds}, nil
	default:
		return nil, errors.New("unsupported select kind")
	}
}
func contractEmoji(value contract.Emoji) (discord.ComponentEmoji, error) {
	emoji := discord.ComponentEmoji{Name: value.Name, Animated: value.Animated}
	if value.ID != "" {
		id, err := parseContractID(value.ID)
		if err != nil {
			return emoji, err
		}
		emoji.ID = snowflake.ID(id)
	}
	return emoji, nil
}
func contractChannelType(kind contract.ChannelKind) discord.ChannelType {
	switch kind {
	case contract.ChannelText:
		return discord.ChannelTypeGuildText
	case contract.ChannelVoice:
		return discord.ChannelTypeGuildVoice
	case contract.ChannelCategory:
		return discord.ChannelTypeGuildCategory
	case contract.ChannelAnnouncement:
		return discord.ChannelTypeGuildNews
	case contract.ChannelStage:
		return discord.ChannelTypeGuildStageVoice
	case contract.ChannelForum:
		return discord.ChannelTypeGuildForum
	case contract.ChannelMedia:
		return discord.ChannelTypeGuildMedia
	default:
		return discord.ChannelTypeGuildText
	}
}
func parseContractID(raw string) (uint64, error) {
	id, err := snowflake.Parse(strings.TrimSpace(raw))
	if err != nil || id == 0 {
		return 0, errors.New("invalid Discord id")
	}
	return uint64(id), nil
}
func parseOptionalContractID(raw string) (uint64, error) {
	if strings.TrimSpace(raw) == "" {
		return 0, nil
	}
	return parseContractID(raw)
}
func parseEmojiContractID(raw string) (snowflake.ID, bool) {
	raw = strings.TrimSpace(raw)
	if id, err := snowflake.Parse(raw); err == nil && id != 0 {
		return id, true
	}
	parts := strings.Split(strings.Trim(raw, "<>"), ":")
	if len(parts) == 3 {
		if id, err := snowflake.Parse(parts[2]); err == nil && id != 0 {
			return id, true
		}
	}
	return 0, false
}

func scopeAttachment(scope pluginhost.EffectScope, id string) (contract.AttachmentRef, error) {
	for _, attachment := range scope.Attachments {
		if attachment.ID == id {
			return attachment, nil
		}
	}
	return contract.AttachmentRef{}, errors.New("attachment is not part of the invocation")
}
func fetchContractAttachment(ctx context.Context, attachment contract.AttachmentRef, limit int64) ([]byte, error) {
	if attachment.Size > limit {
		return nil, errors.New("attachment exceeds byte limit")
	}
	return newDiscordCDNFetcher().Fetch(ctx, attachment.URL, limit)
}

func ContractMessageCreate(pluginID string, value contract.MessageOperation) (discord.MessageCreate, error) {
	message, err := ContractMessage(pluginID, value.Message)
	if value.Ephemeral {
		message.Flags = discord.MessageFlagEphemeral
	}
	return message, err
}
func ContractMessageUpdate(pluginID string, patch contract.MessagePatch) (discord.MessageUpdate, error) {
	update := discord.MessageUpdate{AllowedMentions: &discord.AllowedMentions{}}
	if patch.Content.Set {
		value := patch.Content.Value
		update.Content = &value
	}
	if patch.Embeds.Set {
		message, err := ContractMessage(pluginID, contract.Message{Embeds: patch.Embeds.Values})
		if err != nil {
			return update, err
		}
		update.Embeds = &message.Embeds
	}
	if patch.Components.Set {
		items, err := contractComponents(pluginID, patch.Components.Values)
		if err != nil {
			return update, err
		}
		update.Components = &items
	}
	return update, nil
}
func ContractModal(pluginID string, view contract.ModalView) (discord.ModalCreate, error) {
	id, err := customid.Build(pluginID, view.Handler)
	if err != nil {
		return discord.ModalCreate{}, err
	}
	rows := make([]discord.LayoutComponent, 0, len(view.Fields))
	for _, field := range view.Fields {
		fieldID, e := customid.Build(pluginID, field.ID)
		if e != nil {
			return discord.ModalCreate{}, e
		}
		style := discord.TextInputStyleShort
		if field.Style == contract.TextInputParagraph {
			style = discord.TextInputStyleParagraph
		}
		input := discord.TextInputComponent{CustomID: fieldID, Style: style, Placeholder: field.Placeholder, Value: field.Value, Required: field.Required, MinLength: &field.MinLength, MaxLength: field.MaxLength}
		rows = append(rows, discord.NewLabel(field.Label, input))
	}
	return discord.NewModalCreate(id, view.Title, rows...), nil
}
func ContractAutocompleteChoices(values []contract.AutocompleteChoice) ([]discord.AutocompleteChoice, error) {
	out := make([]discord.AutocompleteChoice, 0, len(values))
	for _, value := range values {
		switch value.Value.Kind {
		case contract.ChoiceString:
			out = append(out, discord.AutocompleteChoiceString{Name: value.Name, Value: value.Value.String})
		case contract.ChoiceInteger:
			out = append(out, discord.AutocompleteChoiceInt{Name: value.Name, Value: int(value.Value.Integer)})
		case contract.ChoiceNumber:
			out = append(out, discord.AutocompleteChoiceFloat{Name: value.Name, Value: value.Value.Number})
		default:
			return nil, errors.New("unsupported autocomplete choice")
		}
	}
	return out, nil
}
