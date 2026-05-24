package discordruntime

import (
	"errors"
	"strings"
	"time"
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
	if deps.Store == nil {
		return errors.New("store is required")
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

func buildSlashBypass(names []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, name := range names {
		n := strings.ToLower(strings.TrimSpace(name))
		if n == "" {
			continue
		}
		out[n] = struct{}{}
	}
	return out
}

func cloneCooldownOverrides(in map[string]time.Duration) map[string]time.Duration {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]time.Duration, len(in))
	for k, v := range in {
		key := strings.ToLower(strings.TrimSpace(k))
		if key == "" {
			continue
		}
		out[key] = v
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
