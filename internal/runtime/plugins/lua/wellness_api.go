package luaplugin

import (
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	lua "github.com/yuin/gopher-lua"

	"github.com/xsyetopz/go-mamacord/internal/scheduling"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
	"github.com/xsyetopz/go-mamacord/internal/timezone"
)

const defaultPluginListLimit = 25

func (v *VM) luaUserSettingsNormalizeTimezone(l *lua.LState) int {
	raw := strings.TrimSpace(l.CheckString(1))
	_, name, err := timezone.LoadLocation(raw)
	if err != nil {
		l.Push(lua.LNil)
		return 1
	}
	l.Push(lua.LString(name))
	return 1
}

func (v *VM) luaUserSettingsGet(l *lua.LState) int {
	if !v.perms.Storage.UserSettings {
		l.RaiseError("permission denied: storage.user_settings")
		return 0
	}
	if v.store == nil || v.store.UserSettings() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	userID := v.luaUserIDArg(l, 1)
	if userID == 0 {
		l.Push(lua.LNil)
		l.Push(lua.LFalse)
		return 2
	}

	settings, ok, err := v.store.UserSettings().GetUserSettings(v.ctx(), userID)
	if err != nil {
		l.RaiseError("storage error")
		return 0
	}
	if !ok {
		l.Push(lua.LNil)
		l.Push(lua.LFalse)
		return 2
	}

	lv, convErr := anyToLuaValue(l, map[string]any{
		"user_id":       settings.UserID,
		"timezone":      settings.Timezone,
		"dm_channel_id": uint64PtrString(settings.DMChannelID),
		"created_at":    settings.CreatedAt.UTC().Unix(),
		"updated_at":    settings.UpdatedAt.UTC().Unix(),
	}, 0)
	if convErr != nil {
		l.RaiseError("storage decode error")
		return 0
	}
	l.Push(lv)
	l.Push(lua.LTrue)
	return 2
}

func (v *VM) luaUserSettingsSetTimezone(l *lua.LState) int {
	if !v.perms.Storage.UserSettings {
		l.RaiseError("permission denied: storage.user_settings")
		return 0
	}
	if v.store == nil || v.store.UserSettings() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	userID := v.luaUserIDArg(l, 1)
	timezoneName := strings.TrimSpace(l.CheckString(2))
	if userID == 0 || timezoneName == "" {
		l.RaiseError("invalid timezone")
		return 0
	}
	_, normalized, err := timezone.LoadLocation(timezoneName)
	if err != nil {
		l.RaiseError("invalid timezone")
		return 0
	}
	if err := v.store.UserSettings().UpsertUserTimezone(v.ctx(), userID, normalized); err != nil {
		l.RaiseError("storage error")
		return 0
	}
	l.Push(lua.LString(normalized))
	return 1
}

func (v *VM) luaUserSettingsClearTimezone(l *lua.LState) int {
	if !v.perms.Storage.UserSettings {
		l.RaiseError("permission denied: storage.user_settings")
		return 0
	}
	if v.store == nil || v.store.UserSettings() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	userID := v.luaUserIDArg(l, 1)
	if userID == 0 {
		l.RaiseError("invalid user id")
		return 0
	}
	if err := v.store.UserSettings().ClearUserTimezone(v.ctx(), userID); err != nil {
		l.RaiseError("storage error")
		return 0
	}
	l.Push(lua.LTrue)
	return 1
}

func (v *VM) luaCheckInsCreate(l *lua.LState) int {
	if !v.perms.Storage.CheckIns {
		l.RaiseError("permission denied: storage.checkins")
		return 0
	}
	if v.store == nil || v.store.CheckIns() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	spec := l.CheckTable(1)
	userID := v.tableUserID(spec, "user_id")
	mood := int(luaIntDefault(spec.RawGetString("mood"), 0))
	createdAt := unixTimeFromLua(spec.RawGetString("created_at"))
	if userID == 0 || mood < 1 || mood > 5 {
		l.RaiseError("invalid checkin")
		return 0
	}

	entry := store.CheckIn{
		ID:        uuid.NewString(),
		UserID:    userID,
		Mood:      mood,
		CreatedAt: createdAt,
	}
	if err := v.store.CheckIns().CreateCheckIn(v.ctx(), entry); err != nil {
		l.RaiseError("storage error")
		return 0
	}

	lv, convErr := anyToLuaValue(l, map[string]any{
		"id":         entry.ID,
		"user_id":    entry.UserID,
		"mood":       entry.Mood,
		"created_at": entry.CreatedAt.UTC().Unix(),
	}, 0)
	if convErr != nil {
		l.RaiseError("storage decode error")
		return 0
	}
	l.Push(lv)
	return 1
}

func (v *VM) luaCheckInsList(l *lua.LState) int {
	if !v.perms.Storage.CheckIns {
		l.RaiseError("permission denied: storage.checkins")
		return 0
	}
	if v.store == nil || v.store.CheckIns() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	userID := v.luaUserIDArg(l, 1)
	limit := l.OptInt(2, 10)
	if userID == 0 {
		l.Push(l.NewTable())
		return 1
	}
	items, err := v.store.CheckIns().ListCheckIns(v.ctx(), userID, limit)
	if err != nil {
		l.RaiseError("storage error")
		return 0
	}

	out := l.NewTable()
	for idx, item := range items {
		value, convErr := anyToLuaValue(l, map[string]any{
			"id":         item.ID,
			"user_id":    item.UserID,
			"mood":       item.Mood,
			"created_at": item.CreatedAt.UTC().Unix(),
		}, 0)
		if convErr != nil {
			l.RaiseError("storage decode error")
			return 0
		}
		out.RawSetInt(idx+1, value)
	}

	l.Push(out)
	return 1
}

func (v *VM) luaRemindersPlan(l *lua.LState) int {
	if !v.perms.Storage.Reminders {
		l.RaiseError("permission denied: storage.reminders")
		return 0
	}
	spec := l.CheckTable(1)
	userID := v.tableUserID(spec, "user_id")
	scheduleText := strings.TrimSpace(luaStringDefault(spec.RawGetString("schedule"), ""))
	if userID == 0 || scheduleText == "" {
		l.Push(lua.LNil)
		return 1
	}

	schedule, err := scheduling.ParseSchedule(scheduleText)
	if err != nil {
		l.Push(lua.LNil)
		return 1
	}

	nextRunAt := schedule.Next(time.Now().UTC(), v.userLocation(userID))
	plan, convErr := anyToLuaValue(l, map[string]any{
		"schedule":    schedule.Spec(),
		"next_run_at": nextRunAt.UTC().Unix(),
	}, 0)
	if convErr != nil {
		l.RaiseError("storage decode error")
		return 0
	}
	l.Push(plan)
	return 1
}

func (v *VM) luaRemindersCreate(l *lua.LState) int {
	if !v.perms.Storage.Reminders {
		l.RaiseError("permission denied: storage.reminders")
		return 0
	}
	if v.store == nil || v.store.Reminders() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	spec := l.CheckTable(1)
	reminder, ok := v.reminderFromLua(spec)
	if !ok {
		l.Push(lua.LNil)
		return 1
	}
	if err := v.store.Reminders().CreateReminder(v.ctx(), reminder); err != nil {
		l.RaiseError("storage error")
		return 0
	}

	lv, convErr := anyToLuaValue(l, reminderMap(reminder), 0)
	if convErr != nil {
		l.RaiseError("storage decode error")
		return 0
	}
	l.Push(lv)
	return 1
}

func (v *VM) luaRemindersList(l *lua.LState) int {
	if !v.perms.Storage.Reminders {
		l.RaiseError("permission denied: storage.reminders")
		return 0
	}
	if v.store == nil || v.store.Reminders() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	userID := v.luaUserIDArg(l, 1)
	limit := l.OptInt(2, defaultPluginListLimit)
	if userID == 0 {
		l.Push(l.NewTable())
		return 1
	}

	items, err := v.store.Reminders().ListReminders(v.ctx(), userID, limit)
	if err != nil {
		l.RaiseError("storage error")
		return 0
	}

	out := l.NewTable()
	for idx, item := range items {
		value, convErr := anyToLuaValue(l, reminderMap(item), 0)
		if convErr != nil {
			l.RaiseError("storage decode error")
			return 0
		}
		out.RawSetInt(idx+1, value)
	}
	l.Push(out)
	return 1
}

func (v *VM) luaRemindersDelete(l *lua.LState) int {
	if !v.perms.Storage.Reminders {
		l.RaiseError("permission denied: storage.reminders")
		return 0
	}
	if v.store == nil || v.store.Reminders() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	userID := v.luaUserIDArg(l, 1)
	reminderID := strings.TrimSpace(l.CheckString(2))
	if userID == 0 || reminderID == "" {
		l.Push(lua.LFalse)
		return 1
	}

	deleted, err := v.store.Reminders().DeleteReminder(v.ctx(), userID, reminderID)
	if err != nil {
		l.RaiseError("storage error")
		return 0
	}
	if deleted {
		l.Push(lua.LTrue)
		return 1
	}
	l.Push(lua.LFalse)
	return 1
}

func (v *VM) reminderFromLua(spec *lua.LTable) (store.Reminder, bool) {
	userID := v.tableUserID(spec, "user_id")
	scheduleText := strings.TrimSpace(luaStringDefault(spec.RawGetString("schedule"), ""))
	kind := strings.TrimSpace(luaStringDefault(spec.RawGetString("kind"), ""))
	if userID == 0 || scheduleText == "" || kind == "" {
		return store.Reminder{}, false
	}

	schedule, err := scheduling.ParseSchedule(scheduleText)
	if err != nil {
		return store.Reminder{}, false
	}

	now := time.Now().UTC()
	nextRunAt := schedule.Next(now, v.userLocation(userID))
	delivery := store.ReminderDelivery(strings.ToLower(strings.TrimSpace(luaStringDefault(spec.RawGetString("delivery"), string(store.ReminderDeliveryDM)))))
	if delivery != store.ReminderDeliveryChannel {
		delivery = store.ReminderDeliveryDM
	}

	reminder := store.Reminder{
		ID:        uuid.NewString(),
		UserID:    userID,
		Schedule:  schedule.Spec(),
		Kind:      kind,
		Note:      strings.TrimSpace(luaStringDefault(spec.RawGetString("note"), "")),
		Delivery:  delivery,
		GuildID:   luaSnowflakePtr(spec.RawGetString("guild_id")),
		ChannelID: luaSnowflakePtr(spec.RawGetString("channel_id")),
		Enabled:   true,
		NextRunAt: nextRunAt,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if delivery == store.ReminderDeliveryChannel && reminder.ChannelID == nil {
		return store.Reminder{}, false
	}
	return reminder, true
}

func reminderMap(reminder store.Reminder) map[string]any {
	out := map[string]any{
		"id":            reminder.ID,
		"user_id":       reminder.UserID,
		"schedule":      reminder.Schedule,
		"kind":          reminder.Kind,
		"note":          reminder.Note,
		"delivery":      string(reminder.Delivery),
		"guild_id":      uint64PtrString(reminder.GuildID),
		"channel_id":    uint64PtrString(reminder.ChannelID),
		"enabled":       reminder.Enabled,
		"next_run_at":   reminder.NextRunAt.UTC().Unix(),
		"failure_count": reminder.FailureCount,
		"created_at":    reminder.CreatedAt.UTC().Unix(),
		"updated_at":    reminder.UpdatedAt.UTC().Unix(),
	}
	if reminder.LastRunAt != nil && !reminder.LastRunAt.IsZero() {
		out["last_run_at"] = reminder.LastRunAt.UTC().Unix()
	} else {
		out["last_run_at"] = nil
	}
	return out
}

func (v *VM) luaUserIDArg(l *lua.LState, idx int) uint64 {
	if l.GetTop() < idx || l.Get(idx) == lua.LNil {
		return v.userID
	}
	switch value := l.Get(idx).(type) {
	case lua.LString:
		if userID := parseSnowflakeString(value.String()); userID != 0 {
			return userID
		}
	case lua.LNumber:
		if value > 0 {
			return uint64(value)
		}
	}
	return v.userID
}

func (v *VM) tableUserID(spec *lua.LTable, key string) uint64 {
	if spec == nil {
		return v.userID
	}
	return luaSnowflake(spec.RawGetString(key), v.userID)
}

func (v *VM) userLocation(userID uint64) *time.Location {
	if v.store == nil || v.store.UserSettings() == nil || userID == 0 {
		return time.UTC
	}
	settings, ok, err := v.store.UserSettings().GetUserSettings(v.ctx(), userID)
	if err != nil || !ok || strings.TrimSpace(settings.Timezone) == "" {
		return time.UTC
	}
	loc, _, err := timezone.LoadLocation(settings.Timezone)
	if err != nil {
		return time.UTC
	}
	return loc
}

func luaSnowflake(value lua.LValue, fallback uint64) uint64 {
	switch typed := value.(type) {
	case lua.LString:
		if parsed := parseSnowflakeString(typed.String()); parsed != 0 {
			return parsed
		}
	case lua.LNumber:
		if typed > 0 {
			return uint64(typed)
		}
	}
	return fallback
}

func luaSnowflakePtr(value lua.LValue) *uint64 {
	parsed := luaSnowflake(value, 0)
	if parsed == 0 {
		return nil
	}
	return &parsed
}

func luaStringDefault(value lua.LValue, fallback string) string {
	if value == lua.LNil {
		return fallback
	}
	if text, ok := value.(lua.LString); ok {
		return text.String()
	}
	return fallback
}

func luaIntDefault(value lua.LValue, fallback int64) int64 {
	if value == lua.LNil {
		return fallback
	}
	if number, ok := value.(lua.LNumber); ok {
		return int64(number)
	}
	return fallback
}

func unixTimeFromLua(value lua.LValue) time.Time {
	if unix := luaIntDefault(value, 0); unix > 0 {
		return time.Unix(unix, 0).UTC()
	}
	return time.Now().UTC()
}

func uint64PtrString(value *uint64) string {
	if value == nil || *value == 0 {
		return ""
	}
	return strconv.FormatUint(*value, 10)
}
