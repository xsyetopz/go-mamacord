package discordruntime

import (
	"errors"
	"strings"
)

func validateNewDeps(deps Dependencies) error {
	if deps.Logger == nil {
		return errors.New("logger is required")
	}
	if deps.EnableGateway == false && deps.EnableScheduler == false {
		return errors.New("at least one runtime role must be enabled")
	}
	if (deps.EnableGateway || deps.EnableScheduler) && strings.TrimSpace(deps.Token) == "" {
		return errors.New("discord token is required")
	}
	if deps.Restrictions == nil {
		return errors.New("restriction store is required")
	}
	if deps.PluginKV == nil {
		return errors.New("plugin kv store is required")
	}
	if deps.ModuleStates == nil {
		return errors.New("module state store is required")
	}
	if deps.UserSettings == nil {
		return errors.New("user settings store is required")
	}
	if deps.Reminders == nil {
		return errors.New("reminder store is required")
	}
	if deps.Guilds == nil {
		return errors.New("guild store is required")
	}
	if deps.Users == nil {
		return errors.New("user store is required")
	}
	if deps.GuildMembers == nil {
		return errors.New("guild member store is required")
	}
	if (strings.TrimSpace(deps.BundledPluginsDir) != "" || strings.TrimSpace(deps.UserPluginsDir) != "") && deps.PluginStore == nil {
		return errors.New("plugin store is required when plugin directories are configured")
	}
	return nil
}

func normalizeRuntimeRoleDeps(enableGateway bool, enableScheduler bool) (bool, bool) {
	if !enableGateway && !enableScheduler {
		return true, true
	}
	return enableGateway, enableScheduler
}

func normalizeCommandRegistrationMode(mode string) (string, error) {
	m := strings.ToLower(strings.TrimSpace(mode))
	if m == "" {
		return commandRegistrationModeGlobal, nil
	}
	switch m {
	case commandRegistrationModeGlobal, commandRegistrationModeGuilds, commandRegistrationModeHybrid:
		return m, nil
	default:
		return "", errors.New("invalid command registration mode")
	}
}

func cloneUint64Pointer(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}
