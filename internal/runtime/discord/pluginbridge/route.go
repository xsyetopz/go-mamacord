package pluginbridge

import pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/host"

type Route struct {
	Host     *pluginhost.Host
	PluginID string
}
