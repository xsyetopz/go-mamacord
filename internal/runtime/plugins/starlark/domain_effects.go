package starlark

import (
	"errors"
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

func domainEffectAPI() starlarkgo.StringDict {
	return starlarkgo.StringDict{
		"send_channel":    starlarkgo.NewBuiltin("send_channel", builtinSendChannel),
		"send_dm":         starlarkgo.NewBuiltin("send_dm", builtinSendDM),
		"timeout_member":  starlarkgo.NewBuiltin("timeout_member", builtinTimeoutMember),
		"set_slowmode":    starlarkgo.NewBuiltin("set_slowmode", builtinSetSlowmode),
		"set_nickname":    starlarkgo.NewBuiltin("set_nickname", builtinSetNickname),
		"purge_messages":  starlarkgo.NewBuiltin("purge_messages", builtinPurgeMessages),
		"create_role":     starlarkgo.NewBuiltin("create_role", builtinCreateRole),
		"edit_role":       starlarkgo.NewBuiltin("edit_role", builtinEditRole),
		"delete_role":     starlarkgo.NewBuiltin("delete_role", builtinDeleteRole),
		"add_role":        starlarkgo.NewBuiltin("add_role", memberRoleBuiltin(true)),
		"remove_role":     starlarkgo.NewBuiltin("remove_role", memberRoleBuiltin(false)),
		"create_emoji":    starlarkgo.NewBuiltin("create_emoji", builtinCreateEmoji),
		"edit_emoji":      starlarkgo.NewBuiltin("edit_emoji", builtinEditEmoji),
		"delete_emoji":    starlarkgo.NewBuiltin("delete_emoji", builtinDeleteEmoji),
		"create_sticker":  starlarkgo.NewBuiltin("create_sticker", builtinCreateSticker),
		"edit_sticker":    starlarkgo.NewBuiltin("edit_sticker", builtinEditSticker),
		"delete_sticker":  starlarkgo.NewBuiltin("delete_sticker", builtinDeleteSticker),
		"set_timezone":    starlarkgo.NewBuiltin("set_timezone", builtinSetTimezone),
		"clear_timezone":  starlarkgo.NewBuiltin("clear_timezone", builtinClearTimezone),
		"create_checkin":  starlarkgo.NewBuiltin("create_checkin", builtinCreateCheckIn),
		"create_reminder": starlarkgo.NewBuiltin("create_reminder", builtinCreateReminder),
		"delete_reminder": starlarkgo.NewBuiltin("delete_reminder", builtinDeleteReminder),
		"create_warning":  starlarkgo.NewBuiltin("create_warning", builtinCreateWarning),
		"delete_warning":  starlarkgo.NewBuiltin("delete_warning", builtinDeleteWarning),
		"append_audit":    starlarkgo.NewBuiltin("append_audit", builtinAppendAudit),
	}
}

func domainEffect(operation contract.Operation) starlarkgo.Value {
	return effectValue(effectDomain, operation)
}

func builtinSendChannel(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var channelID, content string
	if err := starlarkgo.UnpackArgs("send_channel", args, kwargs, "channel_id", &channelID, "content", &content); err != nil {
		return nil, err
	}
	return domainEffect(&contract.SendChannelOperation{ChannelID: channelID, Message: contract.Message{Content: content}}), nil
}
func builtinSendDM(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var userID, content string
	if err := starlarkgo.UnpackArgs("send_dm", args, kwargs, "user_id", &userID, "content", &content); err != nil {
		return nil, err
	}
	return domainEffect(&contract.SendDMOperation{UserID: userID, Message: contract.Message{Content: content}}), nil
}
func builtinTimeoutMember(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var userID string
	var untilUnix int64
	if err := starlarkgo.UnpackArgs("timeout_member", args, kwargs, "user_id", &userID, "until_unix", &untilUnix); err != nil {
		return nil, err
	}
	return domainEffect(&contract.TimeoutMemberOperation{UserID: userID, UntilUnix: untilUnix}), nil
}
func builtinSetSlowmode(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var channelID string
	var seconds int
	if err := starlarkgo.UnpackArgs("set_slowmode", args, kwargs, "channel_id", &channelID, "seconds", &seconds); err != nil {
		return nil, err
	}
	return domainEffect(&contract.SetSlowmodeOperation{ChannelID: channelID, Seconds: seconds}), nil
}
func builtinSetNickname(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var userID string
	var nickname starlarkgo.Value = starlarkgo.None
	if err := starlarkgo.UnpackArgs("set_nickname", args, kwargs, "user_id", &userID, "nickname?", &nickname); err != nil {
		return nil, err
	}
	text := ""
	if nickname != starlarkgo.None {
		var ok bool
		text, ok = starlarkgo.AsString(nickname)
		if !ok {
			return nil, errors.New("nickname must be string or None")
		}
	}
	return domainEffect(&contract.SetNicknameOperation{UserID: userID, Nickname: contract.OptionalString{Set: true, Value: text}}), nil
}
func builtinPurgeMessages(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var channelID, mode, anchor string
	var count int
	if err := starlarkgo.UnpackArgs("purge_messages", args, kwargs, "channel_id", &channelID, "mode", &mode, "count", &count, "anchor_message_id?", &anchor); err != nil {
		return nil, err
	}
	return domainEffect(&contract.PurgeMessagesOperation{ChannelID: channelID, Mode: contract.PurgeMode(mode), AnchorMessageID: anchor, Count: count}), nil
}

func builtinCreateRole(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var name string
	var color, hoist, mentionable starlarkgo.Value
	if err := starlarkgo.UnpackArgs("create_role", args, kwargs, "name", &name, "color?", &color, "hoist?", &hoist, "mentionable?", &mentionable); err != nil {
		return nil, err
	}
	colorValue, err := optionalInt(color)
	if err != nil {
		return nil, err
	}
	hoistValue, err := optionalBool(hoist)
	if err != nil {
		return nil, err
	}
	mentionableValue, err := optionalBool(mentionable)
	if err != nil {
		return nil, err
	}
	return domainEffect(&contract.CreateRoleOperation{Name: name, Color: colorValue, Hoist: hoistValue, Mentionable: mentionableValue}), nil
}
func builtinEditRole(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var roleID string
	var name, color, hoist, mentionable starlarkgo.Value
	if err := starlarkgo.UnpackArgs("edit_role", args, kwargs, "role_id", &roleID, "name?", &name, "color?", &color, "hoist?", &hoist, "mentionable?", &mentionable); err != nil {
		return nil, err
	}
	nameValue, err := optionalString(name)
	if err != nil {
		return nil, err
	}
	colorValue, err := optionalInt(color)
	if err != nil {
		return nil, err
	}
	hoistValue, err := optionalBool(hoist)
	if err != nil {
		return nil, err
	}
	mentionableValue, err := optionalBool(mentionable)
	if err != nil {
		return nil, err
	}
	return domainEffect(&contract.EditRoleOperation{RoleID: roleID, Name: nameValue, Color: colorValue, Hoist: hoistValue, Mentionable: mentionableValue}), nil
}
func builtinDeleteRole(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var roleID string
	if err := starlarkgo.UnpackArgs("delete_role", args, kwargs, "role_id", &roleID); err != nil {
		return nil, err
	}
	return domainEffect(&contract.DeleteRoleOperation{RoleID: roleID}), nil
}
func memberRoleBuiltin(add bool) builtinFunc {
	return func(_ *starlarkgo.Thread, builtin *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
		var userID, roleID string
		if err := starlarkgo.UnpackArgs(builtin.Name(), args, kwargs, "user_id", &userID, "role_id", &roleID); err != nil {
			return nil, err
		}
		return domainEffect(&contract.MemberRoleOperation{UserID: userID, RoleID: roleID, Add: add}), nil
	}
}

func builtinCreateEmoji(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var name, attachmentID string
	if err := starlarkgo.UnpackArgs("create_emoji", args, kwargs, "name", &name, "attachment_id", &attachmentID); err != nil {
		return nil, err
	}
	return domainEffect(&contract.CreateEmojiOperation{Name: name, AttachmentID: attachmentID}), nil
}
func builtinEditEmoji(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var emoji, name string
	if err := starlarkgo.UnpackArgs("edit_emoji", args, kwargs, "emoji", &emoji, "name", &name); err != nil {
		return nil, err
	}
	return domainEffect(&contract.EditEmojiOperation{Emoji: emoji, Name: name}), nil
}
func builtinDeleteEmoji(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var emoji string
	if err := starlarkgo.UnpackArgs("delete_emoji", args, kwargs, "emoji", &emoji); err != nil {
		return nil, err
	}
	return domainEffect(&contract.DeleteEmojiOperation{Emoji: emoji}), nil
}
func builtinCreateSticker(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var name, description, emojiTag, attachmentID string
	if err := starlarkgo.UnpackArgs("create_sticker", args, kwargs, "name", &name, "description", &description, "emoji_tag", &emojiTag, "attachment_id", &attachmentID); err != nil {
		return nil, err
	}
	return domainEffect(&contract.CreateStickerOperation{Name: name, Description: description, EmojiTag: emojiTag, AttachmentID: attachmentID}), nil
}
func builtinEditSticker(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var stickerID string
	var name, description, emojiTag starlarkgo.Value
	if err := starlarkgo.UnpackArgs("edit_sticker", args, kwargs, "sticker_id", &stickerID, "name?", &name, "description?", &description, "emoji_tag?", &emojiTag); err != nil {
		return nil, err
	}
	nameValue, err := optionalString(name)
	if err != nil {
		return nil, err
	}
	descriptionValue, err := optionalString(description)
	if err != nil {
		return nil, err
	}
	tagValue, err := optionalString(emojiTag)
	if err != nil {
		return nil, err
	}
	return domainEffect(&contract.EditStickerOperation{StickerID: stickerID, Name: nameValue, Description: descriptionValue, EmojiTag: tagValue}), nil
}
func builtinDeleteSticker(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var stickerID string
	if err := starlarkgo.UnpackArgs("delete_sticker", args, kwargs, "sticker_id", &stickerID); err != nil {
		return nil, err
	}
	return domainEffect(&contract.DeleteStickerOperation{StickerID: stickerID}), nil
}

func builtinSetTimezone(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var timezone string
	if err := starlarkgo.UnpackArgs("set_timezone", args, kwargs, "timezone", &timezone); err != nil {
		return nil, err
	}
	return domainEffect(&contract.SetTimezoneOperation{Timezone: timezone}), nil
}
func builtinClearTimezone(_ *starlarkgo.Thread, builtin *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	if err := starlarkgo.UnpackArgs(builtin.Name(), args, kwargs); err != nil {
		return nil, err
	}
	return domainEffect(&contract.ClearTimezoneOperation{}), nil
}
func builtinCreateCheckIn(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var mood int
	var createdAt int64
	if err := starlarkgo.UnpackArgs("create_checkin", args, kwargs, "mood", &mood, "created_at", &createdAt); err != nil {
		return nil, err
	}
	return domainEffect(&contract.CreateCheckInOperation{Mood: mood, CreatedAt: createdAt}), nil
}
func builtinCreateReminder(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var reminderID, schedule, kind, note, delivery, channelID string
	var nextRunAt int64
	if err := starlarkgo.UnpackArgs("create_reminder", args, kwargs, "reminder_id", &reminderID, "schedule", &schedule, "kind", &kind, "next_run_at", &nextRunAt, "note?", &note, "delivery?", &delivery, "channel_id?", &channelID); err != nil {
		return nil, err
	}
	if delivery == "" {
		delivery = "dm"
	}
	return domainEffect(&contract.CreateReminderOperation{ReminderID: reminderID, Schedule: schedule, Kind: kind, Note: note, Delivery: delivery, ChannelID: channelID, NextRunAt: nextRunAt}), nil
}
func builtinDeleteReminder(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var id string
	if err := starlarkgo.UnpackArgs("delete_reminder", args, kwargs, "reminder_id", &id); err != nil {
		return nil, err
	}
	return domainEffect(&contract.DeleteReminderOperation{ReminderID: id}), nil
}
func builtinCreateWarning(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var userID, reason string
	var createdAt int64
	if err := starlarkgo.UnpackArgs("create_warning", args, kwargs, "user_id", &userID, "reason", &reason, "created_at", &createdAt); err != nil {
		return nil, err
	}
	return domainEffect(&contract.CreateWarningOperation{UserID: userID, Reason: reason, CreatedAt: createdAt}), nil
}
func builtinDeleteWarning(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var warningID, userID string
	if err := starlarkgo.UnpackArgs("delete_warning", args, kwargs, "warning_id", &warningID, "target_user_id", &userID); err != nil {
		return nil, err
	}
	return domainEffect(&contract.DeleteWarningOperation{WarningID: warningID, TargetUserID: userID}), nil
}
func builtinAppendAudit(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var action, targetType, targetID string
	var createdAt int64
	var metadataValue starlarkgo.Value
	if err := starlarkgo.UnpackArgs("append_audit", args, kwargs, "action", &action, "created_at", &createdAt, "target_type?", &targetType, "target_id?", &targetID, "metadata?", &metadataValue); err != nil {
		return nil, err
	}
	metadata := contract.Value{}
	var err error
	if metadataValue != nil && metadataValue != starlarkgo.None {
		metadata, err = lowerPersistentValue(metadataValue)
		if err != nil {
			return nil, err
		}
	}
	return domainEffect(&contract.AppendAuditOperation{Action: action, TargetType: contract.AuditTargetType(targetType), TargetID: targetID, CreatedAt: createdAt, Metadata: metadata}), nil
}

func optionalString(value starlarkgo.Value) (*string, error) {
	if value == nil || value == starlarkgo.None {
		return nil, nil
	}
	text, ok := starlarkgo.AsString(value)
	if !ok {
		return nil, fmt.Errorf("want string, got %s", value.Type())
	}
	return &text, nil
}
func optionalBool(value starlarkgo.Value) (*bool, error) {
	if value == nil || value == starlarkgo.None {
		return nil, nil
	}
	boolean, ok := value.(starlarkgo.Bool)
	if !ok {
		return nil, fmt.Errorf("want bool, got %s", value.Type())
	}
	result := bool(boolean)
	return &result, nil
}
