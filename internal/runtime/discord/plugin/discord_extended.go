package plugin

import (
	"context"
	"errors"
	"strings"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/omit"
	"github.com/disgoorg/snowflake/v2"

	pluginhostlua "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/lua"
)

func (e Executor) CreateChannel(
	ctx context.Context,
	spec pluginhostlua.ChannelCreateSpec,
) (pluginhostlua.ChannelResult, error) {
	if e.client() == nil {
		return pluginhostlua.ChannelResult{}, errors.New("discord client unavailable")
	}
	if spec.GuildID == 0 || strings.TrimSpace(spec.Name) == "" {
		return pluginhostlua.ChannelResult{}, errors.New("invalid channel spec")
	}

	channelType := strings.ToLower(strings.TrimSpace(spec.Type))
	if channelType == "" {
		channelType = "text"
	}

	var create discord.GuildChannelCreate
	switch channelType {
	case "text", "guild_text":
		input := discord.GuildTextChannelCreate{Name: strings.TrimSpace(spec.Name)}
		if spec.Topic != nil {
			input.Topic = strings.TrimSpace(*spec.Topic)
		}
		if spec.Slowmode != nil {
			input.RateLimitPerUser = *spec.Slowmode
		}
		if spec.Position != nil {
			input.Position = *spec.Position
		}
		if spec.ParentID != nil {
			input.ParentID = snowflake.ID(*spec.ParentID)
		}
		if spec.NSFW != nil {
			input.NSFW = *spec.NSFW
		}
		create = input
	case "voice", "guild_voice":
		input := discord.GuildVoiceChannelCreate{Name: strings.TrimSpace(spec.Name)}
		if spec.Bitrate != nil {
			input.Bitrate = *spec.Bitrate
		}
		if spec.UserLimit != nil {
			input.UserLimit = *spec.UserLimit
		}
		if spec.Slowmode != nil {
			input.RateLimitPerUser = *spec.Slowmode
		}
		if spec.Position != nil {
			input.Position = *spec.Position
		}
		if spec.ParentID != nil {
			input.ParentID = snowflake.ID(*spec.ParentID)
		}
		if spec.NSFW != nil {
			input.NSFW = *spec.NSFW
		}
		create = input
	case "category", "guild_category":
		input := discord.GuildCategoryChannelCreate{Name: strings.TrimSpace(spec.Name)}
		if spec.Position != nil {
			input.Position = *spec.Position
		}
		create = input
	case "news", "announcement", "guild_news":
		input := discord.GuildNewsChannelCreate{Name: strings.TrimSpace(spec.Name)}
		if spec.Topic != nil {
			input.Topic = strings.TrimSpace(*spec.Topic)
		}
		if spec.Slowmode != nil {
			input.RateLimitPerUser = *spec.Slowmode
		}
		if spec.Position != nil {
			input.Position = *spec.Position
		}
		if spec.ParentID != nil {
			input.ParentID = snowflake.ID(*spec.ParentID)
		}
		if spec.NSFW != nil {
			input.NSFW = *spec.NSFW
		}
		create = input
	default:
		return pluginhostlua.ChannelResult{}, errors.New("unsupported_channel_type")
	}

	channel, err := e.client().Rest.CreateGuildChannel(snowflake.ID(spec.GuildID), create, rest.WithCtx(ctx))
	if err != nil {
		return pluginhostlua.ChannelResult{}, errors.New("create_channel_error")
	}
	return channelResult(channel), nil
}

func (e Executor) EditChannel(
	ctx context.Context,
	spec pluginhostlua.ChannelEditSpec,
) (pluginhostlua.ChannelResult, error) {
	if e.client() == nil {
		return pluginhostlua.ChannelResult{}, errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 {
		return pluginhostlua.ChannelResult{}, errors.New("invalid channel spec")
	}

	channel, err := e.client().Rest.GetChannel(snowflake.ID(spec.ChannelID), rest.WithCtx(ctx))
	if err != nil || channel == nil {
		return pluginhostlua.ChannelResult{}, errors.New("edit_channel_error")
	}

	var update discord.ChannelUpdate
	switch channel.Type() {
	case discord.ChannelTypeGuildText:
		input := discord.GuildTextChannelUpdate{
			Name:             spec.Name,
			Topic:            spec.Topic,
			NSFW:             spec.NSFW,
			RateLimitPerUser: spec.Slowmode,
			Position:         spec.Position,
			ParentID:         snowflakePtr(spec.ParentID),
		}
		update = input
	case discord.ChannelTypeGuildVoice, discord.ChannelTypeGuildStageVoice:
		input := discord.GuildVoiceChannelUpdate{
			Name:             spec.Name,
			Bitrate:          spec.Bitrate,
			UserLimit:        spec.UserLimit,
			RateLimitPerUser: spec.Slowmode,
			Position:         spec.Position,
			ParentID:         snowflakePtr(spec.ParentID),
			NSFW:             spec.NSFW,
		}
		update = input
	case discord.ChannelTypeGuildCategory:
		input := discord.GuildCategoryChannelUpdate{
			Name:     spec.Name,
			Position: spec.Position,
		}
		update = input
	case discord.ChannelTypeGuildNews:
		input := discord.GuildNewsChannelUpdate{
			Name:             spec.Name,
			Topic:            spec.Topic,
			RateLimitPerUser: spec.Slowmode,
			Position:         spec.Position,
			ParentID:         snowflakePtr(spec.ParentID),
		}
		update = input
	default:
		return pluginhostlua.ChannelResult{}, errors.New("unsupported_channel_type")
	}

	updated, err := e.client().Rest.UpdateChannel(snowflake.ID(spec.ChannelID), update, rest.WithCtx(ctx))
	if err != nil {
		return pluginhostlua.ChannelResult{}, errors.New("edit_channel_error")
	}
	return channelResult(updated), nil
}

func (e Executor) DeleteChannel(ctx context.Context, channelID uint64) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if channelID == 0 {
		return errors.New("invalid channel spec")
	}
	return e.client().Rest.DeleteChannel(snowflake.ID(channelID), rest.WithCtx(ctx))
}

func (e Executor) SetChannelOverwrite(ctx context.Context, spec pluginhostlua.PermissionOverwriteSpec) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.OverwriteID == 0 {
		return errors.New("invalid overwrite spec")
	}

	var update discord.PermissionOverwriteUpdate
	allow := discord.Permissions(spec.Allow)
	deny := discord.Permissions(spec.Deny)
	switch strings.ToLower(strings.TrimSpace(spec.TargetType)) {
	case "role":
		update = discord.RolePermissionOverwriteUpdate{
			Allow: &allow,
			Deny:  &deny,
		}
	case "member", "user":
		update = discord.MemberPermissionOverwriteUpdate{
			Allow: &allow,
			Deny:  &deny,
		}
	default:
		return errors.New("invalid overwrite spec")
	}

	return e.client().Rest.UpdatePermissionOverwrite(
		snowflake.ID(spec.ChannelID),
		snowflake.ID(spec.OverwriteID),
		update,
		rest.WithCtx(ctx),
	)
}

func (e Executor) DeleteChannelOverwrite(ctx context.Context, channelID, overwriteID uint64) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if channelID == 0 || overwriteID == 0 {
		return errors.New("invalid overwrite spec")
	}
	return e.client().Rest.DeletePermissionOverwrite(
		snowflake.ID(channelID),
		snowflake.ID(overwriteID),
		rest.WithCtx(ctx),
	)
}

func (e Executor) CreateThreadFromMessage(
	ctx context.Context,
	spec pluginhostlua.ThreadCreateFromMessageSpec,
) (pluginhostlua.ThreadResult, error) {
	if e.client() == nil {
		return pluginhostlua.ThreadResult{}, errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || spec.MessageID == 0 || strings.TrimSpace(spec.Name) == "" {
		return pluginhostlua.ThreadResult{}, errors.New("invalid thread spec")
	}
	input := discord.ThreadCreateFromMessage{Name: strings.TrimSpace(spec.Name)}
	if spec.AutoArchiveDuration > 0 {
		input.AutoArchiveDuration = discord.AutoArchiveDuration(spec.AutoArchiveDuration)
	}
	if spec.Slowmode > 0 {
		input.RateLimitPerUser = spec.Slowmode
	}
	thread, err := e.client().Rest.CreateThreadFromMessage(
		snowflake.ID(spec.ChannelID),
		snowflake.ID(spec.MessageID),
		input,
		rest.WithCtx(ctx),
	)
	if err != nil || thread == nil {
		return pluginhostlua.ThreadResult{}, errors.New("create_thread_error")
	}
	return threadResult(*thread), nil
}

func (e Executor) CreateThreadInChannel(
	ctx context.Context,
	spec pluginhostlua.ThreadCreateSpec,
) (pluginhostlua.ThreadResult, error) {
	if e.client() == nil {
		return pluginhostlua.ThreadResult{}, errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || strings.TrimSpace(spec.Name) == "" {
		return pluginhostlua.ThreadResult{}, errors.New("invalid thread spec")
	}
	threadType := strings.ToLower(strings.TrimSpace(spec.Type))
	if threadType == "" {
		threadType = "public"
	}

	var create discord.ThreadCreate
	switch threadType {
	case "public", "guild_public_thread":
		input := discord.GuildPublicThreadCreate{Name: strings.TrimSpace(spec.Name)}
		if spec.AutoArchiveDuration > 0 {
			input.AutoArchiveDuration = discord.AutoArchiveDuration(spec.AutoArchiveDuration)
		}
		create = input
	case "private", "guild_private_thread":
		input := discord.GuildPrivateThreadCreate{Name: strings.TrimSpace(spec.Name)}
		if spec.AutoArchiveDuration > 0 {
			input.AutoArchiveDuration = discord.AutoArchiveDuration(spec.AutoArchiveDuration)
		}
		if spec.Invitable != nil {
			input.Invitable = spec.Invitable
		}
		create = input
	case "news", "announcement", "guild_news_thread":
		input := discord.GuildNewsThreadCreate{Name: strings.TrimSpace(spec.Name)}
		if spec.AutoArchiveDuration > 0 {
			input.AutoArchiveDuration = discord.AutoArchiveDuration(spec.AutoArchiveDuration)
		}
		create = input
	default:
		return pluginhostlua.ThreadResult{}, errors.New("unsupported_thread_type")
	}

	thread, err := e.client().Rest.CreateThread(snowflake.ID(spec.ChannelID), create, rest.WithCtx(ctx))
	if err != nil || thread == nil {
		return pluginhostlua.ThreadResult{}, errors.New("create_thread_error")
	}
	return threadResult(*thread), nil
}

func (e Executor) JoinThread(ctx context.Context, threadID uint64) error {
	return e.threadAction(ctx, threadID, e.client().Rest.JoinThread)
}

func (e Executor) LeaveThread(ctx context.Context, threadID uint64) error {
	return e.threadAction(ctx, threadID, e.client().Rest.LeaveThread)
}

func (e Executor) threadAction(ctx context.Context, threadID uint64, run func(snowflake.ID, ...rest.RequestOpt) error) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if threadID == 0 {
		return errors.New("invalid thread spec")
	}
	return run(snowflake.ID(threadID), rest.WithCtx(ctx))
}

func (e Executor) AddThreadMember(ctx context.Context, threadID, userID uint64) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if threadID == 0 || userID == 0 {
		return errors.New("invalid thread member spec")
	}
	return e.client().Rest.AddThreadMember(snowflake.ID(threadID), snowflake.ID(userID), rest.WithCtx(ctx))
}

func (e Executor) RemoveThreadMember(ctx context.Context, threadID, userID uint64) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if threadID == 0 || userID == 0 {
		return errors.New("invalid thread member spec")
	}
	return e.client().Rest.RemoveThreadMember(snowflake.ID(threadID), snowflake.ID(userID), rest.WithCtx(ctx))
}

func (e Executor) UpdateThread(
	ctx context.Context,
	spec pluginhostlua.ThreadUpdateSpec,
) (pluginhostlua.ThreadResult, error) {
	if e.client() == nil {
		return pluginhostlua.ThreadResult{}, errors.New("discord client unavailable")
	}
	if spec.ThreadID == 0 {
		return pluginhostlua.ThreadResult{}, errors.New("invalid thread spec")
	}
	update := discord.GuildThreadUpdate{
		Name:             spec.Name,
		Archived:         spec.Archived,
		Locked:           spec.Locked,
		Invitable:        spec.Invitable,
		RateLimitPerUser: spec.Slowmode,
	}
	if spec.AutoArchiveDuration != nil {
		duration := discord.AutoArchiveDuration(*spec.AutoArchiveDuration)
		update.AutoArchiveDuration = &duration
	}
	channel, err := e.client().Rest.UpdateChannel(snowflake.ID(spec.ThreadID), update, rest.WithCtx(ctx))
	if err != nil {
		return pluginhostlua.ThreadResult{}, errors.New("update_thread_error")
	}
	if threadValue, ok := channel.(discord.GuildThread); ok {
		return threadResult(threadValue), nil
	}
	return pluginhostlua.ThreadResult{}, errors.New("update_thread_error")
}

func (e Executor) CreateInvite(
	ctx context.Context,
	spec pluginhostlua.InviteCreateSpec,
) (pluginhostlua.InviteResult, error) {
	if e.client() == nil {
		return pluginhostlua.InviteResult{}, errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 {
		return pluginhostlua.InviteResult{}, errors.New("invalid invite spec")
	}
	input := discord.InviteCreate{
		Temporary: spec.Temporary,
		Unique:    spec.Unique,
	}
	if spec.MaxAge != nil {
		input.MaxAge = spec.MaxAge
	}
	if spec.MaxUses != nil {
		input.MaxUses = spec.MaxUses
	}
	invite, err := e.client().Rest.CreateInvite(snowflake.ID(spec.ChannelID), input, rest.WithCtx(ctx))
	if err != nil || invite == nil {
		return pluginhostlua.InviteResult{}, errors.New("create_invite_error")
	}
	return inviteResult(*invite), nil
}

func (e Executor) GetInvite(ctx context.Context, code string) (pluginhostlua.InviteResult, error) {
	if e.client() == nil {
		return pluginhostlua.InviteResult{}, errors.New("discord client unavailable")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return pluginhostlua.InviteResult{}, errors.New("invalid invite spec")
	}
	invite, err := e.client().Rest.GetInvite(code, rest.WithCtx(ctx))
	if err != nil || invite == nil {
		return pluginhostlua.InviteResult{}, errors.New("get_invite_error")
	}
	return inviteResult(*invite), nil
}

func (e Executor) DeleteInvite(ctx context.Context, code string) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	code = strings.TrimSpace(code)
	if code == "" {
		return errors.New("invalid invite spec")
	}
	_, err := e.client().Rest.DeleteInvite(code, rest.WithCtx(ctx))
	return err
}

func (e Executor) ListChannelInvites(ctx context.Context, channelID uint64) ([]pluginhostlua.InviteResult, error) {
	if e.client() == nil {
		return nil, errors.New("discord client unavailable")
	}
	if channelID == 0 {
		return nil, errors.New("invalid invite spec")
	}
	invites, err := e.client().Rest.GetChannelInvites(snowflake.ID(channelID), rest.WithCtx(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]pluginhostlua.InviteResult, 0, len(invites))
	for _, invite := range invites {
		out = append(out, extendedInviteResult(invite))
	}
	return out, nil
}

func (e Executor) ListGuildInvites(ctx context.Context, guildID uint64) ([]pluginhostlua.InviteResult, error) {
	if e.client() == nil {
		return nil, errors.New("discord client unavailable")
	}
	if guildID == 0 {
		return nil, errors.New("invalid invite spec")
	}
	invites, err := e.client().Rest.GetGuildInvites(snowflake.ID(guildID), rest.WithCtx(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]pluginhostlua.InviteResult, 0, len(invites))
	for _, invite := range invites {
		out = append(out, extendedInviteResult(invite))
	}
	return out, nil
}

func (e Executor) CreateWebhook(
	ctx context.Context,
	spec pluginhostlua.WebhookCreateSpec,
) (pluginhostlua.WebhookResult, error) {
	if e.client() == nil {
		return pluginhostlua.WebhookResult{}, errors.New("discord client unavailable")
	}
	if spec.ChannelID == 0 || strings.TrimSpace(spec.Name) == "" {
		return pluginhostlua.WebhookResult{}, errors.New("invalid webhook spec")
	}
	webhook, err := e.client().Rest.CreateWebhook(
		snowflake.ID(spec.ChannelID),
		discord.WebhookCreate{Name: strings.TrimSpace(spec.Name)},
		rest.WithCtx(ctx),
	)
	if err != nil || webhook == nil {
		return pluginhostlua.WebhookResult{}, errors.New("create_webhook_error")
	}
	return webhookResult(*webhook), nil
}

func (e Executor) GetWebhook(ctx context.Context, webhookID uint64) (pluginhostlua.WebhookResult, error) {
	if e.client() == nil {
		return pluginhostlua.WebhookResult{}, errors.New("discord client unavailable")
	}
	if webhookID == 0 {
		return pluginhostlua.WebhookResult{}, errors.New("invalid webhook spec")
	}
	webhook, err := e.client().Rest.GetWebhook(snowflake.ID(webhookID), rest.WithCtx(ctx))
	if err != nil {
		return pluginhostlua.WebhookResult{}, errors.New("get_webhook_error")
	}
	return webhookResultFromValue(webhook)
}

func (e Executor) ListChannelWebhooks(ctx context.Context, channelID uint64) ([]pluginhostlua.WebhookResult, error) {
	if e.client() == nil {
		return nil, errors.New("discord client unavailable")
	}
	if channelID == 0 {
		return nil, errors.New("invalid webhook spec")
	}
	webhooks, err := e.client().Rest.GetWebhooks(snowflake.ID(channelID), rest.WithCtx(ctx))
	if err != nil {
		return nil, err
	}
	out := make([]pluginhostlua.WebhookResult, 0, len(webhooks))
	for _, webhook := range webhooks {
		result, convErr := webhookResultFromValue(webhook)
		if convErr != nil {
			continue
		}
		out = append(out, result)
	}
	return out, nil
}

func (e Executor) EditWebhook(
	ctx context.Context,
	spec pluginhostlua.WebhookEditSpec,
) (pluginhostlua.WebhookResult, error) {
	if e.client() == nil {
		return pluginhostlua.WebhookResult{}, errors.New("discord client unavailable")
	}
	if spec.WebhookID == 0 {
		return pluginhostlua.WebhookResult{}, errors.New("invalid webhook spec")
	}
	update := discord.WebhookUpdate{
		Name:      spec.Name,
		ChannelID: snowflakePtr(spec.ChannelID),
		Avatar:    omit.NewNilPtr[discord.Icon](),
	}
	webhook, err := e.client().Rest.UpdateWebhook(snowflake.ID(spec.WebhookID), update, rest.WithCtx(ctx))
	if err != nil {
		return pluginhostlua.WebhookResult{}, errors.New("edit_webhook_error")
	}
	return webhookResultFromValue(webhook)
}

func (e Executor) DeleteWebhook(ctx context.Context, webhookID uint64) error {
	if e.client() == nil {
		return errors.New("discord client unavailable")
	}
	if webhookID == 0 {
		return errors.New("invalid webhook spec")
	}
	return e.client().Rest.DeleteWebhook(snowflake.ID(webhookID), rest.WithCtx(ctx))
}

func (e Executor) ExecuteWebhook(
	ctx context.Context,
	pluginID string,
	spec pluginhostlua.WebhookExecuteSpec,
) (pluginhostlua.MessageResult, error) {
	if e.client() == nil {
		return pluginhostlua.MessageResult{}, errors.New("discord client unavailable")
	}
	if spec.WebhookID == 0 || strings.TrimSpace(spec.Token) == "" {
		return pluginhostlua.MessageResult{}, errors.New("invalid webhook spec")
	}
	msg, err := ParseAutomationMessage(pluginID, spec.Message)
	if err != nil {
		return pluginhostlua.MessageResult{}, err
	}
	webhookMessage := discord.WebhookMessageCreate{
		Content:         msg.Content,
		Embeds:          msg.Embeds,
		Components:      msg.Components,
		AllowedMentions: msg.AllowedMentions,
		Flags:           msg.Flags,
	}
	created, err := e.client().Rest.CreateWebhookMessage(
		snowflake.ID(spec.WebhookID),
		strings.TrimSpace(spec.Token),
		webhookMessage,
		rest.CreateWebhookMessageParams{Wait: true, WithComponents: true},
		rest.WithCtx(ctx),
	)
	if err != nil || created == nil {
		return pluginhostlua.MessageResult{}, errors.New("execute_webhook_error")
	}
	return pluginhostlua.MessageResult{
		MessageID: uint64(created.ID),
		ChannelID: uint64(created.ChannelID),
	}, nil
}

func snowflakePtr(id *uint64) *snowflake.ID {
	if id == nil || *id == 0 {
		return nil
	}
	value := snowflake.ID(*id)
	return &value
}

func channelResult(channel discord.Channel) pluginhostlua.ChannelResult {
	result := pluginhostlua.ChannelResult{
		ID:        uint64(channel.ID()),
		Name:      strings.TrimSpace(channel.Name()),
		Mention:   discord.ChannelMention(channel.ID()),
		Type:      pluginChannelTypeName(channel.Type()),
		CreatedAt: channel.CreatedAt().UTC().Unix(),
	}
	if guildChannel, ok := channel.(discord.GuildChannel); ok {
		if parentID := guildChannel.ParentID(); parentID != nil {
			result.ParentID = uint64(*parentID)
		}
	}
	return result
}

func threadResult(thread discord.GuildThread) pluginhostlua.ThreadResult {
	parentID := uint64(0)
	if parent := thread.ParentID(); parent != nil {
		parentID = uint64(*parent)
	}
	return pluginhostlua.ThreadResult{
		ID:                  uint64(thread.ID()),
		GuildID:             uint64(thread.GuildID()),
		ParentID:            parentID,
		Name:                strings.TrimSpace(thread.Name()),
		Mention:             discord.ChannelMention(thread.ID()),
		Type:                pluginChannelTypeName(thread.Type()),
		Archived:            thread.ThreadMetadata.Archived,
		Locked:              thread.ThreadMetadata.Locked,
		AutoArchiveDuration: int(thread.ThreadMetadata.AutoArchiveDuration),
		CreatedAt:           thread.CreatedAt().UTC().Unix(),
	}
}

func inviteResult(invite discord.Invite) pluginhostlua.InviteResult {
	result := pluginhostlua.InviteResult{
		Code: strings.TrimSpace(invite.Code),
		URL:  strings.TrimSpace(invite.URL()),
	}
	if invite.Channel != nil {
		result.ChannelID = uint64(invite.Channel.ID)
	}
	if invite.Guild != nil {
		result.GuildID = uint64(invite.Guild.ID)
	}
	if invite.Inviter != nil {
		result.InviterID = uint64(invite.Inviter.ID)
	}
	return result
}

func extendedInviteResult(invite discord.ExtendedInvite) pluginhostlua.InviteResult {
	result := inviteResult(invite.Invite)
	result.MaxAge = invite.MaxAge
	result.MaxUses = invite.MaxUses
	result.Uses = invite.Uses
	result.Temporary = invite.Temporary
	result.CreatedAt = invite.CreatedAt.UTC().Unix()
	return result
}

func webhookResult(value discord.IncomingWebhook) pluginhostlua.WebhookResult {
	result := pluginhostlua.WebhookResult{
		ID:        uint64(value.ID()),
		GuildID:   uint64(value.GuildID),
		ChannelID: uint64(value.ChannelID),
		Name:      strings.TrimSpace(value.Name()),
		Token:     strings.TrimSpace(value.Token),
		URL:       strings.TrimSpace(value.URL()),
	}
	if value.ApplicationID != nil {
		result.ApplicationID = uint64(*value.ApplicationID)
	}
	return result
}

func webhookResultFromValue(value discord.Webhook) (pluginhostlua.WebhookResult, error) {
	switch webhook := value.(type) {
	case discord.IncomingWebhook:
		return webhookResult(webhook), nil
	case *discord.IncomingWebhook:
		if webhook == nil {
			return pluginhostlua.WebhookResult{}, errors.New("get_webhook_error")
		}
		return webhookResult(*webhook), nil
	default:
		return pluginhostlua.WebhookResult{}, errors.New("unsupported_webhook_type")
	}
}
