package plugin

import (
	"os"
	"strings"
	"testing"
)

func TestPluginPackageDoesNotDeclareExecutorStruct(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("types.go")
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("read types.go: %v", err)
	}
	if strings.Contains(string(bytes), "type Executor struct") {
		t.Fatal("plugin package still declares Executor instead of moving bridge implementation into a focused package")
	}
}

func TestPluginPackageDoesNotDeclareSlashInteractionStruct(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("slash_interaction.go")
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("read slash_interaction.go: %v", err)
	}
	if strings.Contains(string(bytes), "type SlashInteraction struct") {
		t.Fatal("plugin package still declares SlashInteraction instead of moving bridge implementation into a focused package")
	}
}

func TestPluginPackageDoesNotDeclareRouteStruct(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("types.go")
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("read types.go: %v", err)
	}
	if strings.Contains(string(bytes), "type Route struct") {
		t.Fatal("plugin package still declares Route instead of moving bridge routing into a focused package")
	}
}

func TestPluginPackageDoesNotDeclareAutomationStruct(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("automation.go")
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("read automation.go: %v", err)
	}
	if strings.Contains(string(bytes), "type Automation struct") {
		t.Fatal("plugin package still declares Automation instead of moving bridge runtime wiring into a focused package")
	}
}
