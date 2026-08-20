package cooldown

import (
	"testing"
	"time"
)

func TestPolicySelectsBypassOverridesAndDefaults(t *testing.T) {
	t.Parallel()
	policy := NewPolicy(5*time.Second, 2*time.Second, 3*time.Second, []string{" PiNg "}, map[string]time.Duration{
		"BAN:HARD": 7 * time.Second,
		"ROOT":     9 * time.Second,
	})
	for _, test := range []struct {
		name string
		key  string
		want time.Duration
	}{
		{name: "bypass", key: "PING:now", want: 0},
		{name: "exact override", key: "ban:hard", want: 7 * time.Second},
		{name: "root override", key: "root:child", want: 9 * time.Second},
		{name: "default", key: "other", want: 5 * time.Second},
		{name: "empty", key: " ", want: 0},
	} {
		if got := policy.CommandCooldown(test.key); got != test.want {
			t.Errorf("%s: duration = %s, want %s", test.name, got, test.want)
		}
	}
	if got := policy.ComponentCooldown("ignored"); got != 2*time.Second {
		t.Fatalf("component duration = %s", got)
	}
	if got := policy.ModalCooldown("ignored"); got != 3*time.Second {
		t.Fatalf("modal duration = %s", got)
	}
}

func TestInteractionKeysAndDisplayedSeconds(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name string
		got  string
		want string
	}{
		{name: "empty component", got: ComponentCooldownKey(""), want: "component"},
		{name: "plugin component", got: ComponentCooldownKey("mamacord:pl:fun:next"), want: "component:fun"},
		{name: "builtin component", got: ComponentCooldownKey("mamacord:about"), want: "component:mamacord"},
		{name: "other component", got: ComponentCooldownKey("other"), want: "component:other"},
		{name: "empty modal", got: ModalCooldownKey(""), want: "modal"},
		{name: "plugin modal", got: ModalCooldownKey("mamacord:pl:fun:edit"), want: "modal:fun"},
	} {
		if test.got != test.want {
			t.Errorf("%s: key = %q, want %q", test.name, test.got, test.want)
		}
	}
	if got := CooldownSeconds(100 * time.Millisecond); got != 1 {
		t.Fatalf("short duration seconds = %d", got)
	}
	if got := CooldownSeconds(1600 * time.Millisecond); got != 2 {
		t.Fatalf("rounded duration seconds = %d", got)
	}
}
