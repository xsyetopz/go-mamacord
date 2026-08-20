package storage

import (
	"context"
	"time"
)

type ReminderDelivery string

const (
	ReminderDeliveryDM      ReminderDelivery = "dm"
	ReminderDeliveryChannel ReminderDelivery = "channel"
)

type Reminder struct {
	ReminderIdentity
	ReminderSchedule
	ReminderDeliveryTarget
	ReminderState
	ReminderTimestamps
}

type ReminderIdentity struct {
	ID     string
	UserID uint64
}

type ReminderSchedule struct {
	Schedule string
	Kind     string
	Note     string
}

type ReminderDeliveryTarget struct {
	Delivery  ReminderDelivery
	GuildID   *uint64
	ChannelID *uint64
}

type ReminderState struct {
	Enabled      bool
	NextRunAt    time.Time
	LastRunAt    *time.Time
	FailureCount int
}

type ReminderTimestamps struct {
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ReminderStore interface {
	CreateReminder(ctx context.Context, reminder Reminder) error
	ListReminders(ctx context.Context, userID uint64, limit int) ([]Reminder, error)
	DeleteReminder(ctx context.Context, userID uint64, reminderID string) (bool, error)
	ClaimDueReminders(ctx context.Context, now time.Time, leaseID string, leaseDuration time.Duration, limit int) ([]Reminder, error)
	FinishReminderRun(ctx context.Context, reminderID, leaseID string, lastRunAt, nextRunAt time.Time, failureCount int, enabled bool) error
}

type CheckIn struct {
	ID        string
	UserID    uint64
	Mood      int
	CreatedAt time.Time
}

type CheckInStore interface {
	CreateCheckIn(ctx context.Context, checkIn CheckIn) error
	ListCheckIns(ctx context.Context, userID uint64, limit int) ([]CheckIn, error)
}
