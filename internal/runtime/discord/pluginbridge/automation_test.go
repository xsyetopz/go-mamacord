package pluginbridge

import (
	"os"
	"strings"
	"testing"
)

func TestAutomationGoDoesNotOwnCronLifecycle(t *testing.T) {
	t.Parallel()

	bytes, err := os.ReadFile("automation.go")
	if err != nil {
		t.Fatalf("read automation.go: %v", err)
	}
	text := string(bytes)
	if strings.Contains(text, "github.com/robfig/cron/v3") {
		t.Fatal("automation.go still imports cron instead of letting the unified scheduler own cron lifecycle")
	}
	if strings.Contains(text, "cron *cron.Cron") {
		t.Fatal("automation.go still owns a cron instance instead of using the unified scheduler")
	}
	for _, symbol := range []string{
		"func (p *Automation) Start(",
		"func (p *Automation) Stop(",
		"func (p *Automation) Restart(",
	} {
		if strings.Contains(text, symbol) {
			t.Fatalf("automation.go still exposes direct cron lifecycle method %q", symbol)
		}
	}
	if !strings.Contains(text, "func (p *Automation) RunJob(") {
		t.Fatal("automation.go should expose job execution to the unified scheduler")
	}
}
