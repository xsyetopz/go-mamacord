package discordruntime

import "github.com/xsyetopz/go-mamacord/internal/runtime/discord/catalog"

func (b *Bot) Stats() catalog.Stats {
	stats, _ := b.stats.Load().(catalog.Stats)
	stats.Ready = b.ready.Load()
	return stats
}
