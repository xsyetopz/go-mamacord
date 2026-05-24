package pluginbridge

import (
	"github.com/disgoorg/disgo/events"

	runtimepluginbridge "github.com/xsyetopz/go-mamacord/internal/runtime/plugins/bridge"
)

type SlashInteraction struct {
	event    *events.ApplicationCommandInteractionCreate
	deferred bool
}

var _ runtimepluginbridge.Interaction = (*SlashInteraction)(nil)

func NewSlashInteraction(event *events.ApplicationCommandInteractionCreate) *SlashInteraction {
	return &SlashInteraction{event: event}
}

func (i *SlashInteraction) Defer(ephemeral bool) error {
	if i.event == nil {
		return nil
	}
	if err := i.event.DeferCreateMessage(ephemeral); err != nil {
		return err
	}
	i.deferred = true
	return nil
}

func (i *SlashInteraction) Deferred() bool {
	return i != nil && i.deferred
}
