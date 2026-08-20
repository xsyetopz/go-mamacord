package automationpg_test

import (
	"context"
	"testing"
	"time"

	automationstore "github.com/xsyetopz/go-mamacord/internal/storage/automation"
	postgresstore "github.com/xsyetopz/go-mamacord/internal/storage/postgres"
	pgtest "github.com/xsyetopz/go-mamacord/internal/storage/postgres/testkit"
)

func TestPostgresReminderLifecycleAndLeaseFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storeDB := newReminderTestStore(t, ctx)

	now := time.Unix(1700000000, 0).UTC()
	channelID := uint64(777)
	guildID := uint64(888)

	mustNoErr(t, storeDB.Reminders().CreateReminder(ctx, automationstore.Reminder{
		ReminderIdentity: automationstore.ReminderIdentity{
			ID:     "due",
			UserID: 42,
		},
		ReminderSchedule: automationstore.ReminderSchedule{
			Schedule: "0 * * * *",
			Kind:     "hydrate",
			Note:     "water",
		},
		ReminderDeliveryTarget: automationstore.ReminderDeliveryTarget{
			Delivery: automationstore.ReminderDeliveryDM,
		},
		ReminderState: automationstore.ReminderState{
			Enabled:   true,
			NextRunAt: now,
		},
	}), "CreateReminder(due)")

	mustNoErr(t, storeDB.Reminders().CreateReminder(ctx, automationstore.Reminder{
		ReminderIdentity: automationstore.ReminderIdentity{
			ID:     "future",
			UserID: 42,
		},
		ReminderSchedule: automationstore.ReminderSchedule{
			Schedule: "0 * * * *",
			Kind:     "stretch",
			Note:     "legs",
		},
		ReminderDeliveryTarget: automationstore.ReminderDeliveryTarget{
			Delivery:  automationstore.ReminderDeliveryChannel,
			GuildID:   &guildID,
			ChannelID: &channelID,
		},
		ReminderState: automationstore.ReminderState{
			Enabled:   true,
			NextRunAt: now.Add(2 * time.Hour),
		},
	}), "CreateReminder(future)")

	claimed, err := storeDB.Reminders().ClaimDueReminders(ctx, now, "lease-a", 30*time.Second, 10)
	mustNoErr(t, err, "ClaimDueReminders(first)")
	if len(claimed) != 1 || claimed[0].ID != "due" {
		t.Fatalf("unexpected claimed reminders: %#v", claimed)
	}

	claimedAgain, err := storeDB.Reminders().ClaimDueReminders(ctx, now.Add(10*time.Second), "lease-b", 30*time.Second, 10)
	mustNoErr(t, err, "ClaimDueReminders(second)")
	if len(claimedAgain) != 0 {
		t.Fatalf("expected active lease to block reclaim, got %#v", claimedAgain)
	}

	if err := storeDB.Reminders().FinishReminderRun(
		ctx,
		"due",
		"wrong-lease",
		now,
		now.Add(time.Hour),
		1,
		true,
	); err == nil {
		t.Fatalf("expected wrong lease to fail")
	}

	mustNoErr(
		t,
		storeDB.Reminders().FinishReminderRun(ctx, "due", "lease-a", now, now.Add(time.Hour), 1, true),
		"FinishReminderRun",
	)

	reminders, err := storeDB.Reminders().ListReminders(ctx, 42, 10)
	mustNoErr(t, err, "ListReminders")

	byID := map[string]automationstore.Reminder{}
	for _, reminder := range reminders {
		byID[reminder.ID] = reminder
	}

	dueReminder := byID["due"]
	if dueReminder.LastRunAt == nil || !dueReminder.LastRunAt.Equal(now) {
		t.Fatalf("unexpected due LastRunAt: %#v", dueReminder.LastRunAt)
	}
	if !dueReminder.NextRunAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("unexpected due NextRunAt: %s", dueReminder.NextRunAt)
	}
	if dueReminder.FailureCount != 1 {
		t.Fatalf("unexpected due FailureCount: %d", dueReminder.FailureCount)
	}

	futureReminder := byID["future"]
	if futureReminder.GuildID == nil || *futureReminder.GuildID != guildID {
		t.Fatalf("unexpected future GuildID: %#v", futureReminder.GuildID)
	}
	if futureReminder.ChannelID == nil || *futureReminder.ChannelID != channelID {
		t.Fatalf("unexpected future ChannelID: %#v", futureReminder.ChannelID)
	}

	deleted, err := storeDB.Reminders().DeleteReminder(ctx, 42, "future")
	mustNoErr(t, err, "DeleteReminder")
	if !deleted {
		t.Fatalf("expected future reminder to be deleted")
	}

	deleted, err = storeDB.Reminders().DeleteReminder(ctx, 42, "future")
	mustNoErr(t, err, "DeleteReminder(second)")
	if deleted {
		t.Fatalf("expected second delete to be false")
	}
}

func TestPostgresClaimDueReminders_ReclaimsExpiredLease(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storeDB := newReminderTestStore(t, ctx)
	now := time.Unix(1700000000, 0).UTC()

	mustNoErr(t, storeDB.Reminders().CreateReminder(ctx, automationstore.Reminder{
		ReminderIdentity: automationstore.ReminderIdentity{
			ID:     "due",
			UserID: 7,
		},
		ReminderSchedule: automationstore.ReminderSchedule{
			Schedule: "0 * * * *",
			Kind:     "breathe",
		},
		ReminderDeliveryTarget: automationstore.ReminderDeliveryTarget{
			Delivery: automationstore.ReminderDeliveryDM,
		},
		ReminderState: automationstore.ReminderState{
			Enabled:   true,
			NextRunAt: now,
		},
	}), "CreateReminder")

	claimed, err := storeDB.Reminders().ClaimDueReminders(ctx, now, "lease-a", 30*time.Second, 1)
	mustNoErr(t, err, "ClaimDueReminders(first)")
	if len(claimed) != 1 {
		t.Fatalf("unexpected first claim count: %d", len(claimed))
	}

	reclaimed, err := storeDB.Reminders().ClaimDueReminders(ctx, now.Add(time.Minute), "lease-b", 30*time.Second, 1)
	mustNoErr(t, err, "ClaimDueReminders(reclaim)")
	if len(reclaimed) != 1 || reclaimed[0].ID != "due" {
		t.Fatalf("unexpected reclaimed reminders: %#v", reclaimed)
	}
}

func newReminderTestStore(t *testing.T, ctx context.Context) *postgresstore.Store {
	t.Helper()

	db := pgtest.OpenMigratedDB(t)
	storeDB, err := postgresstore.New(db)
	mustNoErr(t, err, "postgresstore.New")
	t.Cleanup(func() { _ = storeDB.Close() })
	return storeDB
}

func mustNoErr(t *testing.T, err error, message string) {
	t.Helper()
	if err != nil {
		t.Fatalf("%s: %v", message, err)
	}
}
