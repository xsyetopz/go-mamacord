package catalog

import (
	"testing"

	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
)

func TestRuntimeStatsCountsBuiltinAndPluginModules(t *testing.T) {
	t.Parallel()

	stats := RuntimeStats(
		map[string]moduleapi.Info{
			"core": {
				ID:      "core",
				Runtime: moduleapi.RuntimeGo,
				Enabled: true,
			},
			"fun": {
				ID:      "fun",
				Runtime: moduleapi.RuntimeLua,
				Enabled: true,
			},
			"info": {
				ID:      "info",
				Runtime: moduleapi.RuntimeLua,
				Enabled: false,
			},
		},
		[]slashcmd.Command{{Name: "ping"}, {Name: "help"}},
		3,
		1,
		2,
	)

	if stats.ModuleCount != 3 {
		t.Fatalf("ModuleCount = %d, want 3", stats.ModuleCount)
	}
	if stats.EnabledModuleCount != 2 {
		t.Fatalf("EnabledModuleCount = %d, want 2", stats.EnabledModuleCount)
	}
	if stats.PluginCount != 2 {
		t.Fatalf("PluginCount = %d, want 2", stats.PluginCount)
	}
	if stats.EnabledPluginCount != 1 {
		t.Fatalf("EnabledPluginCount = %d, want 1", stats.EnabledPluginCount)
	}
	if stats.BuiltinCommandCount != 2 {
		t.Fatalf("BuiltinCommandCount = %d, want 2", stats.BuiltinCommandCount)
	}
	if stats.SlashCommandCount != 5 {
		t.Fatalf("SlashCommandCount = %d, want 5", stats.SlashCommandCount)
	}
	if stats.UserCommandCount != 1 {
		t.Fatalf("UserCommandCount = %d, want 1", stats.UserCommandCount)
	}
	if stats.MessageCommandCount != 2 {
		t.Fatalf("MessageCommandCount = %d, want 2", stats.MessageCommandCount)
	}
}
