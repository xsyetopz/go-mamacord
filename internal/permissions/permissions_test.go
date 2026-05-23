package permissions_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/permissions"
)

func TestEffective(t *testing.T) {
	t.Parallel()

	req := permissions.Permissions{Storage: permissions.StoragePermissions{KV: true, UserSettings: true, CheckIns: true, Reminders: true}}
	grant := permissions.Permissions{Storage: permissions.StoragePermissions{KV: false, UserSettings: true, CheckIns: false, Reminders: true}}
	eff := permissions.Effective(req, grant)
	if eff.Storage.KV {
		t.Fatalf("expected kv denied")
	}
	if !eff.Storage.UserSettings {
		t.Fatalf("expected user_settings allowed")
	}
	if eff.Storage.CheckIns {
		t.Fatalf("expected checkins denied")
	}
	if !eff.Storage.Reminders {
		t.Fatalf("expected reminders allowed")
	}

	grant.Storage.KV = true
	eff = permissions.Effective(req, grant)
	if !eff.Storage.KV {
		t.Fatalf("expected kv allowed")
	}

	req.Storage.KV = false
	eff = permissions.Effective(req, grant)
	if eff.Storage.KV {
		t.Fatalf("expected kv denied when not requested")
	}

	req = permissions.Permissions{
		Discord: permissions.DiscordPermissions{Messages: true},
		Network: permissions.NetworkPermissions{HTTP: true},
		Automation: permissions.AutomationPermissions{
			Jobs: true,
			Events: permissions.AutomationEventPermissions{
				MemberJoinLeave: true,
				Moderation:      true,
			},
		},
	}
	grant = permissions.Permissions{
		Discord: permissions.DiscordPermissions{Messages: true},
		Network: permissions.NetworkPermissions{HTTP: false},
		Automation: permissions.AutomationPermissions{
			Jobs: false,
			Events: permissions.AutomationEventPermissions{
				MemberJoinLeave: true,
				Moderation:      false,
			},
		},
	}
	eff = permissions.Effective(req, grant)
	if !eff.Discord.Messages {
		t.Fatalf("expected messages allowed")
	}
	if eff.Network.HTTP {
		t.Fatalf("expected http denied")
	}
	if eff.Automation.Jobs {
		t.Fatalf("expected jobs denied")
	}
	if !eff.Automation.Events.MemberJoinLeave {
		t.Fatalf("expected member_join_leave allowed")
	}
	if eff.Automation.Events.Moderation {
		t.Fatalf("expected moderation denied")
	}
}

func TestPolicyGranted(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	p := filepath.Join(dir, "permissions.json")

	if err := os.WriteFile(p, []byte(`{
  "defaults": { "storage": { "kv": false, "user_settings": false }, "discord": { "messages": false } },
  "plugins": {
    "a": { "storage": { "kv": true, "user_settings": true, "checkins": true, "reminders": true }, "discord": { "messages": true }, "network": { "http": true } }
  }
}`), 0o600); err != nil {
		t.Fatalf("write file: %v", err)
	}

	pol, err := permissions.LoadPolicyFile(p)
	if err != nil {
		t.Fatalf("LoadPolicyFile: %v", err)
	}

	if pol.Granted("x").Storage.KV {
		t.Fatalf("expected default kv denied")
	}
	if !pol.Granted("a").Storage.KV {
		t.Fatalf("expected plugin override kv allowed")
	}
	if !pol.Granted("a").Storage.UserSettings {
		t.Fatalf("expected plugin override user_settings allowed")
	}
	if !pol.Granted("a").Storage.CheckIns {
		t.Fatalf("expected plugin override checkins allowed")
	}
	if !pol.Granted("a").Storage.Reminders {
		t.Fatalf("expected plugin override reminders allowed")
	}
	if !pol.Granted("a").Discord.Messages {
		t.Fatalf("expected plugin override messages allowed")
	}
	if !pol.Granted("a").Network.HTTP {
		t.Fatalf("expected plugin override http allowed")
	}
}
