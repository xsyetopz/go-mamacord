package gateway

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/disgoorg/disgo/gateway"
	"github.com/gorilla/websocket"
)

func TestDecorateGatewayOpenError_4014AddsIntentHelp(t *testing.T) {
	base := &websocket.CloseError{Code: 4014, Text: "Disallowed intent(s)."}
	err := fmt.Errorf("failed to open gateway connection: %w", base)

	intents := gateway.IntentGuilds | gateway.IntentGuildMembers
	got := DecorateOpenError(err, intents)
	if errors.Is(got, base) == false {
		t.Fatalf("expected decorated error to wrap base close error")
	}
	msg := got.Error()
	if !strings.Contains(msg, "4014") {
		t.Fatalf("expected message to contain 4014, got %q", msg)
	}
	if !strings.Contains(msg, "Disallowed intent") {
		t.Fatalf("expected message to contain disallowed intent text, got %q", msg)
	}
	if !strings.Contains(msg, "GuildMembers") {
		t.Fatalf("expected message to mention GuildMembers intent, got %q", msg)
	}
	if !strings.Contains(msg, "Server Members Intent") {
		t.Fatalf("expected message to mention Server Members Intent toggle, got %q", msg)
	}
}

func TestDecorateGatewayOpenError_Non4014NoChange(t *testing.T) {
	err := fmt.Errorf("failed: %w", &websocket.CloseError{Code: 4004, Text: "Authentication failed."})
	got := DecorateOpenError(err, gateway.IntentGuilds)
	if got != err {
		t.Fatalf("expected original error instance for non-4014, got %v", got)
	}
}

func TestDecorateGatewayOpenError_UnwrappedNoChange(t *testing.T) {
	err := errors.New("some other failure")
	got := DecorateOpenError(err, gateway.IntentGuilds)
	if got != err {
		t.Fatalf("expected original error instance for non-close errors, got %v", got)
	}
}
