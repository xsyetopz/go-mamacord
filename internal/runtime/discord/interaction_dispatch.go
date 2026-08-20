package discordruntime

import (
	"log/slog"
	"runtime/debug"
)

const maximumConcurrentInteractions = 128
const maximumConcurrentInteractionRejections = 8
const maximumPendingInteractionRejections = 128

type interactionRejection struct {
	kind string
	run  func()
}

func (b *Bot) launchInteraction(kind string, run func(), reject func()) bool {
	if b == nil || run == nil {
		return false
	}
	slots := b.interactionSlots
	if slots == nil {
		return false
	}
	select {
	case slots <- struct{}{}:
		go func() {
			defer func() {
				<-slots
				if recovered := recover(); recovered != nil {
					logger := b.interactionLogger()
					logger.Error("interaction handler panicked", slog.String("kind", kind), slog.Any("panic", recovered), slog.String("stack", string(debug.Stack())))
					b.incInteractionFailure()
					b.launchInteractionRejection(kind, reject)
				}
			}()
			run()
		}()
		return true
	default:
		logger := b.interactionLogger()
		logger.Warn("interaction concurrency limit reached", slog.String("kind", kind), slog.Int("limit", cap(slots)))
		b.incInteractionFailure()
		b.launchInteractionRejection(kind, reject)
		return false
	}
}

func (b *Bot) launchInteractionRejection(kind string, reject func()) {
	if reject == nil || b == nil || b.interactionRejectSlots == nil {
		return
	}
	job := interactionRejection{kind: kind, run: reject}
	if b.startInteractionRejection(job) {
		return
	}
	queue := b.interactionRejectQueue
	if queue == nil {
		b.interactionLogger().Warn("interaction overload response queue unavailable", slog.String("kind", kind))
		return
	}
	select {
	case queue <- job:
		b.startQueuedInteractionRejection()
	default:
		b.interactionLogger().Warn("interaction overload response queue reached its bounded capacity", slog.String("kind", kind), slog.Int("limit", cap(queue)))
	}
}

func (b *Bot) startInteractionRejection(job interactionRejection) bool {
	select {
	case b.interactionRejectSlots <- struct{}{}:
		go b.runInteractionRejections(&job)
		return true
	default:
		return false
	}
}

func (b *Bot) startQueuedInteractionRejection() {
	select {
	case b.interactionRejectSlots <- struct{}{}:
		go b.runInteractionRejections(nil)
	default:
	}
}

func (b *Bot) runInteractionRejections(first *interactionRejection) {
	defer func() {
		<-b.interactionRejectSlots
		if len(b.interactionRejectQueue) != 0 {
			b.startQueuedInteractionRejection()
		}
	}()
	if first != nil {
		b.runInteractionRejection(*first)
	}
	for b.interactionRejectQueue != nil {
		select {
		case job := <-b.interactionRejectQueue:
			b.runInteractionRejection(job)
		default:
			return
		}
	}
}

func (b *Bot) runInteractionRejection(job interactionRejection) {
	defer func() {
		if recovered := recover(); recovered != nil {
			b.interactionLogger().Error("interaction overload response panicked", slog.String("kind", job.kind), slog.Any("panic", recovered), slog.String("stack", string(debug.Stack())))
		}
	}()
	job.run()
}

func (b *Bot) interactionLogger() *slog.Logger {
	if b != nil && b.logger != nil {
		return b.logger
	}
	return slog.Default()
}
