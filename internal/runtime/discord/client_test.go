package discordruntime

import (
	"os"
	"strings"
	"testing"
)

func TestClientGoDoesNotUseLuaBridgeType(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("client.go")
	if err != nil {
		t.Fatalf("read client.go: %v", err)
	}
	if strings.Contains(string(bytes), "luaplugin.Bridge") {
		t.Fatal("client.go still constructs plugins with luaplugin.Bridge instead of the shared plugin bridge type")
	}
}
