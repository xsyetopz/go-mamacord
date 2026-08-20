package executor

import (
	"context"
	"errors"

	"github.com/disgoorg/disgo/bot"
)

type Discord struct {
	ClientProvider      func() *bot.Client
	EnsureDMChannelFunc func(ctx context.Context, userID uint64) (uint64, error)
}

func (e Discord) Client() *bot.Client {
	if e.ClientProvider == nil {
		return nil
	}
	return e.ClientProvider()
}

func (e Discord) EnsureDMChannel(ctx context.Context, userID uint64) (uint64, error) {
	if e.EnsureDMChannelFunc == nil {
		return 0, errors.New("dm channel service unavailable")
	}
	return e.EnsureDMChannelFunc(ctx, userID)
}
