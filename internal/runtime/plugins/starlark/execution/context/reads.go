package contextapi

import (
	"errors"
	"fmt"

	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/contract"
	starlarkgo "go.starlark.net/starlark"
)

func (value *contextValue) beginRead(capability contract.Capability) error {
	if value.services.Reader == nil {
		return errors.New("read service is unavailable")
	}
	if !value.services.Allows(capability) {
		return fmt.Errorf("capability %q is not granted", capability)
	}
	if value.calls >= value.maxCalls {
		return fmt.Errorf("host call limit %d exceeded", value.maxCalls)
	}
	value.calls++
	return nil
}
func (value *contextValue) getUser(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	userID := ""
	if value.invocation.Author != nil {
		userID = value.invocation.Author.ID
	}
	if err := starlarkgo.UnpackArgs("context.get_user", args, kwargs, "user_id?", &userID); err != nil {
		return nil, err
	}
	if err := value.beginRead(contract.CapabilityDiscordUsers); err != nil {
		return nil, err
	}
	result, ok, err := value.services.Reader.GetUser(value.context, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return starlarkgo.None, nil
	}
	return userDetailsValue(result)
}
func (value *contextValue) getMember(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	if value.invocation.Guild == nil {
		return nil, errors.New("member read requires guild context")
	}
	userID := ""
	if value.invocation.Author != nil {
		userID = value.invocation.Author.ID
	}
	if err := starlarkgo.UnpackArgs("context.get_member", args, kwargs, "user_id?", &userID); err != nil {
		return nil, err
	}
	if err := value.beginRead(contract.CapabilityDiscordMembers); err != nil {
		return nil, err
	}
	result, ok, err := value.services.Reader.GetMember(value.context, value.invocation.Guild.ID, userID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return starlarkgo.None, nil
	}
	return memberDetailsValue(result)
}
func (value *contextValue) getGuild(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	guildID := ""
	if value.invocation.Guild != nil {
		guildID = value.invocation.Guild.ID
	}
	if err := starlarkgo.UnpackArgs("context.get_guild", args, kwargs, "guild_id?", &guildID); err != nil {
		return nil, err
	}
	if err := value.beginRead(contract.CapabilityDiscordGuilds); err != nil {
		return nil, err
	}
	result, ok, err := value.services.Reader.GetGuild(value.context, guildID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return starlarkgo.None, nil
	}
	return guildDetailsValue(result)
}
func (value *contextValue) normalizeTimezone(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var raw string
	if err := starlarkgo.UnpackArgs("context.normalize_timezone", args, kwargs, "timezone", &raw); err != nil {
		return nil, err
	}
	if err := value.beginRead(contract.CapabilityStorageUserSettings); err != nil {
		return nil, err
	}
	normalized, ok, err := value.services.Reader.NormalizeTimezone(value.context, raw)
	if err != nil {
		return nil, err
	}
	if !ok {
		return starlarkgo.None, nil
	}
	return starlarkgo.String(normalized), nil
}
func (value *contextValue) userSettings(_ *starlarkgo.Thread, builtin *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	if err := starlarkgo.UnpackArgs(builtin.Name(), args, kwargs); err != nil {
		return nil, err
	}
	if value.invocation.Author == nil {
		return nil, errors.New("settings require author")
	}
	if err := value.beginRead(contract.CapabilityStorageUserSettings); err != nil {
		return nil, err
	}
	result, ok, err := value.services.Reader.GetUserSettings(value.context, value.invocation.Author.ID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return starlarkgo.None, nil
	}
	return frozenDict(map[string]starlarkgo.Value{"timezone": starlarkgo.String(result.Timezone), "dm_channel_id": starlarkgo.String(result.DMChannelID), "created_at": starlarkgo.MakeInt64(result.CreatedAt), "updated_at": starlarkgo.MakeInt64(result.UpdatedAt)})
}
func (value *contextValue) listCheckIns(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	limit := 10
	if err := starlarkgo.UnpackArgs("context.list_checkins", args, kwargs, "limit?", &limit); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 25 {
		return nil, errors.New("limit must be between 1 and 25")
	}
	if value.invocation.Author == nil {
		return nil, errors.New("check-ins require author")
	}
	if err := value.beginRead(contract.CapabilityStorageCheckIns); err != nil {
		return nil, err
	}
	items, err := value.services.Reader.ListCheckIns(value.context, value.invocation.Author.ID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]starlarkgo.Value, len(items))
	for i, item := range items {
		out[i], err = frozenDict(map[string]starlarkgo.Value{"id": starlarkgo.String(item.ID), "mood": starlarkgo.MakeInt(item.Mood), "created_at": starlarkgo.MakeInt64(item.CreatedAt)})
		if err != nil {
			return nil, err
		}
	}
	list := starlarkgo.NewList(out)
	list.Freeze()
	return list, nil
}
func (value *contextValue) planReminder(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var schedule string
	if err := starlarkgo.UnpackArgs("context.plan_reminder", args, kwargs, "schedule", &schedule); err != nil {
		return nil, err
	}
	if value.invocation.Author == nil {
		return nil, errors.New("reminder plan requires author")
	}
	if err := value.beginRead(contract.CapabilityStorageReminders); err != nil {
		return nil, err
	}
	plan, ok, err := value.services.Reader.PlanReminder(value.context, value.invocation.Author.ID, schedule, value.invocation.NowUnix)
	if err != nil {
		return nil, err
	}
	if !ok {
		return starlarkgo.None, nil
	}
	return frozenDict(map[string]starlarkgo.Value{"schedule": starlarkgo.String(plan.Schedule), "next_run_at": starlarkgo.MakeInt64(plan.NextRunAt)})
}
func (value *contextValue) listReminders(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	limit := 25
	if err := starlarkgo.UnpackArgs("context.list_reminders", args, kwargs, "limit?", &limit); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 25 {
		return nil, errors.New("limit must be between 1 and 25")
	}
	if value.invocation.Author == nil {
		return nil, errors.New("reminders require author")
	}
	if err := value.beginRead(contract.CapabilityStorageReminders); err != nil {
		return nil, err
	}
	items, err := value.services.Reader.ListReminders(value.context, value.invocation.Author.ID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]starlarkgo.Value, len(items))
	for i, item := range items {
		out[i], err = reminderValue(item)
		if err != nil {
			return nil, err
		}
	}
	list := starlarkgo.NewList(out)
	list.Freeze()
	return list, nil
}
func (value *contextValue) countWarnings(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var userID string
	if err := starlarkgo.UnpackArgs("context.count_warnings", args, kwargs, "user_id", &userID); err != nil {
		return nil, err
	}
	if value.invocation.Guild == nil {
		return nil, errors.New("warnings require guild")
	}
	if err := value.beginRead(contract.CapabilityStorageWarnings); err != nil {
		return nil, err
	}
	count, err := value.services.Reader.CountWarnings(value.context, value.invocation.Guild.ID, userID)
	if err != nil {
		return nil, err
	}
	return starlarkgo.MakeInt(count), nil
}
func (value *contextValue) listWarnings(_ *starlarkgo.Thread, _ *starlarkgo.Builtin, args starlarkgo.Tuple, kwargs []starlarkgo.Tuple) (starlarkgo.Value, error) {
	var userID string
	limit := 25
	if err := starlarkgo.UnpackArgs("context.list_warnings", args, kwargs, "user_id", &userID, "limit?", &limit); err != nil {
		return nil, err
	}
	if limit < 1 || limit > 100 {
		return nil, errors.New("limit must be between 1 and 100")
	}
	if value.invocation.Guild == nil {
		return nil, errors.New("warnings require guild")
	}
	if err := value.beginRead(contract.CapabilityStorageWarnings); err != nil {
		return nil, err
	}
	items, err := value.services.Reader.ListWarnings(value.context, value.invocation.Guild.ID, userID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]starlarkgo.Value, len(items))
	for i, item := range items {
		out[i], err = frozenDict(map[string]starlarkgo.Value{"id": starlarkgo.String(item.ID), "user_id": starlarkgo.String(item.UserID), "moderator_id": starlarkgo.String(item.ModeratorID), "reason": starlarkgo.String(item.Reason), "created_at": starlarkgo.MakeInt64(item.CreatedAt)})
		if err != nil {
			return nil, err
		}
	}
	list := starlarkgo.NewList(out)
	list.Freeze()
	return list, nil
}

func userDetailsValue(value contract.UserDetailsRef) (starlarkgo.Value, error) {
	color := starlarkgo.Value(starlarkgo.None)
	if value.AccentColor != nil {
		color = starlarkgo.MakeInt(*value.AccentColor)
	}
	return frozenDict(map[string]starlarkgo.Value{"id": starlarkgo.String(value.User.ID), "username": starlarkgo.String(value.User.Username), "name": starlarkgo.String(value.User.Name), "mention": starlarkgo.String(value.Mention), "avatar_url": starlarkgo.String(value.AvatarURL), "banner_url": starlarkgo.String(value.BannerURL), "bot": starlarkgo.Bool(value.User.Bot), "system": starlarkgo.Bool(value.User.System), "created_at": starlarkgo.MakeInt64(value.CreatedAt), "accent_color": color})
}
func memberDetailsValue(value contract.MemberDetailsRef) (starlarkgo.Value, error) {
	roles := make([]starlarkgo.Value, len(value.Member.RoleIDs))
	for i, role := range value.Member.RoleIDs {
		roles[i] = starlarkgo.String(role)
	}
	return frozenDict(map[string]starlarkgo.Value{"user_id": starlarkgo.String(value.Member.User.ID), "guild_id": starlarkgo.String(value.Member.GuildID), "joined_at": starlarkgo.MakeInt64(value.JoinedAt), "role_ids": starlarkgo.NewList(roles), "avatar_url": starlarkgo.String(value.AvatarURL), "banner_url": starlarkgo.String(value.BannerURL)})
}
func guildDetailsValue(value contract.GuildDetailsRef) (starlarkgo.Value, error) {
	return frozenDict(map[string]starlarkgo.Value{"id": starlarkgo.String(value.Guild.ID), "name": starlarkgo.String(value.Guild.Name), "owner_id": starlarkgo.String(value.OwnerID), "description": starlarkgo.String(value.Description), "icon_url": starlarkgo.String(value.IconURL), "banner_url": starlarkgo.String(value.BannerURL), "roles_count": starlarkgo.MakeInt(value.RolesCount), "emojis_count": starlarkgo.MakeInt(value.EmojisCount), "stickers_count": starlarkgo.MakeInt(value.StickersCount), "member_count": starlarkgo.MakeInt(value.MemberCount), "channels_count": starlarkgo.MakeInt(value.ChannelsCount), "created_at": starlarkgo.MakeInt64(value.CreatedAt)})
}
func reminderValue(value contract.ReminderRef) (starlarkgo.Value, error) {
	return frozenDict(map[string]starlarkgo.Value{"id": starlarkgo.String(value.ID), "schedule": starlarkgo.String(value.Schedule), "kind": starlarkgo.String(value.Kind), "note": starlarkgo.String(value.Note), "delivery": starlarkgo.String(value.Delivery), "guild_id": starlarkgo.String(value.GuildID), "channel_id": starlarkgo.String(value.ChannelID), "enabled": starlarkgo.Bool(value.Enabled), "next_run_at": starlarkgo.MakeInt64(value.NextRunAt), "last_run_at": starlarkgo.MakeInt64(value.LastRunAt), "failure_count": starlarkgo.MakeInt(value.FailureCount), "created_at": starlarkgo.MakeInt64(value.CreatedAt), "updated_at": starlarkgo.MakeInt64(value.UpdatedAt)})
}
