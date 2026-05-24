package luaplugin_test

import (
	"os"
	"strings"
	"testing"
)

func TestVMGoDoesNotDeclareInteractionInterface(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("vm.go")
	if err != nil {
		t.Fatalf("read vm.go: %v", err)
	}
	if strings.Contains(string(bytes), "type Interaction interface") {
		t.Fatal("vm.go still declares the interaction interface instead of using a shared bridge contract")
	}
}

func TestVMGoDoesNotDeclareDiscordInterface(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("vm.go")
	if err != nil {
		t.Fatalf("read vm.go: %v", err)
	}
	if strings.Contains(string(bytes), "type Discord interface") {
		t.Fatal("vm.go still declares the Discord bridge interface instead of using a dedicated bridge contract file")
	}
}

func TestVMGoDoesNotExposeLooseDiscordOptionField(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("vm.go")
	if err != nil {
		t.Fatalf("read vm.go: %v", err)
	}
	if strings.Contains(string(bytes), "Discord     Discord") {
		t.Fatal("vm.go still exposes a loose Discord option field instead of an explicit bridge dependency")
	}
}
