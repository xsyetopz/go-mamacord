package catalog

import (
	"slices"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/commands"
)

func TestBuiltinCommandsAdaptsCoreDefinitionsToRuntimeCommands(t *testing.T) {
	t.Parallel()

	var core commands.ModuleDescriptor
	for _, desc := range commands.Catalog() {
		if desc.ID == "core" {
			core = desc
			break
		}
	}
	if core.ID == "" {
		t.Fatalf("core module not found")
	}

	cmds := BuiltinCommands(core)
	if len(cmds) != 2 {
		t.Fatalf("BuiltinCommands(core) len = %d, want 2", len(cmds))
	}
	for _, cmd := range cmds {
		if cmd.Handle == nil {
			t.Fatalf("command %q missing runtime handler", cmd.Name)
		}
	}
}

func TestBuiltinCommandsAdaptsAdminModulesDefinitionToRuntimeCommand(t *testing.T) {
	t.Parallel()

	var admin commands.ModuleDescriptor
	for _, desc := range commands.Catalog() {
		if desc.ID == "admin" {
			admin = desc
			break
		}
	}
	if admin.ID == "" {
		t.Fatalf("admin module not found")
	}

	cmds := BuiltinCommands(admin)
	found := false
	for _, cmd := range cmds {
		if cmd.Name != "modules" {
			continue
		}
		found = true
		if cmd.Handle == nil {
			t.Fatalf("admin modules command missing runtime handler")
		}
	}
	if !found {
		t.Fatalf("admin modules command not adapted")
	}
}

func TestBuiltinCommandsAdaptsAllAdminDefinitionsToRuntimeCommands(t *testing.T) {
	t.Parallel()

	var admin commands.ModuleDescriptor
	for _, desc := range commands.Catalog() {
		if desc.ID == "admin" {
			admin = desc
			break
		}
	}
	if admin.ID == "" {
		t.Fatalf("admin module not found")
	}

	cmds := BuiltinCommands(admin)
	names := make([]string, 0, len(cmds))
	for _, cmd := range cmds {
		names = append(names, cmd.Name)
		if cmd.Handle == nil {
			t.Fatalf("command %q missing runtime handler", cmd.Name)
		}
	}
	slices.Sort(names)

	want := []string{"block", "modules", "plugins", "unblock"}
	if !slices.Equal(names, want) {
		t.Fatalf("BuiltinCommands(admin) names = %#v, want %#v", names, want)
	}
}
