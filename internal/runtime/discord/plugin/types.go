package plugin

import (
	"context"
	"errors"
	"log/slog"

	"github.com/disgoorg/disgo/bot"
	pluginhost "github.com/xsyetopz/go-mamacord/internal/runtime/plugins"
)

type Route struct {
	Host     *pluginhost.Host
	PluginID string
}

type Target = Route

type Executor struct {
	ClientProvider      func() *bot.Client
	EnsureDMChannelFunc func(ctx context.Context, userID uint64) (uint64, error)
}

func (e Executor) client() *bot.Client {
	if e.ClientProvider == nil {
		return nil
	}
	return e.ClientProvider()
}

func (e Executor) ensureDMChannel(ctx context.Context, userID uint64) (uint64, error) {
	if e.EnsureDMChannelFunc == nil {
		return 0, errors.New("dm channel service unavailable")
	}
	return e.EnsureDMChannelFunc(ctx, userID)
}

func defaultLogger(logger *slog.Logger) *slog.Logger {
	if logger != nil {
		return logger
	}
	return slog.Default()
}
