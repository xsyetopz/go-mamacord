package luaplugin

import (
	"context"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type ChannelCreateSpec struct {
	GuildID   uint64
	Name      string
	Type      string
	Topic     *string
	ParentID  *uint64
	NSFW      *bool
	Slowmode  *int
	Position  *int
	Bitrate   *int
	UserLimit *int
}

type ChannelEditSpec struct {
	ChannelID uint64
	Name      *string
	Topic     *string
	ParentID  *uint64
	NSFW      *bool
	Slowmode  *int
	Position  *int
	Bitrate   *int
	UserLimit *int
}

type PermissionOverwriteSpec struct {
	ChannelID   uint64
	OverwriteID uint64
	TargetType  string
	Allow       int64
	Deny        int64
}

type ThreadResult struct {
	ID                  uint64
	GuildID             uint64
	ParentID            uint64
	Name                string
	Mention             string
	Type                string
	Archived            bool
	Locked              bool
	AutoArchiveDuration int
	CreatedAt           int64
}

type ThreadCreateFromMessageSpec struct {
	ChannelID           uint64
	MessageID           uint64
	Name                string
	AutoArchiveDuration int
	Slowmode            int
}

type ThreadCreateSpec struct {
	ChannelID           uint64
	Name                string
	Type                string
	AutoArchiveDuration int
	Invitable           *bool
}

type ThreadUpdateSpec struct {
	ThreadID            uint64
	Name                *string
	Archived            *bool
	Locked              *bool
	Invitable           *bool
	AutoArchiveDuration *int
	Slowmode            *int
}

type InviteResult struct {
	Code      string
	URL       string
	GuildID   uint64
	ChannelID uint64
	InviterID uint64
	MaxAge    int
	MaxUses   int
	Uses      int
	Temporary bool
	CreatedAt int64
}

type InviteCreateSpec struct {
	ChannelID uint64
	MaxAge    *int
	MaxUses   *int
	Temporary bool
	Unique    bool
}

type WebhookResult struct {
	ID            uint64
	GuildID       uint64
	ChannelID     uint64
	ApplicationID uint64
	Name          string
	Token         string
	URL           string
}

type WebhookCreateSpec struct {
	ChannelID uint64
	Name      string
}

type WebhookEditSpec struct {
	WebhookID uint64
	Name      *string
	ChannelID *uint64
}

type WebhookExecuteSpec struct {
	WebhookID uint64
	Token     string
	Message   EncodedValue
}

func (v *VM) luaDiscordCreateChannel(l *lua.LState) int {
	if !v.perms.Discord.Channels {
		l.RaiseError("permission denied: discord.create_channel")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}

	spec := l.CheckTable(1)
	input := ChannelCreateSpec{
		GuildID:   v.tableGuildID(spec, "guild_id"),
		Name:      strings.TrimSpace(luaStringDefault(spec.RawGetString("name"), "")),
		Type:      strings.TrimSpace(luaStringDefault(spec.RawGetString("type"), "text")),
		Topic:     luaOptionalTrimmedString(spec, "topic"),
		ParentID:  luaOptionalSnowflake(spec, "parent_id"),
		NSFW:      luaOptionalBool(spec, "nsfw"),
		Slowmode:  luaOptionalInt(spec, "slowmode"),
		Position:  luaOptionalInt(spec, "position"),
		Bitrate:   luaOptionalInt(spec, "bitrate"),
		UserLimit: luaOptionalInt(spec, "user_limit"),
	}
	if input.GuildID == 0 || input.Name == "" {
		l.RaiseError("invalid channel spec")
		return 0
	}
	channel, err := v.discord.CreateChannel(v.ctx(), input)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	return pushDiscordValueResult(l, channelMap(channel), "")
}

func (v *VM) luaDiscordEditChannel(l *lua.LState) int {
	if !v.perms.Discord.Channels {
		l.RaiseError("permission denied: discord.edit_channel")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}

	spec := l.CheckTable(1)
	input := ChannelEditSpec{
		ChannelID: luaSnowflake(spec.RawGetString("channel_id"), v.channel),
		Name:      luaOptionalTrimmedString(spec, "name"),
		Topic:     luaOptionalTrimmedString(spec, "topic"),
		ParentID:  luaOptionalSnowflake(spec, "parent_id"),
		NSFW:      luaOptionalBool(spec, "nsfw"),
		Slowmode:  luaOptionalInt(spec, "slowmode"),
		Position:  luaOptionalInt(spec, "position"),
		Bitrate:   luaOptionalInt(spec, "bitrate"),
		UserLimit: luaOptionalInt(spec, "user_limit"),
	}
	if input.ChannelID == 0 {
		l.RaiseError("invalid channel spec")
		return 0
	}
	channel, err := v.discord.EditChannel(v.ctx(), input)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	return pushDiscordValueResult(l, channelMap(channel), "")
}

func (v *VM) luaDiscordDeleteChannel(l *lua.LState) int {
	if !v.perms.Discord.Channels {
		l.RaiseError("permission denied: discord.delete_channel")
		return 0
	}
	if v.discord == nil {
		return pushDiscordBoolResult(l, false, "discord unavailable")
	}
	channelID := luaSnowflake(l.CheckTable(1).RawGetString("channel_id"), v.channel)
	if channelID == 0 {
		l.RaiseError("invalid channel spec")
		return 0
	}
	if err := v.discord.DeleteChannel(v.ctx(), channelID); err != nil {
		return pushDiscordBoolResult(l, false, err.Error())
	}
	return pushDiscordBoolResult(l, true, "")
}

func (v *VM) luaDiscordSetChannelOverwrite(l *lua.LState) int {
	if !v.perms.Discord.Channels {
		l.RaiseError("permission denied: discord.set_overwrite")
		return 0
	}
	if v.discord == nil {
		return pushDiscordBoolResult(l, false, "discord unavailable")
	}
	spec := l.CheckTable(1)
	input := PermissionOverwriteSpec{
		ChannelID:   luaSnowflake(spec.RawGetString("channel_id"), v.channel),
		OverwriteID: luaSnowflake(spec.RawGetString("target_id"), 0),
		TargetType:  strings.TrimSpace(luaStringDefault(spec.RawGetString("target_type"), "")),
		Allow:       int64(luaIntDefault(spec.RawGetString("allow"), 0)),
		Deny:        int64(luaIntDefault(spec.RawGetString("deny"), 0)),
	}
	if input.ChannelID == 0 || input.OverwriteID == 0 || input.TargetType == "" {
		l.RaiseError("invalid overwrite spec")
		return 0
	}
	if err := v.discord.SetChannelOverwrite(v.ctx(), input); err != nil {
		return pushDiscordBoolResult(l, false, err.Error())
	}
	return pushDiscordBoolResult(l, true, "")
}

func (v *VM) luaDiscordDeleteChannelOverwrite(l *lua.LState) int {
	if !v.perms.Discord.Channels {
		l.RaiseError("permission denied: discord.delete_overwrite")
		return 0
	}
	if v.discord == nil {
		return pushDiscordBoolResult(l, false, "discord unavailable")
	}
	spec := l.CheckTable(1)
	channelID := luaSnowflake(spec.RawGetString("channel_id"), v.channel)
	overwriteID := luaSnowflake(spec.RawGetString("target_id"), 0)
	if channelID == 0 || overwriteID == 0 {
		l.RaiseError("invalid overwrite spec")
		return 0
	}
	if err := v.discord.DeleteChannelOverwrite(v.ctx(), channelID, overwriteID); err != nil {
		return pushDiscordBoolResult(l, false, err.Error())
	}
	return pushDiscordBoolResult(l, true, "")
}

func (v *VM) luaDiscordCreateThreadFromMessage(l *lua.LState) int {
	if !v.perms.Discord.Threads {
		l.RaiseError("permission denied: discord.create_thread_from_message")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}
	spec := l.CheckTable(1)
	input := ThreadCreateFromMessageSpec{
		ChannelID:           luaSnowflake(spec.RawGetString("channel_id"), v.channel),
		MessageID:           luaSnowflake(spec.RawGetString("message_id"), 0),
		Name:                strings.TrimSpace(luaStringDefault(spec.RawGetString("name"), "")),
		AutoArchiveDuration: int(luaIntDefault(spec.RawGetString("auto_archive_duration"), 0)),
		Slowmode:            int(luaIntDefault(spec.RawGetString("slowmode"), 0)),
	}
	if input.ChannelID == 0 || input.MessageID == 0 || input.Name == "" {
		l.RaiseError("invalid thread spec")
		return 0
	}
	thread, err := v.discord.CreateThreadFromMessage(v.ctx(), input)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	return pushDiscordValueResult(l, threadMap(thread), "")
}

func (v *VM) luaDiscordCreateThreadInChannel(l *lua.LState) int {
	if !v.perms.Discord.Threads {
		l.RaiseError("permission denied: discord.create_thread")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}
	spec := l.CheckTable(1)
	input := ThreadCreateSpec{
		ChannelID:           luaSnowflake(spec.RawGetString("channel_id"), v.channel),
		Name:                strings.TrimSpace(luaStringDefault(spec.RawGetString("name"), "")),
		Type:                strings.TrimSpace(luaStringDefault(spec.RawGetString("type"), "public")),
		AutoArchiveDuration: int(luaIntDefault(spec.RawGetString("auto_archive_duration"), 0)),
		Invitable:           luaOptionalBool(spec, "invitable"),
	}
	if input.ChannelID == 0 || input.Name == "" {
		l.RaiseError("invalid thread spec")
		return 0
	}
	thread, err := v.discord.CreateThreadInChannel(v.ctx(), input)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	return pushDiscordValueResult(l, threadMap(thread), "")
}

func (v *VM) luaDiscordJoinThread(l *lua.LState) int {
	return v.luaDiscordThreadBoolMutation(l, "discord.join_thread", v.discord.JoinThread)
}
func (v *VM) luaDiscordLeaveThread(l *lua.LState) int {
	return v.luaDiscordThreadBoolMutation(l, "discord.leave_thread", v.discord.LeaveThread)
}
func (v *VM) luaDiscordThreadBoolMutation(l *lua.LState, permName string, run func(ctx context.Context, threadID uint64) error) int {
	if !v.perms.Discord.Threads {
		l.RaiseError("permission denied: %s", permName)
		return 0
	}
	if v.discord == nil {
		return pushDiscordBoolResult(l, false, "discord unavailable")
	}
	threadID := luaSnowflake(l.CheckTable(1).RawGetString("thread_id"), v.channel)
	if threadID == 0 {
		l.RaiseError("invalid thread spec")
		return 0
	}
	if err := run(v.ctx(), threadID); err != nil {
		return pushDiscordBoolResult(l, false, err.Error())
	}
	return pushDiscordBoolResult(l, true, "")
}

func (v *VM) luaDiscordAddThreadMember(l *lua.LState) int {
	return v.luaDiscordThreadMemberMutation(l, "discord.add_thread_member", v.discord.AddThreadMember)
}
func (v *VM) luaDiscordRemoveThreadMember(l *lua.LState) int {
	return v.luaDiscordThreadMemberMutation(l, "discord.remove_thread_member", v.discord.RemoveThreadMember)
}

func (v *VM) luaDiscordThreadMemberMutation(l *lua.LState, permName string, run func(ctx context.Context, threadID, userID uint64) error) int {
	if !v.perms.Discord.Threads {
		l.RaiseError("permission denied: %s", permName)
		return 0
	}
	if v.discord == nil {
		return pushDiscordBoolResult(l, false, "discord unavailable")
	}
	spec := l.CheckTable(1)
	threadID := luaSnowflake(spec.RawGetString("thread_id"), v.channel)
	userID := luaSnowflake(spec.RawGetString("user_id"), v.userID)
	if threadID == 0 || userID == 0 {
		l.RaiseError("invalid thread member spec")
		return 0
	}
	if err := run(v.ctx(), threadID, userID); err != nil {
		return pushDiscordBoolResult(l, false, err.Error())
	}
	return pushDiscordBoolResult(l, true, "")
}

func (v *VM) luaDiscordUpdateThread(l *lua.LState) int {
	if !v.perms.Discord.Threads {
		l.RaiseError("permission denied: discord.update_thread")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}
	spec := l.CheckTable(1)
	input := ThreadUpdateSpec{
		ThreadID:            luaSnowflake(spec.RawGetString("thread_id"), v.channel),
		Name:                luaOptionalTrimmedString(spec, "name"),
		Archived:            luaOptionalBool(spec, "archived"),
		Locked:              luaOptionalBool(spec, "locked"),
		Invitable:           luaOptionalBool(spec, "invitable"),
		AutoArchiveDuration: luaOptionalInt(spec, "auto_archive_duration"),
		Slowmode:            luaOptionalInt(spec, "slowmode"),
	}
	if input.ThreadID == 0 {
		l.RaiseError("invalid thread spec")
		return 0
	}
	thread, err := v.discord.UpdateThread(v.ctx(), input)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	return pushDiscordValueResult(l, threadMap(thread), "")
}

func (v *VM) luaDiscordCreateInvite(l *lua.LState) int {
	if !v.perms.Discord.Invites {
		l.RaiseError("permission denied: discord.create_invite")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}
	spec := l.CheckTable(1)
	input := InviteCreateSpec{
		ChannelID: luaSnowflake(spec.RawGetString("channel_id"), v.channel),
		MaxAge:    luaOptionalInt(spec, "max_age"),
		MaxUses:   luaOptionalInt(spec, "max_uses"),
		Temporary: luaBoolValue(spec.RawGetString("temporary"), false),
		Unique:    luaBoolValue(spec.RawGetString("unique"), false),
	}
	if input.ChannelID == 0 {
		l.RaiseError("invalid invite spec")
		return 0
	}
	invite, err := v.discord.CreateInvite(v.ctx(), input)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	return pushDiscordValueResult(l, inviteMap(invite), "")
}

func (v *VM) luaDiscordGetInvite(l *lua.LState) int {
	return v.luaDiscordInviteLookup(l, "discord.get_invite", v.discord.GetInvite)
}

func (v *VM) luaDiscordDeleteInvite(l *lua.LState) int {
	if !v.perms.Discord.Invites {
		l.RaiseError("permission denied: discord.delete_invite")
		return 0
	}
	if v.discord == nil {
		return pushDiscordBoolResult(l, false, "discord unavailable")
	}
	code := strings.TrimSpace(luaStringDefault(l.CheckTable(1).RawGetString("code"), ""))
	if code == "" {
		l.RaiseError("invalid invite spec")
		return 0
	}
	if err := v.discord.DeleteInvite(v.ctx(), code); err != nil {
		return pushDiscordBoolResult(l, false, err.Error())
	}
	return pushDiscordBoolResult(l, true, "")
}

func (v *VM) luaDiscordListChannelInvites(l *lua.LState) int {
	if !v.perms.Discord.Invites {
		l.RaiseError("permission denied: discord.list_channel_invites")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}
	channelID := luaSnowflake(l.CheckTable(1).RawGetString("channel_id"), v.channel)
	if channelID == 0 {
		l.RaiseError("invalid invite spec")
		return 0
	}
	items, err := v.discord.ListChannelInvites(v.ctx(), channelID)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	return pushDiscordValueResult(l, inviteSlice(items), "")
}

func (v *VM) luaDiscordListGuildInvites(l *lua.LState) int {
	if !v.perms.Discord.Invites {
		l.RaiseError("permission denied: discord.list_guild_invites")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}
	guildID := v.tableGuildID(l.CheckTable(1), "guild_id")
	if guildID == 0 {
		l.RaiseError("invalid invite spec")
		return 0
	}
	items, err := v.discord.ListGuildInvites(v.ctx(), guildID)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	return pushDiscordValueResult(l, inviteSlice(items), "")
}

func (v *VM) luaDiscordInviteLookup(l *lua.LState, permName string, run func(ctx context.Context, code string) (InviteResult, error)) int {
	if !v.perms.Discord.Invites {
		l.RaiseError("permission denied: %s", permName)
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}
	code := strings.TrimSpace(luaStringDefault(l.CheckTable(1).RawGetString("code"), ""))
	if code == "" {
		l.RaiseError("invalid invite spec")
		return 0
	}
	invite, err := run(v.ctx(), code)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	return pushDiscordValueResult(l, inviteMap(invite), "")
}

func (v *VM) luaDiscordCreateWebhook(l *lua.LState) int {
	if !v.perms.Discord.Webhooks {
		l.RaiseError("permission denied: discord.create_webhook")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}
	spec := l.CheckTable(1)
	input := WebhookCreateSpec{
		ChannelID: luaSnowflake(spec.RawGetString("channel_id"), v.channel),
		Name:      strings.TrimSpace(luaStringDefault(spec.RawGetString("name"), "")),
	}
	if input.ChannelID == 0 || input.Name == "" {
		l.RaiseError("invalid webhook spec")
		return 0
	}
	webhook, err := v.discord.CreateWebhook(v.ctx(), input)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	return pushDiscordValueResult(l, webhookMap(webhook), "")
}

func (v *VM) luaDiscordGetWebhook(l *lua.LState) int {
	if !v.perms.Discord.Webhooks {
		l.RaiseError("permission denied: discord.get_webhook")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}
	webhookID := luaSnowflake(l.CheckTable(1).RawGetString("webhook_id"), 0)
	if webhookID == 0 {
		l.RaiseError("invalid webhook spec")
		return 0
	}
	webhook, err := v.discord.GetWebhook(v.ctx(), webhookID)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	return pushDiscordValueResult(l, webhookMap(webhook), "")
}

func (v *VM) luaDiscordListChannelWebhooks(l *lua.LState) int {
	if !v.perms.Discord.Webhooks {
		l.RaiseError("permission denied: discord.list_channel_webhooks")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}
	channelID := luaSnowflake(l.CheckTable(1).RawGetString("channel_id"), v.channel)
	if channelID == 0 {
		l.RaiseError("invalid webhook spec")
		return 0
	}
	items, err := v.discord.ListChannelWebhooks(v.ctx(), channelID)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	out := make([]any, 0, len(items))
	for _, item := range items {
		out = append(out, webhookMap(item))
	}
	return pushDiscordValueResult(l, out, "")
}

func (v *VM) luaDiscordEditWebhook(l *lua.LState) int {
	if !v.perms.Discord.Webhooks {
		l.RaiseError("permission denied: discord.edit_webhook")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}
	spec := l.CheckTable(1)
	input := WebhookEditSpec{
		WebhookID: luaSnowflake(spec.RawGetString("webhook_id"), 0),
		Name:      luaOptionalTrimmedString(spec, "name"),
		ChannelID: luaOptionalSnowflake(spec, "channel_id"),
	}
	if input.WebhookID == 0 {
		l.RaiseError("invalid webhook spec")
		return 0
	}
	webhook, err := v.discord.EditWebhook(v.ctx(), input)
	if err != nil {
		return pushDiscordValueResult(l, nil, err.Error())
	}
	return pushDiscordValueResult(l, webhookMap(webhook), "")
}

func (v *VM) luaDiscordDeleteWebhook(l *lua.LState) int {
	if !v.perms.Discord.Webhooks {
		l.RaiseError("permission denied: discord.delete_webhook")
		return 0
	}
	if v.discord == nil {
		return pushDiscordBoolResult(l, false, "discord unavailable")
	}
	webhookID := luaSnowflake(l.CheckTable(1).RawGetString("webhook_id"), 0)
	if webhookID == 0 {
		l.RaiseError("invalid webhook spec")
		return 0
	}
	if err := v.discord.DeleteWebhook(v.ctx(), webhookID); err != nil {
		return pushDiscordBoolResult(l, false, err.Error())
	}
	return pushDiscordBoolResult(l, true, "")
}

func (v *VM) luaDiscordExecuteWebhook(l *lua.LState) int {
	if !v.perms.Discord.Webhooks {
		l.RaiseError("permission denied: discord.execute_webhook")
		return 0
	}
	if v.discord == nil {
		return pushDiscordValueResult(l, nil, "discord unavailable")
	}
	spec := l.CheckTable(1)
	input := WebhookExecuteSpec{
		WebhookID: luaSnowflake(spec.RawGetString("webhook_id"), 0),
		Token:     strings.TrimSpace(luaStringDefault(spec.RawGetString("token"), "")),
	}
	message, _, err := luaToAny(l.CheckAny(2))
	if err != nil {
		l.RaiseError("invalid webhook message")
		return 0
	}
	input.Message, err = EncodeValue(message)
	if err != nil {
		l.RaiseError("invalid webhook message")
		return 0
	}
	if input.WebhookID == 0 || input.Token == "" {
		l.RaiseError("invalid webhook spec")
		return 0
	}
	result, execErr := v.discord.ExecuteWebhook(v.ctx(), v.plugin, input)
	if execErr != nil {
		return pushDiscordValueResult(l, nil, execErr.Error())
	}
	return pushDiscordValueResult(l, map[string]any{
		"message_id": luaUintString(result.MessageID),
		"channel_id": luaUintString(result.ChannelID),
	}, "")
}

func luaOptionalSnowflake(spec *lua.LTable, key string) *uint64 {
	value := luaSnowflake(spec.RawGetString(key), 0)
	if value == 0 {
		return nil
	}
	return &value
}

func threadMap(thread ThreadResult) map[string]any {
	out := map[string]any{
		"id":                    luaUintString(thread.ID),
		"guild_id":              luaUintString(thread.GuildID),
		"parent_id":             luaUintString(thread.ParentID),
		"name":                  thread.Name,
		"mention":               thread.Mention,
		"type":                  thread.Type,
		"archived":              thread.Archived,
		"locked":                thread.Locked,
		"auto_archive_duration": thread.AutoArchiveDuration,
		"created_at":            thread.CreatedAt,
	}
	return out
}

func inviteMap(invite InviteResult) map[string]any {
	return map[string]any{
		"code":       invite.Code,
		"url":        invite.URL,
		"guild_id":   luaUintString(invite.GuildID),
		"channel_id": luaUintString(invite.ChannelID),
		"inviter_id": luaUintString(invite.InviterID),
		"max_age":    invite.MaxAge,
		"max_uses":   invite.MaxUses,
		"uses":       invite.Uses,
		"temporary":  invite.Temporary,
		"created_at": invite.CreatedAt,
	}
}

func inviteSlice(list []InviteResult) []any {
	out := make([]any, 0, len(list))
	for _, item := range list {
		out = append(out, inviteMap(item))
	}
	return out
}

func webhookMap(webhook WebhookResult) map[string]any {
	return map[string]any{
		"id":             luaUintString(webhook.ID),
		"guild_id":       luaUintString(webhook.GuildID),
		"channel_id":     luaUintString(webhook.ChannelID),
		"application_id": luaUintString(webhook.ApplicationID),
		"name":           webhook.Name,
		"token":          webhook.Token,
		"url":            webhook.URL,
	}
}
