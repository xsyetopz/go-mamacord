package catalog

import (
	"sort"
	"strings"

	"github.com/xsyetopz/go-mamacord/internal/commands"
	"github.com/xsyetopz/go-mamacord/internal/config"
	moduleapi "github.com/xsyetopz/go-mamacord/internal/modules"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/slashcmd"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
)

const (
	ModuleSourceBuiltin = "builtin"
	ModuleSourcePlugin  = "plugins"
)

func BuiltinDefaultEnabled(desc commands.ModuleDescriptor, seed config.ModulesFile) bool {
	if !desc.Toggleable {
		return true
	}
	if entry, ok := seed.Modules[desc.ID]; ok && entry.Enabled != nil {
		return *entry.Enabled
	}
	return desc.DefaultEnabled
}

func ResolveBuiltinModuleEnabled(
	desc commands.ModuleDescriptor,
	seed config.ModulesFile,
	states map[string]store.ModuleState,
) bool {
	if !desc.Toggleable {
		return true
	}
	if state, ok := states[desc.ID]; ok {
		return state.Enabled
	}
	return BuiltinDefaultEnabled(desc, seed)
}

func PluginDefaultEnabled(moduleID string, seed config.ModulesFile) bool {
	defaultEnabled := true
	if seed.Defaults.PluginEnabled != nil {
		defaultEnabled = *seed.Defaults.PluginEnabled
	} else if seed.Defaults.UserEnabled != nil {
		defaultEnabled = *seed.Defaults.UserEnabled
	} else if seed.Defaults.OfficialEnabled != nil {
		defaultEnabled = *seed.Defaults.OfficialEnabled
	}
	if entry, ok := seed.Modules[moduleID]; ok && entry.Enabled != nil {
		defaultEnabled = *entry.Enabled
	}
	return defaultEnabled
}

func ModuleKindForPlugin(pluginID string) moduleapi.Kind {
	_ = strings.TrimSpace(pluginID)
	return moduleapi.KindPlugin
}

func SlashCommandNames(commands []slashcmd.Command) []string {
	out := make([]string, 0, len(commands))
	for _, cmd := range commands {
		if strings.TrimSpace(cmd.Name) != "" {
			out = append(out, cmd.Name)
		}
	}
	sort.Strings(out)
	return out
}

func PluginCommandNames(commands []pluginhost.Command) []string {
	out := make([]string, 0, len(commands))
	for _, cmd := range commands {
		if strings.TrimSpace(cmd.Name) != "" {
			out = append(out, cmd.Name)
		}
	}
	sort.Strings(out)
	return out
}
