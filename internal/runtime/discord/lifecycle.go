package discordruntime

import (
	"context"
	discordgateway "github.com/xsyetopz/go-mamacord/internal/runtime/discord/gateway"
)

func (b *Bot) Start(ctx context.Context) error {
	b.owner.Resolve(ctx, b.client, b.logger)

	if err := b.reloadModules(ctx); err != nil {
		return err
	}

	if err := startSelectedRoles(
		ctx,
		b.enableGateway,
		func(ctx context.Context) error {
			return b.client.OpenGateway(ctx)
		},
		b.enableScheduler,
		func(ctx context.Context) {
			if b.scheduler != nil {
				b.scheduler.Start(ctx)
			}
		},
	); err != nil {
		return discordgateway.DecorateOpenError(err, requestedGatewayIntentsMask())
	}
	b.ready.Store(true)
	return nil
}

func (b *Bot) Close(ctx context.Context) {
	b.ready.Store(false)
	if b.pluginHost != nil {
		_ = b.pluginHost.Close(ctx)
	}
	if b.client != nil {
		b.client.Close(ctx)
	}
	if b.enableScheduler && b.scheduler != nil {
		b.scheduler.Stop()
	}
}

func startSelectedRoles(
	ctx context.Context,
	enableGateway bool,
	openGateway func(context.Context) error,
	enableScheduler bool,
	startScheduler func(context.Context),
) error {
	if enableGateway && openGateway != nil {
		if err := openGateway(ctx); err != nil {
			return err
		}
	}
	if enableScheduler && startScheduler != nil {
		startScheduler(ctx)
	}
	return nil
}

func (b *Bot) registerCommands(ctx context.Context) error {
	return b.commandRegistrar().Register(ctx, b.commandRegistrationMode, b.devGuildID, b.commandGuildIDs)
}
