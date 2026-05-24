package discordruntime

import (
	"context"
	"errors"

	commandruntime "github.com/xsyetopz/go-mamacord/internal/commandruntime"
	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

type pluginAdmin struct{ b *Bot }

func (p pluginAdmin) Configured() bool { return p.b != nil && p.b.pluginHost != nil }

func (p pluginAdmin) Infos() []pluginhost.PluginInfo {
	if p.b == nil {
		return nil
	}
	out := []pluginhost.PluginInfo{}
	if p.b.pluginHost != nil {
		out = append(out, p.b.pluginHost.Infos()...)
	}
	return out
}

func (p pluginAdmin) Reload(ctx context.Context) error {
	if p.b == nil || p.b.pluginHost == nil {
		return errors.New("plugins not configured")
	}
	return p.b.reloadModules(ctx)
}

var _ commandruntime.PluginAdmin = pluginAdmin{}

type moduleAdmin struct{ b *Bot }

func (m moduleAdmin) Configured() bool { return m.b != nil }

func (m moduleAdmin) Infos() []moduleapi.Info {
	if m.b == nil {
		return nil
	}
	return m.b.moduleInfos()
}

func (m moduleAdmin) Reload(ctx context.Context) error {
	if m.b == nil {
		return errors.New("modules not configured")
	}
	return m.b.reloadModules(ctx)
}

func (m moduleAdmin) SetEnabled(ctx context.Context, moduleID string, enabled bool, actorID uint64) error {
	if m.b == nil {
		return errors.New("modules not configured")
	}
	return m.b.setModuleEnabled(ctx, moduleID, enabled, actorID)
}

func (m moduleAdmin) Reset(ctx context.Context, moduleID string) error {
	if m.b == nil {
		return errors.New("modules not configured")
	}
	return m.b.resetModule(ctx, moduleID)
}

var _ moduleapi.Admin = moduleAdmin{}
