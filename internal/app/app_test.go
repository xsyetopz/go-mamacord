package app

import (
	"io"
	"log/slog"
	"testing"

	"github.com/xsyetopz/go-mamacord/internal/config"
)

func TestNewRejectsProdModeWithUnsignedPlugins(t *testing.T) {
	t.Parallel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_, err := New(Dependencies{
		Logger: logger,
		Config: config.Config{
			ProdMode:             true,
			AllowUnsignedPlugins: true,
		},
	})
	if err == nil {
		t.Fatalf("expected prod-mode plugin trust validation error")
	}
}
