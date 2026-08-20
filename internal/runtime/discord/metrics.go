package discordruntime

func (b *Bot) incInteraction() {
	if b == nil || b.metrics == nil {
		return
	}
	b.metrics.IncInteractions()
}

func (b *Bot) incInteractionFailure() {
	if b == nil || b.metrics == nil {
		return
	}
	b.metrics.IncInteractionFailures()
}

func (b *Bot) incPluginFailure() {
	if b == nil || b.metrics == nil {
		return
	}
	b.metrics.IncPluginFailures()
}

func (b *Bot) incAutomationFailure() {
	if b == nil || b.metrics == nil {
		return
	}
	b.metrics.IncAutomationFailures()
}

func (b *Bot) incReminderFailure() {
	if b == nil || b.metrics == nil {
		return
	}
	b.metrics.IncReminderFailures()
}
