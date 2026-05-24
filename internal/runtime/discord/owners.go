package discordruntime

import (
	"context"
	"log/slog"

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

type ownerState struct {
	configuredUserID *uint64
	effectiveUserID  *uint64
	source           string
}

func newOwnerState(configuredUserID *uint64) ownerState {
	state := ownerState{
		configuredUserID: cloneOptionalUint64(configuredUserID),
		source:           ownerSourceUnresolved,
	}
	if state.configuredUserID != nil {
		state.effectiveUserID = cloneOptionalUint64(state.configuredUserID)
		state.source = ownerSourceConfigFallback
	}
	return state
}

func cloneOptionalUint64(value *uint64) *uint64 {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func (b *Bot) resolveOwner(ctx context.Context) {
	if b == nil {
		return
	}
	if ownerID, ok := b.lookupOwnerFromDiscord(ctx); ok {
		b.owner.effectiveUserID = &ownerID
		b.owner.source = ownerSourceDiscord
		return
	}
	if b.owner.configuredUserID != nil {
		b.owner.effectiveUserID = cloneOptionalUint64(b.owner.configuredUserID)
		b.owner.source = ownerSourceConfigFallback
		return
	}
	b.owner.effectiveUserID = nil
	b.owner.source = ownerSourceUnresolved
}

func (b *Bot) lookupOwnerFromDiscord(ctx context.Context) (uint64, bool) {
	if b == nil || b.client == nil || b.client.Rest == nil {
		return 0, false
	}
	application, err := b.client.Rest.GetCurrentApplication(rest.WithCtx(ctx))
	if err != nil {
		b.logOwnerResolutionError("discord application lookup failed", err)
		return 0, false
	}
	ownerID, ok := resolveOwnerFromApplication(application)
	if !ok {
		b.logOwnerResolutionError("discord application owner lookup returned no owner", nil)
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

func (b *Bot) logOwnerResolutionError(message string, err error) {
	if b == nil || b.logger == nil {
		return
	}
	attrs := []any{slog.String("source", ownerSourceDiscord)}
	if err != nil {
		attrs = append(attrs, slog.String("err", err.Error()))
	}
	if b.owner.configuredUserID != nil {
		attrs = append(attrs, slog.Uint64("fallback_owner_user_id", *b.owner.configuredUserID))
	}
	b.logger.Warn(message, attrs...)
}

func (b *Bot) isOwner(userID uint64) bool {
	if b == nil || b.owner.effectiveUserID == nil {
		return false
	}
	return *b.owner.effectiveUserID == userID
}

func (b *Bot) OwnerStatus() OwnerStatus {
	if b == nil {
		return OwnerStatus{Source: ownerSourceUnresolved}
	}
	return OwnerStatus{
		Configured:      b.owner.configuredUserID != nil,
		Resolved:        b.owner.effectiveUserID != nil,
		Source:          b.owner.source,
		EffectiveUserID: cloneOptionalUint64(b.owner.effectiveUserID),
	}
}
