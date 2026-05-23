package luaplugin

import (
	"strings"

	"github.com/google/uuid"
	lua "github.com/yuin/gopher-lua"

	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

const defaultPluginWarningListLimit = 25

func (v *VM) luaWarningsCount(l *lua.LState) int {
	if !v.perms.Storage.Warnings {
		l.RaiseError("permission denied: storage.warnings")
		return 0
	}
	if v.store == nil || v.store.Warnings() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	guildID := v.luaGuildIDArg(l, 1)
	userID := v.luaUserIDArg(l, 2)
	if guildID == 0 || userID == 0 {
		l.Push(lua.LNumber(0))
		return 1
	}

	count, err := v.store.Warnings().CountWarnings(v.ctx(), guildID, userID)
	if err != nil {
		l.RaiseError("storage error")
		return 0
	}
	l.Push(lua.LNumber(count))
	return 1
}

func (v *VM) luaWarningsList(l *lua.LState) int {
	if !v.perms.Storage.Warnings {
		l.RaiseError("permission denied: storage.warnings")
		return 0
	}
	if v.store == nil || v.store.Warnings() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	guildID := v.luaGuildIDArg(l, 1)
	userID := v.luaUserIDArg(l, 2)
	limit := l.OptInt(3, defaultPluginWarningListLimit)
	if guildID == 0 || userID == 0 {
		l.Push(l.NewTable())
		return 1
	}
	if limit <= 0 {
		limit = defaultPluginWarningListLimit
	}

	items, err := v.store.Warnings().ListWarnings(v.ctx(), guildID, userID, limit)
	if err != nil {
		l.RaiseError("storage error")
		return 0
	}

	out := l.NewTable()
	for idx, item := range items {
		value, convErr := anyToLuaValue(l, map[string]any{
			"id":           item.ID,
			"guild_id":     item.GuildID,
			"user_id":      item.UserID,
			"moderator_id": item.ModeratorID,
			"reason":       item.Reason,
			"created_at":   item.CreatedAt.UTC().Unix(),
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

func (v *VM) luaWarningsCreate(l *lua.LState) int {
	if !v.perms.Storage.Warnings {
		l.RaiseError("permission denied: storage.warnings")
		return 0
	}
	if v.store == nil || v.store.Warnings() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	spec := l.CheckTable(1)
	entry := store.Warning{
		ID:          strings.TrimSpace(luaStringDefault(spec.RawGetString("id"), uuid.NewString())),
		GuildID:     v.tableGuildID(spec, "guild_id"),
		UserID:      v.tableUserID(spec, "user_id"),
		ModeratorID: luaSnowflake(spec.RawGetString("moderator_id"), v.userID),
		Reason:      strings.TrimSpace(luaStringDefault(spec.RawGetString("reason"), "")),
		CreatedAt:   unixTimeFromLua(spec.RawGetString("created_at")),
	}
	if entry.GuildID == 0 || entry.UserID == 0 || entry.ModeratorID == 0 || entry.Reason == "" {
		l.RaiseError("invalid warning")
		return 0
	}

	if err := v.store.Warnings().CreateWarning(v.ctx(), entry); err != nil {
		l.RaiseError("storage error")
		return 0
	}

	value, convErr := anyToLuaValue(l, map[string]any{
		"id":           entry.ID,
		"guild_id":     entry.GuildID,
		"user_id":      entry.UserID,
		"moderator_id": entry.ModeratorID,
		"reason":       entry.Reason,
		"created_at":   entry.CreatedAt.UTC().Unix(),
	}, 0)
	if convErr != nil {
		l.RaiseError("storage decode error")
		return 0
	}
	l.Push(value)
	return 1
}

func (v *VM) luaWarningsDelete(l *lua.LState) int {
	if !v.perms.Storage.Warnings {
		l.RaiseError("permission denied: storage.warnings")
		return 0
	}
	if v.store == nil || v.store.Warnings() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	id := strings.TrimSpace(l.CheckString(1))
	if id == "" {
		l.RaiseError("warning id is required")
		return 0
	}
	if err := v.store.Warnings().DeleteWarning(v.ctx(), id); err != nil {
		l.RaiseError("storage error")
		return 0
	}

	l.Push(lua.LTrue)
	return 1
}

func (v *VM) luaAuditAppend(l *lua.LState) int {
	if !v.perms.Storage.Audit {
		l.RaiseError("permission denied: storage.audit")
		return 0
	}
	if v.store == nil || v.store.Audit() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	spec := l.CheckTable(1)
	action := strings.TrimSpace(luaStringDefault(spec.RawGetString("action"), ""))
	if action == "" {
		l.RaiseError("audit action is required")
		return 0
	}

	entry := store.AuditEntry{
		GuildID:   luaSnowflakePtr(spec.RawGetString("guild_id")),
		ActorID:   luaSnowflakePtr(spec.RawGetString("actor_id")),
		Action:    action,
		TargetID:  luaSnowflakePtr(spec.RawGetString("target_id")),
		CreatedAt: unixTimeFromLua(spec.RawGetString("created_at")),
		MetaJSON:  strings.TrimSpace(luaStringDefault(spec.RawGetString("meta_json"), "{}")),
	}

	switch strings.ToLower(strings.TrimSpace(luaStringDefault(spec.RawGetString("target_type"), ""))) {
	case "":
	case string(store.TargetTypeUser):
		targetType := store.TargetTypeUser
		entry.TargetType = &targetType
	case string(store.TargetTypeGuild):
		targetType := store.TargetTypeGuild
		entry.TargetType = &targetType
	default:
		l.RaiseError("invalid audit target_type")
		return 0
	}

	if err := v.store.Audit().Append(v.ctx(), entry); err != nil {
		l.RaiseError("storage error")
		return 0
	}

	l.Push(lua.LTrue)
	return 1
}

func (v *VM) luaGuildIDArg(l *lua.LState, index int) uint64 {
	value := l.Get(index)
	return luaSnowflake(value, v.guildID)
}

func (v *VM) tableGuildID(spec *lua.LTable, key string) uint64 {
	if spec == nil {
		return v.guildID
	}
	return luaSnowflake(spec.RawGetString(key), v.guildID)
}
