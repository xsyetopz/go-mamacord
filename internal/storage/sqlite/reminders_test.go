package sqlitestore_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	migrate "github.com/xsyetopz/go-mamacord/internal/migration"
	"github.com/xsyetopz/go-mamacord/internal/sqlite"
	store "github.com/xsyetopz/go-mamacord/internal/storage"
	sqlitestore "github.com/xsyetopz/go-mamacord/internal/storage/sqlite"
)

func TestReminderLifecycleAndLeaseFlow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storeDB := newReminderTestStore(t, ctx)

	now := time.Unix(1700000000, 0).UTC()
	channelID := uint64(777)
	guildID := uint64(888)

	mustNoErr(t, storeDB.Reminders().CreateReminder(ctx, store.Reminder{
		ID:        "due",
		UserID:    42,
		Schedule:  "0 * * * *",
		Kind:      "hydrate",
		Note:      "water",
		Delivery:  store.ReminderDeliveryDM,
		Enabled:   true,
		NextRunAt: now,
	}), "CreateReminder(due)")

	mustNoErr(t, storeDB.Reminders().CreateReminder(ctx, store.Reminder{
		ID:        "future",
		UserID:    42,
		Schedule:  "0 * * * *",
		Kind:      "stretch",
		Note:      "legs",
		Delivery:  store.ReminderDeliveryChannel,
		GuildID:   &guildID,
		ChannelID: &channelID,
		Enabled:   true,
		NextRunAt: now.Add(2 * time.Hour),
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

	byID := map[string]store.Reminder{}
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

func TestClaimDueReminders_ReclaimsExpiredLease(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	storeDB := newReminderTestStore(t, ctx)
	now := time.Unix(1700000000, 0).UTC()

	mustNoErr(t, storeDB.Reminders().CreateReminder(ctx, store.Reminder{
		ID:        "due",
		UserID:    7,
		Schedule:  "0 * * * *",
		Kind:      "breathe",
		Delivery:  store.ReminderDeliveryDM,
		Enabled:   true,
		NextRunAt: now,
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

func newReminderTestStore(t *testing.T, ctx context.Context) *sqlitestore.Store {
	t.Helper()

	dir := t.TempDir()
	db, err := sqlite.Open(ctx, sqlite.Options{Path: filepath.Join(dir, "test.sqlite")})
	mustNoErr(t, err, "sqlite.Open")

	migrationsDir := filepath.Clean(filepath.Join("..", "..", "..", "migrations", "sqlite"))
	runner, err := migrate.New(migrate.Options{Dir: migrationsDir, BackupDir: filepath.Join(dir, "migration_backups")})
	mustNoErr(t, err, "migrate.New")
	_, err = runner.Up(ctx, db)
	mustNoErr(t, err, "runner.Up")

	storeDB, err := sqlitestore.New(db)
	mustNoErr(t, err, "sqlitestore.New")
	t.Cleanup(func() { _ = storeDB.Close() })
	return storeDB
}
