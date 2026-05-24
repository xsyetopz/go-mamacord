package luaplugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

func (v *VM) loadEntryFile(fileName string) (*pluginDefinition, error) {
	fn, err := v.L.LoadFile(fileName)
	if err != nil {
		return nil, err
	}

	v.L.Push(fn)
	if callErr := v.L.PCall(0, 1, nil); callErr != nil {
		return nil, callErr
	}

	raw := v.L.Get(-1)
	v.L.Pop(1)
	return parsePluginDefinition(raw)
}

func (v *VM) HasDefinition() bool {
	v.mu.Lock()
	defer v.mu.Unlock()

	return v.definition != nil
}

func (v *VM) Definition() (Definition, bool) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.definition == nil {
		return Definition{}, false
	}
	return v.definition.definition(), true
}

func (v *VM) CallRoute(ctx context.Context, kind RouteKind, routeID string, payload Payload) (any, bool, error) {
	res, hasValue, err := v.callRouteResult(ctx, kind, routeID, payload)
	if err != nil || !hasValue {
		return nil, hasValue, err
	}

	out, _, err := luaToAny(res)
	if err != nil {
		return nil, false, fmt.Errorf("lua route %q (%s) return: %w", routeID, kind, err)
	}
	return out, true, nil
}

func (v *VM) CallEncodedRoute(ctx context.Context, kind RouteKind, routeID string, payload Payload) (EncodedValue, bool, error) {
	res, hasValue, err := v.callRouteResult(ctx, kind, routeID, payload)
	if err != nil || !hasValue {
		return nil, hasValue, err
	}

	encoded, err := encodeLuaResult(res, fmt.Sprintf("lua route %q (%s)", routeID, kind))
	if err != nil {
		return nil, false, err
	}
	return encoded, true, nil
}

func (v *VM) CallAutocomplete(ctx context.Context, routeID string, payload Payload) (EncodedValue, error) {
	encoded, hasValue, err := v.CallEncodedRoute(ctx, RouteAutocomplete, routeID, payload)
	if err != nil || !hasValue {
		return nil, err
	}
	return encoded, nil
}

func encodeLuaResult(res lua.LValue, context string) (EncodedValue, error) {
	out, _, err := luaToAny(res)
	if err != nil {
		return nil, fmt.Errorf("%s return: %w", context, err)
	}

	encoded, err := EncodeValue(out)
	if err != nil {
		return nil, fmt.Errorf("%s encode: %w", context, err)
	}
	return encoded, nil
}

func (v *VM) callRouteResult(ctx context.Context, kind RouteKind, routeID string, payload Payload) (lua.LValue, bool, error) {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.L == nil {
		return lua.LNil, false, errors.New("lua vm is closed")
	}
	if v.definition == nil {
		return lua.LNil, false, errors.New("plugin descriptor not loaded")
	}

	handler, ok := v.definition.lookup(kind, routeID)
	if !ok || handler == nil {
		return lua.LNil, false, fmt.Errorf("route %q (%s) not found", routeID, kind)
	}

	timeoutCtx := ctx
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		timeoutCtx, cancel = context.WithTimeout(ctx, defaultCallTimeout)
		defer cancel()
	}

	prevCtx := v.L.RemoveContext()
	v.L.SetContext(timeoutCtx)
	defer func() {
		_ = v.L.RemoveContext()
		if prevCtx != nil {
			v.L.SetContext(prevCtx)
		}
	}()

	v.execCtx = timeoutCtx
	v.locale = strings.TrimSpace(payload.Locale)
	v.userID = parseSnowflakeString(payload.UserID)
	v.guildID = parseSnowflakeString(payload.GuildID)
	v.channel = parseSnowflakeString(payload.ChannelID)
	v.interaction = payload.Interaction
	v.routeDeferred = false
	defer func() {
		v.execCtx = nil
		v.locale = ""
		v.userID = 0
		v.guildID = 0
		v.channel = 0
		v.interaction = nil
		v.routeDeferred = false
	}()

	ctxTable, err := v.routeContextToLua(kind, routeID, payload)
	if err != nil {
		return lua.LNil, false, err
	}

	v.L.Push(handler)
	v.L.Push(ctxTable)
	if callErr := v.L.PCall(1, 1, nil); callErr != nil {
		return lua.LNil, false, fmt.Errorf("lua route %q (%s): %w", routeID, kind, callErr)
	}

	res := v.L.Get(-1)
	v.L.Pop(1)
	if res == lua.LNil {
		return lua.LNil, false, nil
	}

	return res, true, nil
}

func (v *VM) routeContextToLua(kind RouteKind, routeID string, payload Payload) (*lua.LTable, error) {
	root := v.L.NewTable()

	root.RawSetString("guild_id", lua.LString(strings.TrimSpace(payload.GuildID)))
	root.RawSetString("channel_id", lua.LString(strings.TrimSpace(payload.ChannelID)))
	root.RawSetString("user_id", lua.LString(strings.TrimSpace(payload.UserID)))
	root.RawSetString("locale", lua.LString(strings.TrimSpace(payload.Locale)))
	root.RawSetString("is_owner", lua.LBool(payload.IsOwner))

	root.RawSetString("guild", v.entityTable(payload.GuildID))
	root.RawSetString("channel", v.entityTable(payload.ChannelID))
	root.RawSetString("user", v.entityTable(payload.UserID))
	root.RawSetString("plugin", v.entityTable(v.plugin))
	root.RawSetString("store", v.scopedStoreTable(payload.GuildID))
	root.RawSetString("bot", v.L.GetGlobal("bot"))

	optionsTable, err := anyToLuaValue(v.L, payload.Options, 0)
	if err != nil {
		return nil, fmt.Errorf("route options: %w", err)
	}
	root.RawSetString("options", optionsTable)

	switch kind {
	case RouteCommand, RouteUserCommand, RouteMessageCommand:
		args := commandArgs(payload.Options)
		argsTable, err := anyToLuaValue(v.L, args, 0)
		if err != nil {
			return nil, fmt.Errorf("command args: %w", err)
		}
		resolved := commandResolved(payload.Options)
		resolvedTable, err := anyToLuaValue(v.L, resolved, 0)
		if err != nil {
			return nil, fmt.Errorf("command resolved: %w", err)
		}
		commandTable := v.L.NewTable()
		commandTable.RawSetString("name", lua.LString(strings.TrimSpace(routeID)))
		commandTable.RawSetString("kind", lua.LString(routeKindName(kind)))
		commandTable.RawSetString("group", lua.LString(payloadString(payload.Options, "__group")))
		commandTable.RawSetString("subcommand", lua.LString(payloadString(payload.Options, "__subcommand")))
		commandTable.RawSetString("args", argsTable)
		commandTable.RawSetString("resolved", resolvedTable)
		root.RawSetString("command", commandTable)
		root.RawSetString("args", argsTable)
		if kind == RouteUserCommand || kind == RouteMessageCommand {
			targetTable, targetErr := targetContextTable(v.L, payload.Options)
			if targetErr != nil {
				return nil, targetErr
			}
			root.RawSetString("target", targetTable)
		}
	case RouteAutocomplete:
		args := commandArgs(payload.Options)
		argsTable, err := anyToLuaValue(v.L, args, 0)
		if err != nil {
			return nil, fmt.Errorf("autocomplete args: %w", err)
		}
		commandTable := v.L.NewTable()
		commandTable.RawSetString("name", lua.LString(payloadString(payload.Options, "__command")))
		commandTable.RawSetString("kind", lua.LString("slash"))
		commandTable.RawSetString("group", lua.LString(payloadString(payload.Options, "__group")))
		commandTable.RawSetString("subcommand", lua.LString(payloadString(payload.Options, "__subcommand")))
		commandTable.RawSetString("args", argsTable)
		root.RawSetString("command", commandTable)
		root.RawSetString("args", argsTable)

		autocompleteTable := v.L.NewTable()
		autocompleteTable.RawSetString("command", lua.LString(payloadString(payload.Options, "__command")))
		autocompleteTable.RawSetString("group", lua.LString(payloadString(payload.Options, "__group")))
		autocompleteTable.RawSetString("subcommand", lua.LString(payloadString(payload.Options, "__subcommand")))
		autocompleteTable.RawSetString("option", lua.LString(payloadString(payload.Options, "__option")))
		if value, ok := payload.Options["__value"]; ok {
			lv, valueErr := anyToLuaValue(v.L, value, 0)
			if valueErr != nil {
				return nil, fmt.Errorf("autocomplete value: %w", valueErr)
			}
			autocompleteTable.RawSetString("value", lv)
		}
		root.RawSetString("autocomplete", autocompleteTable)
	case RouteComponent:
		componentTable := v.L.NewTable()
		componentTable.RawSetString("id", lua.LString(strings.TrimSpace(routeID)))
		componentTable.RawSetString("kind", lua.LString(payloadString(payload.Options, "type")))
		if values, ok := payload.Options["values"]; ok {
			lv, err := anyToLuaValue(v.L, values, 0)
			if err != nil {
				return nil, fmt.Errorf("component values: %w", err)
			}
			componentTable.RawSetString("values", lv)
		}
		root.RawSetString("component", componentTable)
	case RouteModal:
		modalTable := v.L.NewTable()
		modalTable.RawSetString("id", lua.LString(strings.TrimSpace(routeID)))
		if fields, ok := payload.Options["fields"]; ok {
			lv, err := anyToLuaValue(v.L, fields, 0)
			if err != nil {
				return nil, fmt.Errorf("modal fields: %w", err)
			}
			modalTable.RawSetString("fields", lv)
		}
		root.RawSetString("modal", modalTable)
	case RouteEvent:
		eventTable := v.L.NewTable()
		eventTable.RawSetString("name", lua.LString(strings.TrimSpace(routeID)))
		root.RawSetString("event", eventTable)
	case RouteJob:
		jobTable := v.L.NewTable()
		jobTable.RawSetString("id", lua.LString(strings.TrimSpace(routeID)))
		root.RawSetString("job", jobTable)
	}

	return root, nil
}

func (v *VM) entityTable(id string) *lua.LTable {
	table := v.L.NewTable()
	table.RawSetString("id", lua.LString(strings.TrimSpace(id)))
	return table
}

func commandArgs(options map[string]any) map[string]any {
	if len(options) == 0 {
		return map[string]any{}
	}

	args := make(map[string]any, len(options))
	for key, value := range options {
		switch key {
		case "__group", "__subcommand":
			continue
		default:
			if strings.HasPrefix(key, "__resolved:") {
				continue
			}
			args[key] = value
		}
	}
	return args
}

func commandResolved(options map[string]any) map[string]any {
	if len(options) == 0 {
		return map[string]any{}
	}

	resolved := map[string]any{}
	for key, value := range options {
		if !strings.HasPrefix(key, "__resolved:") {
			continue
		}
		name := strings.TrimSpace(strings.TrimPrefix(key, "__resolved:"))
		if name == "" {
			continue
		}
		resolved[name] = value
	}
	return resolved
}

func routeKindName(kind RouteKind) string {
	switch kind {
	case RouteUserCommand:
		return "user"
	case RouteMessageCommand:
		return "message"
	default:
		return "slash"
	}
}

func targetContextTable(l *lua.LState, options map[string]any) (*lua.LTable, error) {
	target := l.NewTable()
	if user, ok := options["__target_user"]; ok {
		lv, err := anyToLuaValue(l, user, 0)
		if err != nil {
			return nil, fmt.Errorf("target user: %w", err)
		}
		target.RawSetString("user", lv)
	}
	if member, ok := options["__target_member"]; ok {
		lv, err := anyToLuaValue(l, member, 0)
		if err != nil {
			return nil, fmt.Errorf("target member: %w", err)
		}
		target.RawSetString("member", lv)
	}
	if message, ok := options["__target_message"]; ok {
		lv, err := anyToLuaValue(l, message, 0)
		if err != nil {
			return nil, fmt.Errorf("target message: %w", err)
		}
		target.RawSetString("message", lv)
	}
	return target, nil
}

func payloadString(options map[string]any, key string) string {
	if options == nil {
		return ""
	}
	value, ok := options[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(text)
}

func (v *VM) scopedStoreTable(guildID string) *lua.LTable {
	table := v.L.NewTable()
	table.RawSetString("get", v.L.NewFunction(func(l *lua.LState) int {
		return v.luaScopedKVGet(l, guildID)
	}))
	table.RawSetString("put", v.L.NewFunction(func(l *lua.LState) int {
		return v.luaScopedKVPut(l, guildID)
	}))
	table.RawSetString("del", v.L.NewFunction(func(l *lua.LState) int {
		return v.luaScopedKVDel(l, guildID)
	}))
	table.RawSetString("get_json", v.L.NewFunction(func(l *lua.LState) int {
		return v.luaScopedKVGetJSON(l, guildID)
	}))
	table.RawSetString("put_json", v.L.NewFunction(func(l *lua.LState) int {
		return v.luaScopedKVPutJSON(l, guildID)
	}))
	return table
}

func (v *VM) luaScopedKVGet(l *lua.LState, guildID string) int {
	if !v.perms.Storage.KV {
		l.RaiseError("permission denied: storage.kv")
		return 0
	}
	if v.store == nil || v.store.PluginKV() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	guild := parseSnowflakeString(guildID)
	key := strings.TrimSpace(l.CheckString(1))
	if guild == 0 || key == "" {
		l.Push(lua.LNil)
		l.Push(lua.LFalse)
		return 2
	}

	value, ok, err := v.store.PluginKV().GetPluginKV(v.ctx(), guild, v.plugin, key)
	if err != nil {
		l.RaiseError("storage error")
		return 0
	}
	if !ok {
		l.Push(lua.LNil)
		l.Push(lua.LFalse)
		return 2
	}

	var decoded any
	if unmarshalErr := json.Unmarshal([]byte(value), &decoded); unmarshalErr != nil {
		l.RaiseError("storage decode error")
		return 0
	}
	lv, err := anyToLuaValue(l, decoded, 0)
	if err != nil {
		l.RaiseError("storage decode error")
		return 0
	}

	l.Push(lv)
	l.Push(lua.LTrue)
	return 2
}

func (v *VM) luaScopedKVPut(l *lua.LState, guildID string) int {
	if !v.perms.Storage.KV {
		l.RaiseError("permission denied: storage.kv")
		return 0
	}
	if v.store == nil || v.store.PluginKV() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	guild := parseSnowflakeString(guildID)
	key := strings.TrimSpace(l.CheckString(1))
	if guild == 0 || key == "" {
		l.RaiseError("invalid key")
		return 0
	}

	goVal, _, err := luaToAny(l.CheckAny(2))
	if err != nil {
		l.RaiseError("invalid value")
		return 0
	}
	enc, err := json.Marshal(goVal)
	if err != nil {
		l.RaiseError("value must be JSON encodable")
		return 0
	}
	if len(enc) > 16*1024 {
		l.RaiseError("value too large")
		return 0
	}
	if putErr := v.store.PluginKV().PutPluginKV(v.ctx(), guild, v.plugin, key, string(enc)); putErr != nil {
		l.RaiseError("storage error")
		return 0
	}

	l.Push(lua.LTrue)
	return 1
}

func (v *VM) luaScopedKVDel(l *lua.LState, guildID string) int {
	if !v.perms.Storage.KV {
		l.RaiseError("permission denied: storage.kv")
		return 0
	}
	if v.store == nil || v.store.PluginKV() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	guild := parseSnowflakeString(guildID)
	key := strings.TrimSpace(l.CheckString(1))
	if guild == 0 || key == "" {
		l.RaiseError("invalid key")
		return 0
	}
	if err := v.store.PluginKV().DeletePluginKV(v.ctx(), guild, v.plugin, key); err != nil {
		l.RaiseError("storage error")
		return 0
	}

	l.Push(lua.LTrue)
	return 1
}

func (v *VM) luaScopedKVGetJSON(l *lua.LState, guildID string) int {
	if !v.perms.Storage.KV {
		l.RaiseError("permission denied: storage.kv")
		return 0
	}
	if v.store == nil || v.store.PluginKV() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	guild := parseSnowflakeString(guildID)
	key := strings.TrimSpace(l.CheckString(1))
	if guild == 0 || key == "" {
		l.Push(lua.LNil)
		l.Push(lua.LFalse)
		return 2
	}

	value, ok, err := v.store.PluginKV().GetPluginKV(v.ctx(), guild, v.plugin, key)
	if err != nil {
		l.RaiseError("storage error")
		return 0
	}
	if !ok {
		l.Push(lua.LNil)
		l.Push(lua.LFalse)
		return 2
	}

	l.Push(lua.LString(value))
	l.Push(lua.LTrue)
	return 2
}

func (v *VM) luaScopedKVPutJSON(l *lua.LState, guildID string) int {
	if !v.perms.Storage.KV {
		l.RaiseError("permission denied: storage.kv")
		return 0
	}
	if v.store == nil || v.store.PluginKV() == nil {
		l.RaiseError("storage unavailable")
		return 0
	}

	guild := parseSnowflakeString(guildID)
	key := strings.TrimSpace(l.CheckString(1))
	value := l.CheckString(2)
	if guild == 0 || key == "" {
		l.RaiseError("invalid key")
		return 0
	}
	if !json.Valid([]byte(value)) {
		l.RaiseError("value must be JSON")
		return 0
	}
	if len(value) > 16*1024 {
		l.RaiseError("value too large")
		return 0
	}
	if putErr := v.store.PluginKV().PutPluginKV(v.ctx(), guild, v.plugin, key, value); putErr != nil {
		l.RaiseError("storage error")
		return 0
	}

	l.Push(lua.LTrue)
	return 1
}
