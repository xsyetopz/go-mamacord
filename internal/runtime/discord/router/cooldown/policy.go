package cooldown

import (
	"github.com/xsyetopz/go-mamacord/internal/runtime/plugins/customid"
	"strings"
	"time"
)

type Policy struct {
	slashCooldown          time.Duration
	componentCooldown      time.Duration
	modalCooldown          time.Duration
	slashBypass            map[string]struct{}
	slashCooldownOverrides map[string]time.Duration
}

func NewPolicy(slash, component, modal time.Duration, bypass []string, overrides map[string]time.Duration) Policy {
	policy := Policy{
		slashCooldown: slash, componentCooldown: component, modalCooldown: modal,
		slashBypass: map[string]struct{}{}, slashCooldownOverrides: map[string]time.Duration{},
	}
	for _, name := range bypass {
		if value := strings.ToLower(strings.TrimSpace(name)); value != "" {
			policy.slashBypass[value] = struct{}{}
		}
	}
	for key, duration := range overrides {
		if value := strings.ToLower(strings.TrimSpace(key)); value != "" {
			policy.slashCooldownOverrides[value] = duration
		}
	}
	return policy
}

func (policy Policy) CommandCooldown(cmdName string) time.Duration {
	if policy.slashCooldown <= 0 && len(policy.slashBypass) == 0 && len(policy.slashCooldownOverrides) == 0 {
		return 0
	}
	key := strings.ToLower(strings.TrimSpace(cmdName))
	if key == "" {
		return 0
	}

	root := key
	if idx := strings.IndexByte(root, ':'); idx >= 0 {
		root = root[:idx]
	}

	if _, ok := policy.slashBypass[root]; ok {
		return 0
	}

	if d, ok := policy.slashCooldownOverrides[key]; ok {
		return d
	}
	if root != key {
		if d, ok := policy.slashCooldownOverrides[root]; ok {
			return d
		}
	}

	if policy.slashCooldown <= 0 {
		return 0
	}
	return policy.slashCooldown
}

func (policy Policy) ComponentCooldown(_ string) time.Duration {
	if policy.componentCooldown <= 0 {
		return 0
	}
	return policy.componentCooldown
}

func (policy Policy) ModalCooldown(_ string) time.Duration {
	if policy.modalCooldown <= 0 {
		return 0
	}
	return policy.modalCooldown
}

func CooldownSeconds(remaining time.Duration) int {
	secs := int(remaining.Round(time.Second).Seconds())
	return max(1, secs)
}

func ComponentCooldownKey(customID string) string {
	cid := strings.TrimSpace(customID)
	if cid == "" {
		return "component"
	}
	if pid, _, ok := customid.Parse(cid); ok {
		return "component:" + pid
	}
	if strings.HasPrefix(cid, "mamacord:") {
		return "component:mamacord"
	}
	return "component:other"
}

func ModalCooldownKey(customID string) string {
	cid := strings.TrimSpace(customID)
	if cid == "" {
		return "modal"
	}
	if pid, _, ok := customid.Parse(cid); ok {
		return "modal:" + pid
	}
	if strings.HasPrefix(cid, "mamacord:") {
		return "modal:mamacord"
	}
	return "modal:other"
}
