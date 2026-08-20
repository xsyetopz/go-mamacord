package discordruntime

import (
	"fmt"
	"sync"
	"testing"

	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	discordpluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
)

func TestRuntimeCatalogPublishesCoherentSnapshotsConcurrently(t *testing.T) {
	bot := &Bot{}
	const iterations = 10_000

	var readers sync.WaitGroup
	readers.Add(2)
	for range 2 {
		go func() {
			defer readers.Done()
			for range iterations {
				dispatcher := bot.commandDispatcher()
				for _, marker := range []string{"a", "b"} {
					_, hasCommand := dispatcher.Commands[marker]
					_, hasPlugin := dispatcher.PluginCommands[marker]
					if hasCommand != hasPlugin {
						t.Errorf("observed mixed runtime catalog for marker %q", marker)
						return
					}
				}
			}
		}()
	}

	for i := range iterations {
		marker := "a"
		if i%2 != 0 {
			marker = "b"
		}
		bot.runtimeCatalog.Store(&runtimeCatalog{
			modules: map[string]moduleapi.Info{
				marker: {ID: marker, Name: fmt.Sprintf("module-%s", marker)},
			},
			commands: map[string]slashcmd.Command{
				marker: {Name: marker},
			},
			pluginCommands: map[string]discordpluginbridge.Route{
				marker: {PluginID: marker},
			},
			pluginUserCommands:    map[string]discordpluginbridge.Route{},
			pluginMessageCommands: map[string]discordpluginbridge.Route{},
			pluginRoutes:          map[string]discordpluginbridge.Route{},
		})
	}
	readers.Wait()
}
