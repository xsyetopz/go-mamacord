package automation

import (
	"os"
	"strings"
	"testing"
)

func TestRemindersGoDoesNotOwnTickerLifecycle(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("reminders.go")
	if err != nil {
		t.Fatalf("read reminders.go: %v", err)
	}
	text := string(bytes)
	if strings.Contains(text, "func (r Reminders) Start(") {
		t.Fatal("reminders.go still owns start lifecycle instead of the unified scheduler")
	}
	if strings.Contains(text, "time.NewTicker(") {
		t.Fatal("reminders.go still owns ticker lifecycle instead of the unified scheduler")
	}
	if !strings.Contains(text, "func (r Reminders) PollDue(") {
		t.Fatal("reminders.go should expose one reminder polling unit for the unified scheduler")
	}
}
