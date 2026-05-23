package discordruntime

import "github.com/xsyetopz/go-mamacord/internal/runtime/discord/catalog"

type Stats = catalog.Stats

func (b *Bot) Stats() Stats {
	stats, _ := b.stats.Load().(Stats)
	stats.Ready = b.ready.Load()
	return stats
}
