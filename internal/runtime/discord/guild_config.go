package discordruntime

import (
	"context"

	"github.com/xsyetopz/go-mamacord/internal/guildconfig"
)

func (b *Bot) guildCommandEnabled(ctx context.Context, guildID uint64, pluginID, commandName string) (bool, error) {
	if b == nil || guildID == 0 {
		return true, nil
	}
	if !b.moduleEnabled(pluginID) {
		return false, nil
	}
	return guildconfig.CommandEnabled(ctx, b.store, guildID, pluginID, commandName)
}

func (b *Bot) guildPluginEnabled(ctx context.Context, guildID uint64, pluginID string) (bool, error) {
	if b == nil || guildID == 0 {
		return true, nil
	}
	if !b.moduleEnabled(pluginID) {
		return false, nil
	}
	return guildconfig.PluginEnabled(ctx, b.store, guildID, pluginID)
}
