package pluginbridge

import pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"

type Route struct {
	Host     *pluginhost.Host
	PluginID string
}

type Target = Route
