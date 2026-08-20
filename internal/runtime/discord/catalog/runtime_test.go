package catalog

import (
	"fmt"
	"sync"
	"testing"

	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	discordpluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/discord/pluginbridge"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
)

func TestRuntimePublishesCoherentSnapshotsConcurrently(t *testing.T) {
	runtime := &Runtime{}
	const iterations = 10_000
	var readers sync.WaitGroup
	readers.Add(2)
	for range 2 {
		go func() {
			defer readers.Done()
			for range iterations {
				snapshot := runtime.Snapshot()
				for _, marker := range []string{"a", "b"} {
					_, hasCommand := snapshot.Commands[marker]
					_, hasPlugin := snapshot.PluginCommands[marker]
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
		runtime.snapshot.Store(&Snapshot{
			Modules:               map[string]moduleapi.Info{marker: {ID: marker, Name: fmt.Sprintf("module-%s", marker)}},
			Commands:              map[string]slashcmd.Command{marker: {Name: marker}},
			PluginCommands:        map[string]discordpluginbridge.Route{marker: {PluginID: marker}},
			PluginUserCommands:    map[string]discordpluginbridge.Route{},
			PluginMessageCommands: map[string]discordpluginbridge.Route{},
			PluginRoutes:          map[string]discordpluginbridge.Route{},
		})
	}
	readers.Wait()
}
