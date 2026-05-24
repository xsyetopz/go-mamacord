package luaplugin

import (
	"context"
	"time"
)

type Discord interface {
	SelfUser(ctx context.Context) (UserResult, error)
	GetUser(ctx context.Context, userID uint64) (UserResult, error)
	GetMember(ctx context.Context, guildID, userID uint64) (MemberResult, error)
	GetGuild(ctx context.Context, guildID uint64) (GuildResult, error)
	GetRole(ctx context.Context, guildID, roleID uint64) (RoleResult, error)
	GetChannel(ctx context.Context, channelID uint64) (ChannelResult, error)
	CreateChannel(ctx context.Context, spec ChannelCreateSpec) (ChannelResult, error)
	EditChannel(ctx context.Context, spec ChannelEditSpec) (ChannelResult, error)
	DeleteChannel(ctx context.Context, channelID uint64) error
	SetChannelOverwrite(ctx context.Context, spec PermissionOverwriteSpec) error
	DeleteChannelOverwrite(ctx context.Context, channelID, overwriteID uint64) error
	GetMessage(ctx context.Context, spec MessageGetSpec) (MessageInfo, error)
	SendDM(ctx context.Context, pluginID string, userID uint64, message EncodedValue) (MessageResult, error)
	SendChannel(ctx context.Context, pluginID string, channelID uint64, message EncodedValue) (MessageResult, error)
	TimeoutMember(ctx context.Context, guildID, userID uint64, until time.Time) error
	SetSlowmode(ctx context.Context, channelID uint64, seconds int) error
	SetNickname(ctx context.Context, guildID, userID uint64, nickname *string) error
	CreateRole(ctx context.Context, spec RoleCreateSpec) (RoleResult, error)
	EditRole(ctx context.Context, spec RoleEditSpec) (RoleResult, error)
	DeleteRole(ctx context.Context, guildID, roleID uint64) error
	AddRole(ctx context.Context, spec RoleMemberSpec) error
	RemoveRole(ctx context.Context, spec RoleMemberSpec) error
	ListMessages(ctx context.Context, spec MessageListSpec) ([]MessageInfo, error)
	DeleteMessage(ctx context.Context, spec MessageDeleteSpec) error
	BulkDeleteMessages(ctx context.Context, channelID uint64, messageIDs []uint64) (int, error)
	PurgeMessages(ctx context.Context, spec PurgeSpec) (int, error)
	CrosspostMessage(ctx context.Context, spec MessageGetSpec) (MessageInfo, error)
	PinMessage(ctx context.Context, spec MessageGetSpec) error
	UnpinMessage(ctx context.Context, spec MessageGetSpec) error
	GetReactions(ctx context.Context, spec ReactionListSpec) ([]UserResult, error)
	AddReaction(ctx context.Context, spec ReactionSpec) error
	RemoveOwnReaction(ctx context.Context, spec ReactionSpec) error
	RemoveUserReaction(ctx context.Context, spec ReactionUserSpec) error
	ClearReactions(ctx context.Context, spec MessageGetSpec) error
	ClearReactionsForEmoji(ctx context.Context, spec ReactionSpec) error
	CreateThreadFromMessage(ctx context.Context, spec ThreadCreateFromMessageSpec) (ThreadResult, error)
	CreateThreadInChannel(ctx context.Context, spec ThreadCreateSpec) (ThreadResult, error)
	JoinThread(ctx context.Context, threadID uint64) error
	LeaveThread(ctx context.Context, threadID uint64) error
	AddThreadMember(ctx context.Context, threadID, userID uint64) error
	RemoveThreadMember(ctx context.Context, threadID, userID uint64) error
	UpdateThread(ctx context.Context, spec ThreadUpdateSpec) (ThreadResult, error)
	CreateInvite(ctx context.Context, spec InviteCreateSpec) (InviteResult, error)
	GetInvite(ctx context.Context, code string) (InviteResult, error)
	DeleteInvite(ctx context.Context, code string) error
	ListChannelInvites(ctx context.Context, channelID uint64) ([]InviteResult, error)
	ListGuildInvites(ctx context.Context, guildID uint64) ([]InviteResult, error)
	CreateWebhook(ctx context.Context, spec WebhookCreateSpec) (WebhookResult, error)
	GetWebhook(ctx context.Context, webhookID uint64) (WebhookResult, error)
	ListChannelWebhooks(ctx context.Context, channelID uint64) ([]WebhookResult, error)
	EditWebhook(ctx context.Context, spec WebhookEditSpec) (WebhookResult, error)
	DeleteWebhook(ctx context.Context, webhookID uint64) error
	ExecuteWebhook(ctx context.Context, pluginID string, spec WebhookExecuteSpec) (MessageResult, error)
	CreateEmoji(ctx context.Context, spec EmojiCreateSpec) (EmojiResult, error)
	EditEmoji(ctx context.Context, spec EmojiEditSpec) (EmojiResult, error)
	DeleteEmoji(ctx context.Context, spec EmojiDeleteSpec) error
	CreateSticker(ctx context.Context, spec StickerCreateSpec) (StickerResult, error)
	EditSticker(ctx context.Context, spec StickerEditSpec) (StickerResult, error)
	DeleteSticker(ctx context.Context, spec StickerDeleteSpec) error
}

type Bridge struct {
	Discord Discord
}
