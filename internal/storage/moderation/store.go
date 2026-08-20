package moderation

import (
	"context"
	"time"
)

type TargetType string

const (
	TargetTypeUser  TargetType = "user"
	TargetTypeGuild TargetType = "guild"
)

type Restriction struct {
	TargetType TargetType
	TargetID   uint64
	Reason     string
	CreatedBy  uint64
	CreatedAt  time.Time
}

type Warning struct {
	ID          string
	GuildID     uint64
	UserID      uint64
	ModeratorID uint64
	Reason      string
	CreatedAt   time.Time
}

type AuditEntry struct {
	GuildID    *uint64
	ActorID    *uint64
	Action     string
	TargetType *TargetType
	TargetID   *uint64
	CreatedAt  time.Time
	MetaJSON   string
}

type RestrictionStore interface {
	GetRestriction(ctx context.Context, targetType TargetType, targetID uint64) (Restriction, bool, error)
	PutRestriction(ctx context.Context, restriction Restriction) error
	DeleteRestriction(ctx context.Context, targetType TargetType, targetID uint64) error
}

type WarningStore interface {
	CountWarnings(ctx context.Context, guildID, userID uint64) (int, error)
	ListWarnings(ctx context.Context, guildID, userID uint64, limit int) ([]Warning, error)
	CreateWarning(ctx context.Context, warning Warning) error
	DeleteWarning(ctx context.Context, id string) error
}

type AuditStore interface {
	Append(ctx context.Context, entry AuditEntry) error
}
