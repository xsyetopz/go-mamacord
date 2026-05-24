package luaplugin

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	lua "github.com/yuin/gopher-lua"
)

func (v *VM) luaPlugin(l *lua.LState) int {
	spec := l.CheckTable(1)
	l.Push(spec)
	return 1
}

func (v *VM) luaCommand(l *lua.LState) int {
	return v.luaTypedCommand(l, "slash")
}

func (v *VM) luaUserCommand(l *lua.LState) int {
	return v.luaTypedCommand(l, "user")
}

func (v *VM) luaMessageCommand(l *lua.LState) int {
	return v.luaTypedCommand(l, "message")
}

func (v *VM) luaTypedCommand(l *lua.LState, kind string) int {
	name := strings.TrimSpace(l.CheckString(1))
	spec := copyLuaTable(l, l.CheckTable(2))
	if name == "" {
		l.RaiseError("command name is required")
		return 0
	}
	spec.RawSetString("name", lua.LString(name))
	spec.RawSetString("type", lua.LString(kind))
	l.Push(spec)
	return 1
}

func (v *VM) luaJob(l *lua.LState) int {
	id := strings.TrimSpace(l.CheckString(1))
	spec := copyLuaTable(l, l.CheckTable(2))
	if id == "" {
		l.RaiseError("job id is required")
		return 0
	}
	spec.RawSetString("id", lua.LString(id))
	l.Push(spec)
	return 1
}

func (v *VM) luaStringOption(l *lua.LState) int      { return v.luaOption(l, "string") }
func (v *VM) luaBoolOption(l *lua.LState) int        { return v.luaOption(l, "bool") }
func (v *VM) luaIntOption(l *lua.LState) int         { return v.luaOption(l, "int") }
func (v *VM) luaFloatOption(l *lua.LState) int       { return v.luaOption(l, "float") }
func (v *VM) luaUserOption(l *lua.LState) int        { return v.luaOption(l, "user") }
func (v *VM) luaChannelOption(l *lua.LState) int     { return v.luaOption(l, "channel") }
func (v *VM) luaRoleOption(l *lua.LState) int        { return v.luaOption(l, "role") }
func (v *VM) luaMentionableOption(l *lua.LState) int { return v.luaOption(l, "mentionable") }
func (v *VM) luaAttachmentOption(l *lua.LState) int  { return v.luaOption(l, "attachment") }

func (v *VM) luaOption(l *lua.LState, typ string) int {
	name := strings.TrimSpace(l.CheckString(1))
	spec := copyLuaTable(l, l.CheckTable(2))
	if name == "" {
		l.RaiseError("option name is required")
		return 0
	}
	spec.RawSetString("name", lua.LString(name))
	spec.RawSetString("type", lua.LString(typ))
	l.Push(spec)
	return 1
}

func (v *VM) luaReply(l *lua.LState) int {
	spec := copyLuaTable(l, l.CheckTable(1))
	spec.RawSetString("type", lua.LString("message"))
	l.Push(spec)
	return 1
}

func (v *VM) luaDefer(l *lua.LState) int {
	spec := l.OptTable(1, l.NewTable())
	ephemeral := true
	if value := spec.RawGetString("ephemeral"); value != lua.LNil {
		ephemeral = luaBoolValue(value, true)
	}
	if v.interaction == nil {
		return pushDiscordBoolResult(l, false, "interaction unavailable")
	}
	if v.routeDeferred {
		return pushDiscordBoolResult(l, true, "")
	}
	if err := v.interaction.Defer(ephemeral); err != nil {
		return pushDiscordBoolResult(l, false, err.Error())
	}
	v.routeDeferred = true
	return pushDiscordBoolResult(l, true, "")
}

func luaBoolValue(value lua.LValue, fallback bool) bool {
	boolean, ok := value.(lua.LBool)
	if !ok {
		return fallback
	}
	return bool(boolean)
}

func (v *VM) luaUpdate(l *lua.LState) int {
	spec := copyLuaTable(l, l.CheckTable(1))
	spec.RawSetString("type", lua.LString("update"))
	if v.routeDeferred {
		spec.RawSetString("__deferred", lua.LTrue)
	}
	l.Push(spec)
	return 1
}

func (v *VM) luaModal(l *lua.LState) int {
	id := strings.TrimSpace(l.CheckString(1))
	spec := copyLuaTable(l, l.CheckTable(2))
	if id == "" {
		l.RaiseError("modal id is required")
		return 0
	}
	spec.RawSetString("type", lua.LString("modal"))
	spec.RawSetString("id", lua.LString(id))
	l.Push(spec)
	return 1
}

func (v *VM) luaPresent(l *lua.LState) int {
	spec := copyLuaTable(l, l.CheckTable(1))
	present := l.NewTable()
	for _, key := range []string{"kind", "title", "body", "fields"} {
		value := spec.RawGetString(key)
		if value != lua.LNil {
			present.RawSetString(key, value)
		}
	}

	out := l.NewTable()
	out.RawSetString("present", present)
	for _, key := range []string{"ephemeral", "components", "content", "embeds"} {
		value := spec.RawGetString(key)
		if value != lua.LNil {
			out.RawSetString(key, value)
		}
	}
	l.Push(out)
	return 1
}

func (v *VM) luaButton(l *lua.LState) int {
	id := strings.TrimSpace(l.CheckString(1))
	spec := copyLuaTable(l, l.CheckTable(2))
	if id == "" {
		l.RaiseError("button id is required")
		return 0
	}
	spec.RawSetString("type", lua.LString("button"))
	spec.RawSetString("id", lua.LString(id))
	l.Push(spec)
	return 1
}

func (v *VM) luaStringSelectOption(l *lua.LState) int {
	label := strings.TrimSpace(l.CheckString(1))
	value := strings.TrimSpace(l.CheckString(2))
	spec := l.OptTable(3, l.NewTable())
	if label == "" {
		l.RaiseError("string select option label is required")
		return 0
	}
	if value == "" {
		l.RaiseError("string select option value is required")
		return 0
	}

	option := copyLuaTable(l, spec)
	option.RawSetString("label", lua.LString(label))
	option.RawSetString("value", lua.LString(value))
	l.Push(option)
	return 1
}

func (v *VM) luaChoice(l *lua.LState) int {
	name := strings.TrimSpace(l.CheckString(1))
	if name == "" {
		l.RaiseError("choice name is required")
		return 0
	}
	out := l.NewTable()
	out.RawSetString("name", lua.LString(name))
	out.RawSetString("value", l.CheckAny(2))
	l.Push(out)
	return 1
}

func (v *VM) luaChoices(l *lua.LState) int {
	l.Push(copyLuaTable(l, l.CheckTable(1)))
	return 1
}

func (v *VM) luaStringSelect(l *lua.LState) int {
	id := strings.TrimSpace(l.CheckString(1))
	spec := copyLuaTable(l, l.CheckTable(2))
	if id == "" {
		l.RaiseError("string select id is required")
		return 0
	}
	spec.RawSetString("type", lua.LString("string_select"))
	spec.RawSetString("id", lua.LString(id))
	l.Push(spec)
	return 1
}

func (v *VM) luaTextInput(l *lua.LState) int {
	id := strings.TrimSpace(l.CheckString(1))
	spec := copyLuaTable(l, l.CheckTable(2))
	if id == "" {
		l.RaiseError("text input id is required")
		return 0
	}
	spec.RawSetString("id", lua.LString(id))
	l.Push(spec)
	return 1
}

func (v *VM) luaRandomInt(l *lua.LState) int {
	min := l.CheckInt(1)
	max := l.CheckInt(2)
	if max < min {
		l.RaiseError("max must be >= min")
		return 0
	}

	n, err := cryptoRandIntInclusive(min, max)
	if err != nil {
		l.RaiseError("random failed")
		return 0
	}
	l.Push(lua.LNumber(n))
	return 1
}

func (v *VM) luaRandomChoice(l *lua.LState) int {
	list := l.CheckTable(1)
	length := list.Len()
	if length == 0 {
		l.RaiseError("choice requires a non-empty list")
		return 0
	}

	index, err := cryptoRandIntInclusive(1, length)
	if err != nil {
		l.RaiseError("random failed")
		return 0
	}
	l.Push(list.RawGetInt(index))
	return 1
}

func (v *VM) luaTimeUnix(l *lua.LState) int {
	l.Push(lua.LNumber(time.Now().UTC().Unix()))
	return 1
}

func (v *VM) luaEffectSendChannel(l *lua.LState) int {
	spec := copyLuaTable(l, l.CheckTable(1))
	action := l.NewTable()
	action.RawSetString("type", lua.LString("send_channel"))
	if channelID := spec.RawGetString("channel_id"); channelID != lua.LNil {
		action.RawSetString("channel_id", channelID)
	}
	action.RawSetString("message", spec.RawGetString("message"))

	actions := l.NewTable()
	actions.RawSetInt(1, action)
	out := l.NewTable()
	out.RawSetString("actions", actions)
	l.Push(out)
	return 1
}

func (v *VM) luaEffectSendDM(l *lua.LState) int {
	spec := copyLuaTable(l, l.CheckTable(1))
	action := l.NewTable()
	action.RawSetString("type", lua.LString("send_dm"))
	if userID := spec.RawGetString("user_id"); userID != lua.LNil {
		action.RawSetString("user_id", userID)
	}
	action.RawSetString("message", spec.RawGetString("message"))

	actions := l.NewTable()
	actions.RawSetInt(1, action)
	out := l.NewTable()
	out.RawSetString("actions", actions)
	l.Push(out)
	return 1
}

func (v *VM) luaEffectTimeoutMember(l *lua.LState) int {
	spec := copyLuaTable(l, l.CheckTable(1))
	action := l.NewTable()
	action.RawSetString("type", lua.LString("timeout_member"))
	if guildID := spec.RawGetString("guild_id"); guildID != lua.LNil {
		action.RawSetString("guild_id", guildID)
	}
	if userID := spec.RawGetString("user_id"); userID != lua.LNil {
		action.RawSetString("user_id", userID)
	}
	if untilUnix := spec.RawGetString("until_unix"); untilUnix != lua.LNil {
		action.RawSetString("until_unix", untilUnix)
	}

	actions := l.NewTable()
	actions.RawSetInt(1, action)
	out := l.NewTable()
	out.RawSetString("actions", actions)
	l.Push(out)
	return 1
}

func (v *VM) luaDiscordSendDM(l *lua.LState) int {
	if !v.perms.Discord.Messages {
		l.RaiseError("permission denied: discord.send_dm")
		return 0
	}
	if v.discord == nil {
		l.Push(lua.LNil)
		l.Push(lua.LString("discord unavailable"))
		return 2
	}

	spec := l.CheckTable(1)
	userID := luaSnowflake(spec.RawGetString("user_id"), v.userID)
	message, ok, err := luaToAny(spec.RawGetString("message"))
	if err != nil {
		l.RaiseError("invalid send_dm spec")
		return 0
	}
	if !ok || userID == 0 {
		l.RaiseError("invalid send_dm spec")
		return 0
	}
	encoded, err := EncodeValue(message)
	if err != nil {
		l.RaiseError("invalid send_dm spec")
		return 0
	}

	result, execErr := v.discord.SendDM(v.ctx(), v.plugin, userID, encoded)
	if execErr != nil {
		l.Push(lua.LNil)
		l.Push(lua.LString(execErr.Error()))
		return 2
	}

	l.Push(v.luaMessageResult(l, result))
	l.Push(lua.LNil)
	return 2
}

func (v *VM) luaDiscordSendChannel(l *lua.LState) int {
	if !v.perms.Discord.Messages {
		l.RaiseError("permission denied: discord.send_channel")
		return 0
	}
	if v.discord == nil {
		l.Push(lua.LNil)
		l.Push(lua.LString("discord unavailable"))
		return 2
	}

	spec := l.CheckTable(1)
	channelID := luaSnowflake(spec.RawGetString("channel_id"), v.channel)
	message, ok, err := luaToAny(spec.RawGetString("message"))
	if err != nil {
		l.RaiseError("invalid send_channel spec")
		return 0
	}
	if !ok || channelID == 0 {
		l.RaiseError("invalid send_channel spec")
		return 0
	}
	encoded, err := EncodeValue(message)
	if err != nil {
		l.RaiseError("invalid send_channel spec")
		return 0
	}

	result, execErr := v.discord.SendChannel(v.ctx(), v.plugin, channelID, encoded)
	if execErr != nil {
		l.Push(lua.LNil)
		l.Push(lua.LString(execErr.Error()))
		return 2
	}

	l.Push(v.luaMessageResult(l, result))
	l.Push(lua.LNil)
	return 2
}

func (v *VM) luaDiscordTimeoutMember(l *lua.LState) int {
	if !v.perms.Discord.Members {
		l.RaiseError("permission denied: discord.timeout_member")
		return 0
	}
	if v.discord == nil {
		l.Push(lua.LFalse)
		l.Push(lua.LString("discord unavailable"))
		return 2
	}

	spec := l.CheckTable(1)
	guildID := v.tableGuildID(spec, "guild_id")
	userID := luaSnowflake(spec.RawGetString("user_id"), v.userID)
	untilUnix := luaIntDefault(spec.RawGetString("until_unix"), 0)
	if guildID == 0 || userID == 0 || untilUnix <= 0 {
		l.RaiseError("invalid timeout spec")
		return 0
	}

	err := v.discord.TimeoutMember(v.ctx(), guildID, userID, time.Unix(untilUnix, 0).UTC())
	if err != nil {
		l.Push(lua.LFalse)
		l.Push(lua.LString(err.Error()))
		return 2
	}

	l.Push(lua.LTrue)
	l.Push(lua.LNil)
	return 2
}

func (v *VM) luaMessageResult(l *lua.LState, result MessageResult) *lua.LTable {
	out := l.NewTable()
	out.RawSetString("message_id", lua.LString(strconv.FormatUint(result.MessageID, 10)))
	out.RawSetString("channel_id", lua.LString(strconv.FormatUint(result.ChannelID, 10)))
	if result.UserID != 0 {
		out.RawSetString("user_id", lua.LString(strconv.FormatUint(result.UserID, 10)))
	}
	return out
}

func (v *VM) luaRequire(l *lua.LState) int {
	relPath := strings.TrimSpace(l.CheckString(1))
	if relPath == "" {
		l.RaiseError("module path is required")
		return 0
	}

	cleanPath, absPath, err := v.resolveLocalLuaPath(relPath)
	if err != nil {
		l.RaiseError("%s", err.Error())
		return 0
	}

	if cached, ok := v.moduleCache[cleanPath]; ok {
		l.Push(cached)
		return 1
	}

	fn, loadErr := l.LoadFile(absPath)
	if loadErr != nil {
		l.RaiseError("module load failed")
		return 0
	}

	l.Push(fn)
	if callErr := l.PCall(0, 1, nil); callErr != nil {
		l.RaiseError("module load failed")
		return 0
	}

	module := l.Get(-1)
	l.Pop(1)
	if module == lua.LNil {
		module = lua.LTrue
	}

	v.moduleCache[cleanPath] = module
	l.Push(module)
	return 1
}

func (v *VM) resolveLocalLuaPath(rel string) (string, string, error) {
	if strings.Contains(rel, "\\") {
		return "", "", fmt.Errorf("invalid lua path")
	}
	if !strings.HasSuffix(strings.ToLower(rel), ".lua") {
		return "", "", fmt.Errorf("lua path must end with .lua")
	}
	if strings.HasPrefix(rel, "/") {
		return "", "", fmt.Errorf("lua path must be relative")
	}

	clean := filepath.Clean(rel)
	if clean == "." || strings.HasPrefix(clean, "..") || strings.Contains(clean, "/..") {
		return "", "", fmt.Errorf("lua path escapes plugin dir")
	}

	baseAbs, err := filepath.Abs(v.dir)
	if err != nil {
		return "", "", fmt.Errorf("lua path error")
	}
	targetAbs, err := filepath.Abs(filepath.Join(v.dir, clean))
	if err != nil {
		return "", "", fmt.Errorf("lua path error")
	}

	relToBase, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil {
		return "", "", fmt.Errorf("lua path escapes plugin dir")
	}
	relToBaseSlash := filepath.ToSlash(relToBase)
	if relToBase == "." || strings.HasPrefix(relToBase, "..") || strings.HasPrefix(relToBaseSlash, "../") {
		return "", "", fmt.Errorf("lua path escapes plugin dir")
	}

	fi, err := os.Stat(targetAbs)
	if err != nil {
		return "", "", fmt.Errorf("lua file not found")
	}
	if fi.Size() > 128*1024 {
		return "", "", fmt.Errorf("lua file too large")
	}

	return filepath.ToSlash(clean), targetAbs, nil
}

func copyLuaTable(l *lua.LState, source *lua.LTable) *lua.LTable {
	clone := l.NewTable()
	source.ForEach(func(key, value lua.LValue) {
		clone.RawSet(key, value)
	})
	return clone
}

func cryptoRandIntInclusive(min, max int) (int, error) {
	if max < min {
		return 0, fmt.Errorf("invalid range")
	}
	width := max - min + 1
	if width <= 0 {
		return 0, fmt.Errorf("invalid range")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(int64(width)))
	if err != nil {
		return 0, err
	}
	return min + int(n.Int64()), nil
}
