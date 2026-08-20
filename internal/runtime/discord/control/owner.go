package control

import (
	"context"
	"log/slog"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
)

const (
	ownerSourceDiscord        = "discord"
	ownerSourceConfigFallback = "config_fallback"
	ownerSourceUnresolved     = "unresolved"
)

type OwnerStatus struct {
	Configured      bool
	Resolved        bool
	Source          string
	EffectiveUserID *uint64
}

type Owner struct {
	configuredUserID *uint64
	effectiveUserID  *uint64
	source           string
}

func NewOwner(configuredUserID *uint64) Owner {
	owner := Owner{configuredUserID: cloneOptionalUint64(configuredUserID), source: ownerSourceUnresolved}
	if owner.configuredUserID != nil {
		owner.effectiveUserID = cloneOptionalUint64(owner.configuredUserID)
		owner.source = ownerSourceConfigFallback
	}
	return owner
}

func (owner *Owner) Resolve(ctx context.Context, client *bot.Client, logger *slog.Logger) {
	if owner == nil {
		return
	}
	if ownerID, ok := lookupOwnerFromDiscord(ctx, client, logger, owner.configuredUserID); ok {
		owner.effectiveUserID = &ownerID
		owner.source = ownerSourceDiscord
		return
	}
	if owner.configuredUserID != nil {
		owner.effectiveUserID = cloneOptionalUint64(owner.configuredUserID)
		owner.source = ownerSourceConfigFallback
		return
	}
	owner.effectiveUserID = nil
	owner.source = ownerSourceUnresolved
}

func (owner Owner) Is(userID uint64) bool {
	return owner.effectiveUserID != nil && *owner.effectiveUserID == userID
}

func (owner Owner) Status() OwnerStatus {
	return OwnerStatus{
		Configured: owner.configuredUserID != nil, Resolved: owner.effectiveUserID != nil,
		Source: owner.source, EffectiveUserID: cloneOptionalUint64(owner.effectiveUserID),
	}
}

func lookupOwnerFromDiscord(ctx context.Context, client *bot.Client, logger *slog.Logger, fallback *uint64) (uint64, bool) {
	if client == nil || client.Rest == nil {
		return 0, false
	}
	application, err := client.Rest.GetCurrentApplication(rest.WithCtx(ctx))
	if err != nil {
		logOwnerResolutionError(logger, "discord application lookup failed", err, fallback)
		return 0, false
	}
	ownerID, ok := resolveOwnerFromApplication(application)
	if !ok {
		logOwnerResolutionError(logger, "discord application owner lookup returned no owner", nil, fallback)
		return 0, false
	}
	return ownerID, true
}

func resolveOwnerFromApplication(application *discord.Application) (uint64, bool) {
	if application == nil {
		return 0, false
	}
	if application.Team != nil && application.Team.OwnerID != 0 {
		return uint64(application.Team.OwnerID), true
	}
	if application.Owner != nil && application.Owner.ID != 0 {
		return uint64(application.Owner.ID), true
	}
	return 0, false
}

func logOwnerResolutionError(logger *slog.Logger, message string, err error, fallback *uint64) {
	if logger == nil {
		return
	}
	attrs := []any{slog.String("source", ownerSourceDiscord)}
	if err != nil {
		attrs = append(attrs, slog.String("err", err.Error()))
	}
	if fallback != nil {
		attrs = append(attrs, slog.Uint64("fallback_owner_user_id", *fallback))
	}
	logger.Warn(message, attrs...)
}

func cloneOptionalUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}
