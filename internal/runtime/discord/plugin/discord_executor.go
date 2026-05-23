package plugin

import (
	"bytes"
	"context"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"

	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/cdn"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/parse"
	pluginhostlua "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

const (
	pluginEmojiMaxFileBytes   = 256 * 1024
	pluginEmojiMaxDimension   = 320
	pluginStickerMaxFileBytes = 512 * 1024
	pluginStickerMaxDimension = 320
)

func (e Executor) SetSlowmode(ctx context.Context, channelID uint64, seconds int) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if channelID == 0 || seconds < 0 {
		return errors.New("invalid slowmode spec")
	}

	updSeconds := seconds
	_, err := e.client().Rest.UpdateChannel(snowflake.ID(channelID), discord.GuildTextChannelUpdate{
		RateLimitPerUser: &updSeconds,
	}, rest.WithCtx(ctx))
	return err
}

func (e Executor) SetNickname(ctx context.Context, guildID, userID uint64, nickname *string) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if guildID == 0 || userID == 0 {
		return errors.New("invalid nickname spec")
	}

	nick := ""
	if nickname != nil {
		nick = strings.TrimSpace(*nickname)
	}
	_, err := e.client().Rest.UpdateMember(snowflake.ID(guildID), snowflake.ID(userID), discord.MemberUpdate{
		Nick: &nick,
	}, rest.WithCtx(ctx))
	return err
}

func (e Executor) CreateRole(
	ctx context.Context,
	spec pluginhostlua.RoleCreateSpec,
) (pluginhostlua.RoleResult, error) {
	if e.client() == nil {
		return pluginhostlua.RoleResult{}, errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || strings.TrimSpace(spec.Name) == "" {
		return pluginhostlua.RoleResult{}, errors.New("invalid role spec")
	}

	input := discord.RoleCreate{Name: strings.TrimSpace(spec.Name)}
	if spec.Color != nil {
		input.Color = *spec.Color
	}
	if spec.Hoist != nil {
		input.Hoist = *spec.Hoist
	}
	if spec.Mentionable != nil {
		input.Mentionable = *spec.Mentionable
	}

	role, err := e.client().Rest.CreateRole(snowflake.ID(spec.GuildID), input, rest.WithCtx(ctx))
	if err != nil {
		return pluginhostlua.RoleResult{}, err
	}
	return roleResult(*role), nil
}

func (e Executor) EditRole(
	ctx context.Context,
	spec pluginhostlua.RoleEditSpec,
) (pluginhostlua.RoleResult, error) {
	if e.client() == nil {
		return pluginhostlua.RoleResult{}, errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || spec.RoleID == 0 {
		return pluginhostlua.RoleResult{}, errors.New("invalid role spec")
	}
	if spec.RoleID == spec.GuildID {
		return pluginhostlua.RoleResult{}, errors.New("cannot_edit_everyone")
	}

	input := discord.RoleUpdate{}
	if spec.Name != nil && strings.TrimSpace(*spec.Name) != "" {
		name := strings.TrimSpace(*spec.Name)
		input.Name = &name
	}
	if spec.Color != nil {
		input.Color = spec.Color
	}
	if spec.Hoist != nil {
		input.Hoist = spec.Hoist
	}
	if spec.Mentionable != nil {
		input.Mentionable = spec.Mentionable
	}

	role, err := e.client().Rest.UpdateRole(
		snowflake.ID(spec.GuildID),
		snowflake.ID(spec.RoleID),
		input,
		rest.WithCtx(ctx),
	)
	if err != nil {
		return pluginhostlua.RoleResult{}, err
	}
	return roleResult(*role), nil
}

func (e Executor) DeleteRole(ctx context.Context, guildID, roleID uint64) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if guildID == 0 || roleID == 0 {
		return errors.New("invalid role spec")
	}
	if guildID == roleID {
		return errors.New("cannot_delete_everyone")
	}
	return e.client().Rest.DeleteRole(snowflake.ID(guildID), snowflake.ID(roleID), rest.WithCtx(ctx))
}

func (e Executor) AddRole(ctx context.Context, spec pluginhostlua.RoleMemberSpec) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || spec.UserID == 0 || spec.RoleID == 0 {
		return errors.New("invalid role member spec")
	}
	return e.client().Rest.AddMemberRole(
		snowflake.ID(spec.GuildID),
		snowflake.ID(spec.UserID),
		snowflake.ID(spec.RoleID),
		rest.WithCtx(ctx),
	)
}

func (e Executor) RemoveRole(ctx context.Context, spec pluginhostlua.RoleMemberSpec) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || spec.UserID == 0 || spec.RoleID == 0 {
		return errors.New("invalid role member spec")
	}
	return e.client().Rest.RemoveMemberRole(
		snowflake.ID(spec.GuildID),
		snowflake.ID(spec.UserID),
		snowflake.ID(spec.RoleID),
		rest.WithCtx(ctx),
	)
}

func (e Executor) ListMessages(
	ctx context.Context,
	spec pluginhostlua.MessageListSpec,
) ([]pluginhostlua.MessageInfo, error) {
	if e.client() == nil {
		return nil, errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.Limit <= 0 {
		return nil, errors.New("invalid list_messages spec")
	}

	messages, err := e.client().Rest.GetMessages(
		snowflake.ID(spec.ChannelID),
		snowflake.ID(spec.AroundID),
		snowflake.ID(spec.BeforeID),
		snowflake.ID(spec.AfterID),
		spec.Limit,
		rest.WithCtx(ctx),
	)
	if err != nil {
		return nil, err
	}

	out := make([]pluginhostlua.MessageInfo, 0, len(messages))
	for _, message := range messages {
		out = append(out, messageInfo(message))
	}
	return out, nil
}

func (e Executor) DeleteMessage(ctx context.Context, spec pluginhostlua.MessageDeleteSpec) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.MessageID == 0 {
		return errors.New("invalid delete_message spec")
	}
	return e.client().Rest.DeleteMessage(
		snowflake.ID(spec.ChannelID),
		snowflake.ID(spec.MessageID),
		rest.WithCtx(ctx),
	)
}

func (e Executor) BulkDeleteMessages(
	ctx context.Context,
	channelID uint64,
	messageIDs []uint64,
) (int, error) {
	if e.client() == nil {
		return 0, errors.New("discord client unavailable")
	}
	if channelID == 0 || len(messageIDs) == 0 {
		return 0, errors.New("invalid bulk_delete_messages spec")
	}

	ids := make([]snowflake.ID, 0, len(messageIDs))
	for _, id := range messageIDs {
		if id != 0 {
			ids = append(ids, snowflake.ID(id))
		}
	}
	if len(ids) == 0 {
		return 0, errors.New("invalid bulk_delete_messages spec")
	}

	if err := e.client().Rest.BulkDeleteMessages(snowflake.ID(channelID), ids, rest.WithCtx(ctx)); err != nil {
		return 0, err
	}
	return len(ids), nil
}

func (e Executor) PurgeMessages(ctx context.Context, spec pluginhostlua.PurgeSpec) (int, error) {
	if e.client() == nil {
		return 0, errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.Count <= 0 {
		return 0, errors.New("invalid purge_messages spec")
	}

	around, before, after, ok := purgeAnchorIDs(spec.Mode, spec.AnchorRaw)
	if !ok {
		return 0, errors.New("invalid_message")
	}

	messages, err := e.client().Rest.GetMessages(
		snowflake.ID(spec.ChannelID),
		around,
		before,
		after,
		spec.Count,
		rest.WithCtx(ctx),
	)
	if err != nil {
		return 0, err
	}

	ids := make([]snowflake.ID, 0, len(messages))
	for _, message := range messages {
		ids = append(ids, message.ID)
	}
	return deleteMessagesBestEffort(e.client().Rest, snowflake.ID(spec.ChannelID), ids, time.Now()), nil
}

func (e Executor) CrosspostMessage(
	ctx context.Context,
	spec pluginhostlua.MessageGetSpec,
) (pluginhostlua.MessageInfo, error) {
	if e.client() == nil {
		return pluginhostlua.MessageInfo{}, errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.MessageID == 0 {
		return pluginhostlua.MessageInfo{}, errors.New("invalid crosspost_message spec")
	}

	message, err := e.client().Rest.CrosspostMessage(
		snowflake.ID(spec.ChannelID),
		snowflake.ID(spec.MessageID),
		rest.WithCtx(ctx),
	)
	if err != nil || message == nil {
		return pluginhostlua.MessageInfo{}, errors.New("crosspost_message_error")
	}
	return messageInfo(*message), nil
}

func (e Executor) PinMessage(ctx context.Context, spec pluginhostlua.MessageGetSpec) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.MessageID == 0 {
		return errors.New("invalid pin_message spec")
	}
	return e.client().Rest.PinMessage(
		snowflake.ID(spec.ChannelID),
		snowflake.ID(spec.MessageID),
		rest.WithCtx(ctx),
	)
}

func (e Executor) UnpinMessage(ctx context.Context, spec pluginhostlua.MessageGetSpec) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.MessageID == 0 {
		return errors.New("invalid unpin_message spec")
	}
	return e.client().Rest.UnpinMessage(
		snowflake.ID(spec.ChannelID),
		snowflake.ID(spec.MessageID),
		rest.WithCtx(ctx),
	)
}

func (e Executor) GetReactions(
	ctx context.Context,
	spec pluginhostlua.ReactionListSpec,
) ([]pluginhostlua.UserResult, error) {
	if e.client() == nil {
		return nil, errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.MessageID == 0 || strings.TrimSpace(spec.Emoji) == "" || spec.Limit <= 0 {
		return nil, errors.New("invalid get_reactions spec")
	}

	users, err := e.client().Rest.GetReactions(
		snowflake.ID(spec.ChannelID),
		snowflake.ID(spec.MessageID),
		strings.TrimSpace(spec.Emoji),
		discord.MessageReactionTypeNormal,
		int(spec.AfterID),
		spec.Limit,
		rest.WithCtx(ctx),
	)
	if err != nil {
		return nil, err
	}

	out := make([]pluginhostlua.UserResult, 0, len(users))
	for _, user := range users {
		out = append(out, userResult(user))
	}
	return out, nil
}

func (e Executor) AddReaction(ctx context.Context, spec pluginhostlua.ReactionSpec) error {
	return e.runReactionMutation(ctx, spec, "invalid add_reaction spec", func() error {
		return e.client().Rest.AddReaction(
			snowflake.ID(spec.ChannelID),
			snowflake.ID(spec.MessageID),
			strings.TrimSpace(spec.Emoji),
			rest.WithCtx(ctx),
		)
	})
}

func (e Executor) RemoveOwnReaction(ctx context.Context, spec pluginhostlua.ReactionSpec) error {
	return e.runReactionMutation(ctx, spec, "invalid remove_own_reaction spec", func() error {
		return e.client().Rest.RemoveOwnReaction(
			snowflake.ID(spec.ChannelID),
			snowflake.ID(spec.MessageID),
			strings.TrimSpace(spec.Emoji),
			rest.WithCtx(ctx),
		)
	})
}

func (e Executor) RemoveUserReaction(ctx context.Context, spec pluginhostlua.ReactionUserSpec) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.MessageID == 0 || spec.UserID == 0 || strings.TrimSpace(spec.Emoji) == "" {
		return errors.New("invalid remove_user_reaction spec")
	}
	return e.client().Rest.RemoveUserReaction(
		snowflake.ID(spec.ChannelID),
		snowflake.ID(spec.MessageID),
		strings.TrimSpace(spec.Emoji),
		snowflake.ID(spec.UserID),
		rest.WithCtx(ctx),
	)
}

func (e Executor) ClearReactions(ctx context.Context, spec pluginhostlua.MessageGetSpec) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.MessageID == 0 {
		return errors.New("invalid clear_reactions spec")
	}
	return e.client().Rest.RemoveAllReactions(
		snowflake.ID(spec.ChannelID),
		snowflake.ID(spec.MessageID),
		rest.WithCtx(ctx),
	)
}

func (e Executor) ClearReactionsForEmoji(ctx context.Context, spec pluginhostlua.ReactionSpec) error {
	return e.runReactionMutation(ctx, spec, "invalid clear_reactions_for_emoji spec", func() error {
		return e.client().Rest.RemoveAllReactionsForEmoji(
			snowflake.ID(spec.ChannelID),
			snowflake.ID(spec.MessageID),
			strings.TrimSpace(spec.Emoji),
			rest.WithCtx(ctx),
		)
	})
}

func (e Executor) runReactionMutation(
	ctx context.Context,
	spec pluginhostlua.ReactionSpec,
	invalidMessage string,
	run func() error,
) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.MessageID == 0 || strings.TrimSpace(spec.Emoji) == "" {
		return errors.New(invalidMessage)
	}
	return run()
}

func (e Executor) CreateEmoji(
	ctx context.Context,
	spec pluginhostlua.EmojiCreateSpec,
) (pluginhostlua.EmojiResult, error) {
	if e.client() == nil {
		return pluginhostlua.EmojiResult{}, errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || strings.TrimSpace(spec.Name) == "" {
		return pluginhostlua.EmojiResult{}, errors.New("invalid emoji spec")
	}

	if err := validateEmojiAttachment(spec.File); err != nil {
		return pluginhostlua.EmojiResult{}, err
	}

	guild, err := e.client().Rest.GetGuild(snowflake.ID(spec.GuildID), false, rest.WithCtx(ctx))
	if err != nil || guild == nil {
		return pluginhostlua.EmojiResult{}, errors.New("create_error")
	}
	maxAllowed := maxGuildEmojis(guild.PremiumTier)
	if len(guild.Emojis) >= maxAllowed {
		return pluginhostlua.EmojiResult{}, errors.New("too_many:" + strconv.Itoa(maxAllowed))
	}

	body, err := fetchDiscordAttachment(ctx, spec.File, pluginEmojiMaxFileBytes)
	if err != nil {
		return pluginhostlua.EmojiResult{}, errors.New("download_error")
	}
	if !allowedEmojiExtension(spec.File.Filename) {
		return pluginhostlua.EmojiResult{}, errors.New("bad_extension")
	}
	width, height, ok := imageDims(spec.File.Width, spec.File.Height, body)
	if !ok {
		return pluginhostlua.EmojiResult{}, errors.New("dimensions_error")
	}
	if width > pluginEmojiMaxDimension || height > pluginEmojiMaxDimension {
		return pluginhostlua.EmojiResult{}, errors.New("too_large_dims:" + strconv.Itoa(width) + ":" + strconv.Itoa(height))
	}

	icon, err := discord.ParseIconRaw(body)
	if err != nil || icon == nil {
		return pluginhostlua.EmojiResult{}, errors.New("bad_image")
	}

	emoji, err := e.client().Rest.CreateEmoji(snowflake.ID(spec.GuildID), discord.EmojiCreate{
		Name:  strings.TrimSpace(spec.Name),
		Image: *icon,
	}, rest.WithCtx(ctx))
	if err != nil {
		return pluginhostlua.EmojiResult{}, errors.New("create_error")
	}
	return pluginhostlua.EmojiResult{ID: uint64(emoji.ID), Name: emoji.Name}, nil
}

func (e Executor) EditEmoji(
	ctx context.Context,
	spec pluginhostlua.EmojiEditSpec,
) (pluginhostlua.EmojiResult, error) {
	if e.client() == nil {
		return pluginhostlua.EmojiResult{}, errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || strings.TrimSpace(spec.RawEmoji) == "" || strings.TrimSpace(spec.Name) == "" {
		return pluginhostlua.EmojiResult{}, errors.New("invalid emoji spec")
	}

	emojiID, ok := parse.ParseEmojiID(spec.RawEmoji)
	if !ok {
		return pluginhostlua.EmojiResult{}, errors.New("invalid_emoji")
	}

	emoji, err := e.client().Rest.UpdateEmoji(
		snowflake.ID(spec.GuildID),
		emojiID,
		discord.EmojiUpdate{Name: ptr(strings.TrimSpace(spec.Name))},
		rest.WithCtx(ctx),
	)
	if err != nil {
		return pluginhostlua.EmojiResult{}, errors.New("edit_error")
	}
	return pluginhostlua.EmojiResult{ID: uint64(emoji.ID), Name: emoji.Name}, nil
}

func (e Executor) DeleteEmoji(ctx context.Context, spec pluginhostlua.EmojiDeleteSpec) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || strings.TrimSpace(spec.RawEmoji) == "" {
		return errors.New("invalid emoji spec")
	}
	emojiID, ok := parse.ParseEmojiID(spec.RawEmoji)
	if !ok {
		return errors.New("invalid_emoji")
	}
	if err := e.client().Rest.DeleteEmoji(snowflake.ID(spec.GuildID), emojiID, rest.WithCtx(ctx)); err != nil {
		return errors.New("delete_error")
	}
	return nil
}

func (e Executor) CreateSticker(
	ctx context.Context,
	spec pluginhostlua.StickerCreateSpec,
) (pluginhostlua.StickerResult, error) {
	if e.client() == nil {
		return pluginhostlua.StickerResult{}, errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || strings.TrimSpace(spec.Name) == "" || strings.TrimSpace(spec.EmojiTag) == "" {
		return pluginhostlua.StickerResult{}, errors.New("invalid sticker spec")
	}

	if err := validateStickerAttachment(spec.File); err != nil {
		return pluginhostlua.StickerResult{}, err
	}

	guild, err := e.client().Rest.GetGuild(snowflake.ID(spec.GuildID), false, rest.WithCtx(ctx))
	if err != nil || guild == nil {
		return pluginhostlua.StickerResult{}, errors.New("create_error")
	}
	maxAllowed := maxGuildStickers(guild.PremiumTier)
	if len(guild.Stickers) >= maxAllowed {
		return pluginhostlua.StickerResult{}, errors.New("too_many:" + strconv.Itoa(maxAllowed))
	}

	body, err := fetchDiscordAttachment(ctx, spec.File, pluginStickerMaxFileBytes)
	if err != nil {
		return pluginhostlua.StickerResult{}, errors.New("download_error")
	}
	if !allowedStickerExtension(spec.File.Filename) {
		return pluginhostlua.StickerResult{}, errors.New("bad_extension")
	}
	width, height, ok := imageDims(spec.File.Width, spec.File.Height, body)
	if !ok {
		return pluginhostlua.StickerResult{}, errors.New("dimensions_error")
	}
	if width > pluginStickerMaxDimension || height > pluginStickerMaxDimension {
		return pluginhostlua.StickerResult{}, errors.New("too_large_dims:" + strconv.Itoa(width) + ":" + strconv.Itoa(height))
	}

	sticker, err := e.client().Rest.CreateSticker(snowflake.ID(spec.GuildID), discord.StickerCreate{
		Name:        strings.TrimSpace(spec.Name),
		Description: strings.TrimSpace(spec.Description),
		Tags:        strings.TrimSpace(spec.EmojiTag),
		File:        discord.NewFile(spec.File.Filename, "", bytes.NewReader(body)),
	}, rest.WithCtx(ctx))
	if err != nil {
		return pluginhostlua.StickerResult{}, errors.New("create_error")
	}
	return pluginhostlua.StickerResult{ID: uint64(sticker.ID), Name: sticker.Name}, nil
}

func (e Executor) EditSticker(
	ctx context.Context,
	spec pluginhostlua.StickerEditSpec,
) (pluginhostlua.StickerResult, error) {
	if e.client() == nil {
		return pluginhostlua.StickerResult{}, errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || strings.TrimSpace(spec.RawID) == "" || strings.TrimSpace(spec.Name) == "" {
		return pluginhostlua.StickerResult{}, errors.New("invalid sticker spec")
	}
	stickerID, ok := parse.ParseStickerID(spec.RawID)
	if !ok {
		return pluginhostlua.StickerResult{}, errors.New("invalid_id")
	}

	update := discord.StickerUpdate{Name: ptr(strings.TrimSpace(spec.Name))}
	if spec.Description != nil {
		value := strings.TrimSpace(*spec.Description)
		if value != "" {
			update.Description = &value
		}
	}

	sticker, err := e.client().Rest.UpdateSticker(
		snowflake.ID(spec.GuildID),
		stickerID,
		update,
		rest.WithCtx(ctx),
	)
	if err != nil {
		return pluginhostlua.StickerResult{}, errors.New("edit_error")
	}
	return pluginhostlua.StickerResult{ID: uint64(sticker.ID), Name: sticker.Name}, nil
}

func (e Executor) DeleteSticker(ctx context.Context, spec pluginhostlua.StickerDeleteSpec) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || strings.TrimSpace(spec.RawID) == "" {
		return errors.New("invalid sticker spec")
	}
	stickerID, ok := parse.ParseStickerID(spec.RawID)
	if !ok {
		return errors.New("invalid_id")
	}
	if err := e.client().Rest.DeleteSticker(snowflake.ID(spec.GuildID), stickerID, rest.WithCtx(ctx)); err != nil {
		return errors.New("delete_error")
	}
	return nil
}

func (e Executor) CreateEmojiUpload(
	ctx context.Context,
	guildID uint64,
	name, filename string,
	body []byte,
	width, height int,
) (pluginhostlua.EmojiResult, error) {
	if e.client() == nil {
		return pluginhostlua.EmojiResult{}, errors.New("discord client unavailable")
	}
	if guildID == 0 || strings.TrimSpace(name) == "" || strings.TrimSpace(filename) == "" || len(body) == 0 {
		return pluginhostlua.EmojiResult{}, errors.New("invalid emoji spec")
	}
	if len(body) > pluginEmojiMaxFileBytes {
		return pluginhostlua.EmojiResult{}, errors.New("file_too_large")
	}
	if !allowedEmojiExtension(filename) {
		return pluginhostlua.EmojiResult{}, errors.New("bad_extension")
	}

	guild, err := e.client().Rest.GetGuild(snowflake.ID(guildID), false, rest.WithCtx(ctx))
	if err != nil || guild == nil {
		return pluginhostlua.EmojiResult{}, errors.New("create_error")
	}
	maxAllowed := maxGuildEmojis(guild.PremiumTier)
	if len(guild.Emojis) >= maxAllowed {
		return pluginhostlua.EmojiResult{}, errors.New("too_many:" + strconv.Itoa(maxAllowed))
	}

	detectedWidth, detectedHeight, ok := imageDims(width, height, body)
	if !ok {
		return pluginhostlua.EmojiResult{}, errors.New("dimensions_error")
	}
	if detectedWidth > pluginEmojiMaxDimension || detectedHeight > pluginEmojiMaxDimension {
		return pluginhostlua.EmojiResult{}, errors.New("too_large_dims:" + strconv.Itoa(detectedWidth) + ":" + strconv.Itoa(detectedHeight))
	}

	icon, err := discord.ParseIconRaw(body)
	if err != nil || icon == nil {
		return pluginhostlua.EmojiResult{}, errors.New("bad_image")
	}

	emoji, err := e.client().Rest.CreateEmoji(snowflake.ID(guildID), discord.EmojiCreate{
		Name:  strings.TrimSpace(name),
		Image: *icon,
	}, rest.WithCtx(ctx))
	if err != nil {
		return pluginhostlua.EmojiResult{}, errors.New("create_error")
	}
	return pluginhostlua.EmojiResult{ID: uint64(emoji.ID), Name: emoji.Name}, nil
}

func (e Executor) CreateStickerUpload(
	ctx context.Context,
	guildID uint64,
	name, description, emojiTag, filename string,
	body []byte,
	width, height int,
) (pluginhostlua.StickerResult, error) {
	if e.client() == nil {
		return pluginhostlua.StickerResult{}, errors.New("discord client unavailable")
	}
	if guildID == 0 || strings.TrimSpace(name) == "" || strings.TrimSpace(emojiTag) == "" || strings.TrimSpace(filename) == "" || len(body) == 0 {
		return pluginhostlua.StickerResult{}, errors.New("invalid sticker spec")
	}
	if len(body) > pluginStickerMaxFileBytes {
		return pluginhostlua.StickerResult{}, errors.New("file_too_large")
	}
	if !allowedStickerExtension(filename) {
		return pluginhostlua.StickerResult{}, errors.New("bad_extension")
	}

	guild, err := e.client().Rest.GetGuild(snowflake.ID(guildID), false, rest.WithCtx(ctx))
	if err != nil || guild == nil {
		return pluginhostlua.StickerResult{}, errors.New("create_error")
	}
	maxAllowed := maxGuildStickers(guild.PremiumTier)
	if len(guild.Stickers) >= maxAllowed {
		return pluginhostlua.StickerResult{}, errors.New("too_many:" + strconv.Itoa(maxAllowed))
	}

	detectedWidth, detectedHeight, ok := imageDims(width, height, body)
	if !ok {
		return pluginhostlua.StickerResult{}, errors.New("dimensions_error")
	}
	if detectedWidth > pluginStickerMaxDimension || detectedHeight > pluginStickerMaxDimension {
		return pluginhostlua.StickerResult{}, errors.New("too_large_dims:" + strconv.Itoa(detectedWidth) + ":" + strconv.Itoa(detectedHeight))
	}

	sticker, err := e.client().Rest.CreateSticker(snowflake.ID(guildID), discord.StickerCreate{
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(description),
		Tags:        strings.TrimSpace(emojiTag),
		File:        discord.NewFile(filename, "", bytes.NewReader(body)),
	}, rest.WithCtx(ctx))
	if err != nil {
		return pluginhostlua.StickerResult{}, errors.New("create_error")
	}
	return pluginhostlua.StickerResult{ID: uint64(sticker.ID), Name: sticker.Name}, nil
}

func roleResult(role discord.Role) pluginhostlua.RoleResult {
	return pluginhostlua.RoleResult{
		ID:          uint64(role.ID),
		Name:        role.Name,
		Mention:     discord.RoleMention(role.ID),
		Color:       role.Color,
		Hoist:       role.Hoist,
		Mentionable: role.Mentionable,
		Position:    role.Position,
		Managed:     role.Managed,
		Permissions: int64(role.Permissions),
		CreatedAt:   role.CreatedAt().UTC().Unix(),
	}
}

func fetchDiscordAttachment(ctx context.Context, file pluginhostlua.AttachmentInput, maxBytes int64) ([]byte, error) {
	if strings.TrimSpace(file.URL) == "" || strings.TrimSpace(file.Filename) == "" {
		return nil, errors.New("file_missing")
	}
	fetcher := cdn.NewDiscordCDNFetcher()
	return fetcher.Fetch(ctx, file.URL, maxBytes)
}

func validateEmojiAttachment(file pluginhostlua.AttachmentInput) error {
	if file.ID == 0 || strings.TrimSpace(file.URL) == "" {
		return errors.New("file_missing")
	}
	if file.Size > pluginEmojiMaxFileBytes {
		return errors.New("file_too_large")
	}
	if !allowedEmojiExtension(file.Filename) {
		return errors.New("bad_extension")
	}
	return nil
}

func validateStickerAttachment(file pluginhostlua.AttachmentInput) error {
	if file.ID == 0 || strings.TrimSpace(file.URL) == "" {
		return errors.New("file_missing")
	}
	if file.Size > pluginStickerMaxFileBytes {
		return errors.New("file_too_large")
	}
	if !allowedStickerExtension(file.Filename) {
		return errors.New("bad_extension")
	}
	return nil
}

func allowedEmojiExtension(filename string) bool {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), ".")) {
	case "gif", "jpeg", "jpg", "png":
		return true
	default:
		return false
	}
}

func allowedStickerExtension(filename string) bool {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), ".")) {
	case "png", "gif", "apng":
		return true
	default:
		return false
	}
}

func imageDims(width, height int, raw []byte) (int, int, bool) {
	if width > 0 && height > 0 {
		return width, height, true
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, false
	}
	return cfg.Width, cfg.Height, true
}

func maxGuildEmojis(tier discord.PremiumTier) int {
	switch tier {
	case discord.PremiumTierNone:
		return 50
	case discord.PremiumTier1:
		return 100
	case discord.PremiumTier2:
		return 150
	case discord.PremiumTier3:
		return 250
	default:
		return 50
	}
}

func maxGuildStickers(tier discord.PremiumTier) int {
	switch tier {
	case discord.PremiumTierNone:
		return 5
	case discord.PremiumTier1:
		return 15
	case discord.PremiumTier2:
		return 30
	case discord.PremiumTier3:
		return 60
	default:
		return 5
	}
}

func purgeAnchorIDs(mode, raw string) (snowflake.ID, snowflake.ID, snowflake.ID, bool) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "all":
		return 0, 0, 0, true
	case "before":
		id, ok := parse.ParseMessageID(raw)
		return 0, id, 0, ok
	case "after":
		id, ok := parse.ParseMessageID(raw)
		return 0, 0, id, ok
	case "around":
		id, ok := parse.ParseMessageID(raw)
		return id, 0, 0, ok
	default:
		return 0, 0, 0, false
	}
}

func deleteMessagesBestEffort(r rest.Rest, channelID snowflake.ID, messageIDs []snowflake.ID, now time.Time) int {
	if len(messageIDs) == 0 {
		return 0
	}

	const (
		discordBulkDeleteMaxAge  = 14 * 24 * time.Hour
		discordBulkDeletePadding = time.Hour
		bulkDeleteChunkMax       = 100
	)
	cutoff := now.Add(-discordBulkDeleteMaxAge).Add(discordBulkDeletePadding)

	var bulkIDs []snowflake.ID
	var singleIDs []snowflake.ID
	for _, id := range messageIDs {
		if id == 0 {
			continue
		}
		if id.Time().Before(cutoff) {
			singleIDs = append(singleIDs, id)
			continue
		}
		bulkIDs = append(bulkIDs, id)
	}

	deleted := 0
	for len(bulkIDs) > 0 {
		chunk := bulkIDs
		if len(chunk) > bulkDeleteChunkMax {
			chunk = bulkIDs[:bulkDeleteChunkMax]
		}
		bulkIDs = bulkIDs[len(chunk):]

		if len(chunk) == 1 {
			singleIDs = append(singleIDs, chunk[0])
			continue
		}
		if err := r.BulkDeleteMessages(channelID, chunk); err != nil {
			singleIDs = append(singleIDs, chunk...)
			continue
		}
		deleted += len(chunk)
	}

	for _, id := range singleIDs {
		if err := r.DeleteMessage(channelID, id); err != nil {
			continue
		}
		deleted++
	}
	return deleted
}

func ptr[T any](value T) *T {
	return &value
}
