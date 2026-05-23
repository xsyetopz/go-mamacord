package automation

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/google/uuid"

	commandapi "github.com/xsyetopz/go-mamacord/internal/commands/api"
	"github.com/xsyetopz/go-mamacord/internal/i18n"
	"github.com/xsyetopz/go-mamacord/internal/runtime/discord/interactions"
	"github.com/xsyetopz/go-mamacord/internal/scheduling"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
	"github.com/xsyetopz/go-mamacord/internal/timezone"
)

const (
	reminderClaimLimit     = 10
	reminderLeaseDuration  = 30 * time.Second
	reminderPollInterval   = 5 * time.Second
	reminderMaxFailures    = 3
	reminderDefaultLocale  = discord.LocaleEnglishUS
	reminderMessageMaxNote = 120
)

type DMEnsurer interface {
	EnsureDMChannel(ctx context.Context, userID uint64) (uint64, error)
}

type Reminders struct {
	Logger     *slog.Logger
	I18n       i18n.Registry
	Store      commandapi.Store
	Client     *bot.Client
	DMChannels DMEnsurer
	IncFailure func()
}

func (r Reminders) Start(ctx context.Context) {
	if r.Client == nil || r.Store == nil {
		return
	}
	go r.run(ctx, uuid.NewString())
}

func (r Reminders) run(ctx context.Context, leaseID string) {
	ticker := time.NewTicker(reminderPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.tick(ctx, leaseID)
		}
	}
}

func (r Reminders) tick(ctx context.Context, leaseID string) {
	now := time.Now().UTC()
	reminders, err := r.Store.Reminders().ClaimDueReminders(
		ctx,
		now,
		leaseID,
		reminderLeaseDuration,
		reminderClaimLimit,
	)
	if err != nil {
		r.incFailure()
		r.logger().ErrorContext(ctx, "claim due reminders failed", slog.String("err", err.Error()))
		return
	}

	for _, reminder := range reminders {
		r.runOne(ctx, leaseID, now, reminder)
	}
}

func (r Reminders) runOne(ctx context.Context, leaseID string, now time.Time, reminder store.Reminder) {
	t := commandapi.Translator{
		Registry: r.I18n,
		Locale:   reminderDefaultLocale,
		PluginID: "wellness",
		UserID:   reminder.UserID,
	}

	loc := r.userLocation(ctx, reminder.UserID)
	sched, err := scheduling.ParseSchedule(reminder.Schedule)
	if err != nil {
		r.incFailure()
		r.logger().ErrorContext(
			ctx,
			"invalid reminder schedule",
			slog.String("err", err.Error()),
			slog.String("id", reminder.ID),
		)
		_ = r.finish(ctx, leaseID, reminder, now, now.Add(365*24*time.Hour), reminder.FailureCount+1, false)
		return
	}

	next := sched.Next(now, loc)
	if next.IsZero() || !next.After(now) {
		next = now.Add(time.Hour)
	}

	failureCount := reminder.FailureCount
	enabled := reminder.Enabled
	if enabled {
		if sendErr := r.send(ctx, t, reminder); sendErr != nil {
			r.incFailure()
			failureCount++
			if failureCount >= reminderMaxFailures {
				enabled = false
			}
			r.logger().WarnContext(
				ctx,
				"send reminder failed",
				slog.String("id", reminder.ID),
				slog.String("err", sendErr.Error()),
				slog.Int("failures", failureCount),
				slog.Bool("enabled", enabled),
			)
		} else {
			failureCount = 0
		}
	}

	if !enabled {
		next = now.Add(365 * 24 * time.Hour)
	}

	_ = r.finish(ctx, leaseID, reminder, now, next, failureCount, enabled)
}

func (r Reminders) finish(
	ctx context.Context,
	leaseID string,
	reminder store.Reminder,
	lastRunAt time.Time,
	nextRunAt time.Time,
	failureCount int,
	enabled bool,
) error {
	if r.Store == nil {
		return errors.New("store not configured")
	}
	return r.Store.Reminders().FinishReminderRun(ctx, reminder.ID, leaseID, lastRunAt, nextRunAt, failureCount, enabled)
}

func (r Reminders) userLocation(ctx context.Context, userID uint64) *time.Location {
	if r.Store == nil {
		return time.UTC
	}
	settings, ok, err := r.Store.UserSettings().GetUserSettings(ctx, userID)
	if err == nil && ok && strings.TrimSpace(settings.Timezone) != "" {
		if loc, _, loadErr := timezone.LoadLocation(settings.Timezone); loadErr == nil {
			return loc
		}
	}
	return time.UTC
}

func (r Reminders) send(ctx context.Context, t commandapi.Translator, reminder store.Reminder) error {
	if r.Client == nil {
		return errors.New("discord client not configured")
	}

	kindText := reminderKindText(t, reminder.Kind)
	note := strings.TrimSpace(reminder.Note)
	if len(note) > reminderMessageMaxNote {
		note = note[:reminderMessageMaxNote]
	}

	body := t.S("wellness.reminder.fire", map[string]any{
		"Kind": kindText,
		"Note": note,
	})

	msg := interactions.NoticeMessage(interactions.KindInfo, "", body, false)

	switch reminder.Delivery {
	case store.ReminderDeliveryChannel:
		if reminder.ChannelID == nil || *reminder.ChannelID == 0 {
			return errors.New("missing channel_id for channel delivery")
		}
		_, err := r.Client.Rest.CreateMessage(snowflake.ID(*reminder.ChannelID), msg)
		return err
	case store.ReminderDeliveryDM:
		fallthrough
	default:
		if r.DMChannels == nil {
			return errors.New("dm channel service not configured")
		}
		chID, err := r.DMChannels.EnsureDMChannel(ctx, reminder.UserID)
		if err != nil {
			return err
		}
		_, err = r.Client.Rest.CreateMessage(snowflake.ID(chID), msg)
		return err
	}
}

func reminderKindText(t commandapi.Translator, kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "hydrate":
		return t.S("wellness.reminder.kind.hydrate", nil)
	case "stretch":
		return t.S("wellness.reminder.kind.stretch", nil)
	case "breathe":
		return t.S("wellness.reminder.kind.breathe", nil)
	case "meds":
		return t.S("wellness.reminder.kind.meds", nil)
	case "sleep":
		return t.S("wellness.reminder.kind.sleep", nil)
	case "checkin":
		return t.S("wellness.reminder.kind.checkin", nil)
	default:
		return strings.TrimSpace(kind)
	}
}

func (r Reminders) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.Default()
}

func (r Reminders) incFailure() {
	if r.IncFailure != nil {
		r.IncFailure()
	}
}
