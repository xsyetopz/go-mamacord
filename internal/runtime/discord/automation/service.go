package automation

import (
	"context"
	"log/slog"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/snowflake/v2"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	storage "github.com/xsyetopz/go-mamacord/internal/storage"
)

type Service struct {
	logger        *slog.Logger
	i18n          i18n.Registry
	reminderStore storage.ReminderStore
	userSettings  storage.UserSettingsStore
	client        *bot.Client
	incFailure    func()
}

func NewService(logger *slog.Logger, registry i18n.Registry, reminders storage.ReminderStore, settings storage.UserSettingsStore, client *bot.Client, incFailure func()) *Service {
	return &Service{logger: logger, i18n: registry, reminderStore: reminders, userSettings: settings, client: client, incFailure: incFailure}
}

func (service *Service) PollDue(ctx context.Context, leaseID string) {
	service.reminders().PollDue(ctx, leaseID)
}

func (service *Service) reminders() Reminders {
	return Reminders{
		Logger: service.logger, I18n: service.i18n, ReminderStore: service.reminderStore,
		UserSettings: service.userSettings, Client: service.client, DMChannels: service, IncFailure: service.incFailure,
	}
}

func (service *Service) EnsureDMChannel(ctx context.Context, userID uint64) (uint64, error) {
	setting, ok, err := service.userSettings.GetUserSettings(ctx, userID)
	if err != nil {
		return 0, err
	}
	if ok && setting.DMChannelID != nil && *setting.DMChannelID != 0 {
		return *setting.DMChannelID, nil
	}
	dm, err := service.client.Rest.CreateDMChannel(snowflake.ID(userID))
	if err != nil {
		return 0, err
	}
	channelID := uint64(dm.ID())
	if channelID != 0 {
		_ = service.userSettings.UpsertUserDMChannelID(ctx, userID, channelID)
	}
	return channelID, nil
}
