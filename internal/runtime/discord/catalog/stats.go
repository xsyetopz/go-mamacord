package catalog

import commandapi "github.com/xsyetopz/go-mamacord/internal/commands/api"

type Stats struct {
	Ready               bool
	ModuleCount         int
	EnabledModuleCount  int
	PluginCount         int
	EnabledPluginCount  int
	BuiltinCommandCount int
	SlashCommandCount   int
	UserCommandCount    int
	MessageCommandCount int
}

func RuntimeStats(
	modules map[string]commandapi.ModuleInfo,
	builtinCommands []commandapi.SlashCommand,
	slashPlugins int,
	userPlugins int,
	messagePlugins int,
) Stats {
	stats := Stats{
		BuiltinCommandCount: len(builtinCommands),
		SlashCommandCount:   len(builtinCommands) + slashPlugins,
		UserCommandCount:    userPlugins,
		MessageCommandCount: messagePlugins,
	}
	for _, info := range modules {
		stats.ModuleCount++
		if info.Enabled {
			stats.EnabledModuleCount++
		}
		if info.Runtime != commandapi.ModuleRuntimeLua {
			continue
		}
		stats.PluginCount++
		if info.Enabled {
			stats.EnabledPluginCount++
		}
	}
	return stats
}
