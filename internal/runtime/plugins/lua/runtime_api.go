package luaplugin

import (
	"github.com/xsyetopz/go-mamacord/internal/buildinfo"
	lua "github.com/yuin/gopher-lua"
)

func (v *VM) luaRuntimeBuildInfo(l *lua.LState) int {
	current := buildinfo.Current()
	info := map[string]any{
		"version":            current.Version,
		"description":        current.Description,
		"repository":         current.Repository,
		"mascot_image_url":   current.MascotImageURL,
		"developer_url":      current.DeveloperURL,
		"support_server_url": current.SupportServerURL,
	}

	value, err := anyToLuaValue(l, info, 0)
	if err != nil {
		l.RaiseError("runtime build info unavailable")
		return 0
	}
	l.Push(value)
	return 1
}
