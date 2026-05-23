package discordruntime

import (
	"context"

	"github.com/disgoorg/snowflake/v2"

	discordautomation "github.com/xsyetopz/go-mamacord/internal/runtime/discord/automation"
)

func (b *Bot) startReminderScheduler(ctx context.Context) {
	b.reminders().Start(ctx)
}

func (b *Bot) reminders() discordautomation.Reminders {
	return discordautomation.Reminders{
		Logger:     b.logger,
		I18n:       b.i18n,
		Store:      b.store,
		Client:     b.client,
		DMChannels: b,
		IncFailure: b.incReminderFailure,
	}
}

func (b *Bot) EnsureDMChannel(ctx context.Context, userID uint64) (uint64, error) {
	return b.ensureDMChannel(ctx, userID)
}

func (b *Bot) ensureDMChannel(ctx context.Context, userID uint64) (uint64, error) {
	settingsStore := b.store.UserSettings()
	setting, ok, err := settingsStore.GetUserSettings(ctx, userID)
	if err != nil {
		return 0, err
	}
	if ok && setting.DMChannelID != nil && *setting.DMChannelID != 0 {
		return *setting.DMChannelID, nil
	}

	dm, err := b.client.Rest.CreateDMChannel(snowflake.ID(userID))
	if err != nil {
		return 0, err
	}
	chID := uint64(dm.ID())
	if chID != 0 {
		_ = settingsStore.UpsertUserDMChannelID(ctx, userID, chID)
	}
	return chID, nil
}
