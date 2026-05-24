package discordruntime

import (
	"strings"

	discordpluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

func (b *Bot) enabledPluginIDsForHost(host *pluginhost.Host) map[string]struct{} {
	if host == nil {
		return nil
	}

	out := map[string]struct{}{}
	for moduleID, route := range b.pluginRoutes {
		if route.Host != host {
			continue
		}
		info, ok := b.modules[moduleID]
		if !ok || !info.Enabled {
			continue
		}
		out[moduleID] = struct{}{}
	}
	return out
}

func (b *Bot) pluginRoute(pluginID string) (discordpluginbridge.Route, bool) {
	route, ok := b.pluginRoutes[strings.TrimSpace(pluginID)]
	return route, ok
}

func (b *Bot) enabledPluginJobs() []pluginhost.PluginJob {
	out := []pluginhost.PluginJob{}
	if b.pluginHost != nil {
		for _, job := range b.pluginHost.Jobs() {
			if b.moduleEnabled(job.PluginID) {
				out = append(out, job)
			}
		}
	}
	return out
}

func (b *Bot) enabledPluginEventSubscribers(eventName string) []discordpluginbridge.Route {
	out := []discordpluginbridge.Route{}
	if b.pluginHost != nil {
		for _, pluginID := range b.pluginHost.EventSubscribers(eventName) {
			if !b.moduleEnabled(pluginID) {
				continue
			}
			out = append(out, discordpluginbridge.Route{Host: b.pluginHost, PluginID: pluginID})
		}
	}
	return out
}
