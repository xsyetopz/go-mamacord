package pluginbridge

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"path/filepath"
	"strings"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/omit"
	"github.com/disgoorg/snowflake/v2"

	discordcontrol "github.com/xsyetopz/go-mamacord/internal/runtime/discord/control"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/parse"
)

const (
	pluginEmojiMaxFileBytes   = 256 * 1024
	pluginEmojiMaxDimension   = 320
	pluginStickerMaxFileBytes = 512 * 1024
	pluginStickerMaxDimension = 320
)

func (e Executor) CreateRole(ctx context.Context, spec discordcontrol.RoleCreateSpec) (discordcontrol.RoleResult, error) {
	if e.client() == nil {
		return discordcontrol.RoleResult{}, errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || strings.TrimSpace(spec.Name) == "" {
		return discordcontrol.RoleResult{}, errors.New("invalid role spec")
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
		return discordcontrol.RoleResult{}, err
	}
	return roleResult(*role), nil
}
func (e Executor) EditRole(ctx context.Context, spec discordcontrol.RoleEditSpec) (discordcontrol.RoleResult, error) {
	if e.client() == nil {
		return discordcontrol.RoleResult{}, errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || spec.RoleID == 0 || spec.RoleID == spec.GuildID {
		return discordcontrol.RoleResult{}, errors.New("invalid role spec")
	}
	input := discord.RoleUpdate{Name: spec.Name, Color: spec.Color, Hoist: spec.Hoist, Mentionable: spec.Mentionable}
	role, err := e.client().Rest.UpdateRole(snowflake.ID(spec.GuildID), snowflake.ID(spec.RoleID), input, rest.WithCtx(ctx))
	if err != nil {
		return discordcontrol.RoleResult{}, err
	}
	return roleResult(*role), nil
}
func (e Executor) DeleteRole(ctx context.Context, guildID, roleID uint64) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if guildID == 0 || roleID == 0 || guildID == roleID {
		return errors.New("invalid role spec")
	}
	return e.client().Rest.DeleteRole(snowflake.ID(guildID), snowflake.ID(roleID), rest.WithCtx(ctx))
}
func (e Executor) AddRole(ctx context.Context, spec discordcontrol.RoleMemberSpec) error {
	return e.memberRole(ctx, spec, true)
}
func (e Executor) RemoveRole(ctx context.Context, spec discordcontrol.RoleMemberSpec) error {
	return e.memberRole(ctx, spec, false)
}
func (e Executor) memberRole(ctx context.Context, spec discordcontrol.RoleMemberSpec, add bool) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || spec.UserID == 0 || spec.RoleID == 0 {
		return errors.New("invalid role member spec")
	}
	if add {
		return e.client().Rest.AddMemberRole(snowflake.ID(spec.GuildID), snowflake.ID(spec.UserID), snowflake.ID(spec.RoleID), rest.WithCtx(ctx))
	}
	return e.client().Rest.RemoveMemberRole(snowflake.ID(spec.GuildID), snowflake.ID(spec.UserID), snowflake.ID(spec.RoleID), rest.WithCtx(ctx))
}
func (e Executor) PurgeMessages(ctx context.Context, spec discordcontrol.PurgeSpec) (int, error) {
	if e.client() == nil {
		return 0, errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.Count <= 0 {
		return 0, errors.New("invalid purge spec")
	}
	around, before, after, ok := purgeAnchorIDs(spec.Mode, spec.AnchorRaw)
	if !ok {
		return 0, errors.New("invalid message")
	}
	messages, err := e.client().Rest.GetMessages(snowflake.ID(spec.ChannelID), around, before, after, spec.Count, rest.WithCtx(ctx))
	if err != nil {
		return 0, err
	}
	ids := make([]snowflake.ID, len(messages))
	for i, message := range messages {
		ids[i] = message.ID
	}
	return deleteMessages(ctx, e.client().Rest, snowflake.ID(spec.ChannelID), ids, time.Now())
}
func (e Executor) CreateEmojiUpload(ctx context.Context, guildID uint64, name, filename string, body []byte, width, height int) (discordcontrol.EmojiResult, error) {
	if e.client() == nil {
		return discordcontrol.EmojiResult{}, errors.New("discord client unavailable")
	}
	if guildID == 0 || strings.TrimSpace(name) == "" || len(body) == 0 || len(body) > pluginEmojiMaxFileBytes || !allowedEmojiExtension(filename) {
		return discordcontrol.EmojiResult{}, errors.New("invalid emoji spec")
	}
	guild, err := e.client().Rest.GetGuild(snowflake.ID(guildID), false, rest.WithCtx(ctx))
	if err != nil || guild == nil || len(guild.Emojis) >= maxGuildEmojis(guild.PremiumTier) {
		return discordcontrol.EmojiResult{}, errors.New("emoji capacity unavailable")
	}
	width, height, ok := imageDims(width, height, body)
	if !ok || width > pluginEmojiMaxDimension || height > pluginEmojiMaxDimension {
		return discordcontrol.EmojiResult{}, errors.New("invalid emoji dimensions")
	}
	icon, err := discord.ParseIconRaw(body)
	if err != nil || icon == nil {
		return discordcontrol.EmojiResult{}, errors.New("invalid emoji image")
	}
	emoji, err := e.client().Rest.CreateEmoji(snowflake.ID(guildID), discord.EmojiCreate{Name: strings.TrimSpace(name), Image: *icon}, rest.WithCtx(ctx))
	if err != nil {
		return discordcontrol.EmojiResult{}, err
	}
	return discordcontrol.EmojiResult{ID: uint64(emoji.ID), Name: emoji.Name}, nil
}
func (e Executor) EditEmoji(ctx context.Context, spec discordcontrol.EmojiEditSpec) (discordcontrol.EmojiResult, error) {
	if e.client() == nil {
		return discordcontrol.EmojiResult{}, errors.New("discord client unavailable")
	}
	id, ok := parse.ParseEmojiID(spec.RawEmoji)
	if spec.GuildID == 0 || !ok || strings.TrimSpace(spec.Name) == "" {
		return discordcontrol.EmojiResult{}, errors.New("invalid emoji spec")
	}
	name := strings.TrimSpace(spec.Name)
	emoji, err := e.client().Rest.UpdateEmoji(snowflake.ID(spec.GuildID), id, discord.EmojiUpdate{Name: &name}, rest.WithCtx(ctx))
	if err != nil {
		return discordcontrol.EmojiResult{}, err
	}
	return discordcontrol.EmojiResult{ID: uint64(emoji.ID), Name: emoji.Name}, nil
}
func (e Executor) DeleteEmoji(ctx context.Context, spec discordcontrol.EmojiDeleteSpec) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	id, ok := parse.ParseEmojiID(spec.RawEmoji)
	if spec.GuildID == 0 || !ok {
		return errors.New("invalid emoji spec")
	}
	return e.client().Rest.DeleteEmoji(snowflake.ID(spec.GuildID), id, rest.WithCtx(ctx))
}
func (e Executor) CreateStickerUpload(ctx context.Context, guildID uint64, name, description, emojiTag, filename string, body []byte, width, height int) (discordcontrol.StickerResult, error) {
	if e.client() == nil {
		return discordcontrol.StickerResult{}, errors.New("discord client unavailable")
	}
	if guildID == 0 || strings.TrimSpace(name) == "" || strings.TrimSpace(emojiTag) == "" || len(body) == 0 || len(body) > pluginStickerMaxFileBytes || !allowedStickerExtension(filename) {
		return discordcontrol.StickerResult{}, errors.New("invalid sticker spec")
	}
	guild, err := e.client().Rest.GetGuild(snowflake.ID(guildID), false, rest.WithCtx(ctx))
	if err != nil || guild == nil || len(guild.Stickers) >= maxGuildStickers(guild.PremiumTier) {
		return discordcontrol.StickerResult{}, errors.New("sticker capacity unavailable")
	}
	width, height, ok := imageDims(width, height, body)
	if !ok || width > pluginStickerMaxDimension || height > pluginStickerMaxDimension {
		return discordcontrol.StickerResult{}, errors.New("invalid sticker dimensions")
	}
	sticker, err := e.client().Rest.CreateSticker(snowflake.ID(guildID), discord.StickerCreate{Name: strings.TrimSpace(name), Description: strings.TrimSpace(description), Tags: strings.TrimSpace(emojiTag), File: discord.NewFile(filename, "", bytes.NewReader(body))}, rest.WithCtx(ctx))
	if err != nil {
		return discordcontrol.StickerResult{}, err
	}
	return discordcontrol.StickerResult{ID: uint64(sticker.ID), Name: sticker.Name}, nil
}
func (e Executor) EditSticker(ctx context.Context, spec discordcontrol.StickerEditSpec) (discordcontrol.StickerResult, error) {
	if e.client() == nil {
		return discordcontrol.StickerResult{}, errors.New("discord client unavailable")
	}
	id, ok := parse.ParseStickerID(spec.RawID)
	if spec.GuildID == 0 || !ok || strings.TrimSpace(spec.Name) == "" {
		return discordcontrol.StickerResult{}, errors.New("invalid sticker spec")
	}
	name := strings.TrimSpace(spec.Name)
	update := discord.StickerUpdate{Name: &name, Description: spec.Description}
	sticker, err := e.client().Rest.UpdateSticker(snowflake.ID(spec.GuildID), id, update, rest.WithCtx(ctx))
	if err != nil {
		return discordcontrol.StickerResult{}, err
	}
	return discordcontrol.StickerResult{ID: uint64(sticker.ID), Name: sticker.Name}, nil
}
func (e Executor) DeleteSticker(ctx context.Context, spec discordcontrol.StickerDeleteSpec) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	id, ok := parse.ParseStickerID(spec.RawID)
	if spec.GuildID == 0 || !ok {
		return errors.New("invalid sticker spec")
	}
	return e.client().Rest.DeleteSticker(snowflake.ID(spec.GuildID), id, rest.WithCtx(ctx))
}
func roleResult(role discord.Role) discordcontrol.RoleResult {
	return discordcontrol.RoleResult{ID: uint64(role.ID), Name: role.Name, Mention: discord.RoleMention(role.ID), Color: role.Color, Hoist: role.Hoist, Mentionable: role.Mentionable, Position: role.Position, Managed: role.Managed, Permissions: int64(role.Permissions), CreatedAt: role.CreatedAt().UTC().Unix()}
}
func allowedEmojiExtension(filename string) bool {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), ".")) {
	case "gif", "jpeg", "jpg", "png":
		return true
	}
	return false
}
func allowedStickerExtension(filename string) bool {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), ".")) {
	case "png", "gif", "apng":
		return true
	}
	return false
}
func imageDims(width, height int, raw []byte) (int, int, bool) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, false
	}
	if width > 0 && width != cfg.Width || height > 0 && height != cfg.Height {
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
	}
	return 50
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
	}
	return 5
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
	}
	return 0, 0, 0, false
}
func deleteMessages(ctx context.Context, r rest.Rest, channelID snowflake.ID, messageIDs []snowflake.ID, now time.Time) (int, error) {
	const maxAge = 14 * 24 * time.Hour
	cutoff := now.Add(-maxAge).Add(time.Hour)
	bulk, single := []snowflake.ID{}, []snowflake.ID{}
	for _, id := range messageIDs {
		if id == 0 {
			continue
		}
		if id.Time().Before(cutoff) {
			single = append(single, id)
		} else {
			bulk = append(bulk, id)
		}
	}
	deleted := 0
	for len(bulk) > 0 {
		n := min(len(bulk), 100)
		chunk := bulk[:n]
		bulk = bulk[n:]
		if len(chunk) == 1 {
			single = append(single, chunk[0])
			continue
		}
		if err := r.BulkDeleteMessages(channelID, chunk, rest.WithCtx(ctx)); err != nil {
			single = append(single, chunk...)
		} else {
			deleted += len(chunk)
		}
	}
	var failures []error
	for _, id := range single {
		if err := r.DeleteMessage(channelID, id, rest.WithCtx(ctx)); err != nil {
			failures = append(failures, err)
		} else {
			deleted++
		}
	}
	if len(failures) != 0 {
		return deleted, fmt.Errorf("delete %d of %d messages: %w", deleted, len(messageIDs), errors.Join(failures...))
	}
	return deleted, nil
}

func (e Executor) SetSlowmode(ctx context.Context, channelID uint64, seconds int) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if channelID == 0 || seconds < 0 {
		return errors.New("invalid slowmode spec")
	}
	_, err := e.client().Rest.UpdateChannel(snowflake.ID(channelID), discord.GuildTextChannelUpdate{RateLimitPerUser: &seconds}, rest.WithCtx(ctx))
	return err
}
func (e Executor) SetNickname(ctx context.Context, guildID, userID uint64, nickname *string) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if guildID == 0 || userID == 0 {
		return errors.New("invalid nickname spec")
	}
	value := ""
	if nickname != nil {
		value = strings.TrimSpace(*nickname)
	}
	_, err := e.client().Rest.UpdateMember(snowflake.ID(guildID), snowflake.ID(userID), discord.MemberUpdate{Nick: &value}, rest.WithCtx(ctx))
	return err
}
func (e Executor) TimeoutMember(ctx context.Context, guildID, userID uint64, until time.Time) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if guildID == 0 || userID == 0 {
		return errors.New("invalid timeout spec")
	}
	_, err := e.client().Rest.UpdateMember(snowflake.ID(guildID), snowflake.ID(userID), discord.MemberUpdate{CommunicationDisabledUntil: omit.NewPtr(until.UTC())}, rest.WithCtx(ctx))
	return err
}
