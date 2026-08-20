package interactions

import (
	"log/slog"
	"runtime/debug"
)

const maximumConcurrentInteractions = 128
const maximumConcurrentInteractionRejections = 8
const maximumPendingInteractionRejections = 128

type Dispatcher struct {
	logger      *slog.Logger
	onFailure   func()
	slots       chan struct{}
	rejectSlots chan struct{}
	rejectQueue chan interactionRejection
}

func NewDispatcher(logger *slog.Logger, onFailure func()) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}
	return &Dispatcher{
		logger: logger, onFailure: onFailure,
		slots:       make(chan struct{}, maximumConcurrentInteractions),
		rejectSlots: make(chan struct{}, maximumConcurrentInteractionRejections),
		rejectQueue: make(chan interactionRejection, maximumPendingInteractionRejections),
	}
}

type interactionRejection struct {
	kind string
	run  func()
}

func (dispatcher *Dispatcher) Launch(kind string, run func(), reject func()) bool {
	if dispatcher == nil || run == nil {
		return false
	}
	slots := dispatcher.slots
	if slots == nil {
		return false
	}
	select {
	case slots <- struct{}{}:
		go func() {
			defer func() {
				<-slots
				if recovered := recover(); recovered != nil {
					logger := dispatcher.interactionLogger()
					logger.Error("interaction handler panicked", slog.String("kind", kind), slog.Any("panic", recovered), slog.String("stack", string(debug.Stack())))
					if dispatcher.onFailure != nil {
						dispatcher.onFailure()
					}
					dispatcher.launchRejection(kind, reject)
				}
			}()
			run()
		}()
		return true
	default:
		logger := dispatcher.interactionLogger()
		logger.Warn("interaction concurrency limit reached", slog.String("kind", kind), slog.Int("limit", cap(slots)))
		if dispatcher.onFailure != nil {
			dispatcher.onFailure()
		}
		dispatcher.launchRejection(kind, reject)
		return false
	}
}

func (dispatcher *Dispatcher) launchRejection(kind string, reject func()) {
	if reject == nil || dispatcher == nil || dispatcher.rejectSlots == nil {
		return
	}
	job := interactionRejection{kind: kind, run: reject}
	if dispatcher.startRejection(job) {
		return
	}
	queue := dispatcher.rejectQueue
	if queue == nil {
		dispatcher.interactionLogger().Warn("interaction overload response queue unavailable", slog.String("kind", kind))
		return
	}
	select {
	case queue <- job:
		dispatcher.startQueuedRejection()
	default:
		dispatcher.interactionLogger().Warn("interaction overload response queue reached its bounded capacity", slog.String("kind", kind), slog.Int("limit", cap(queue)))
	}
}

func (dispatcher *Dispatcher) startRejection(job interactionRejection) bool {
	select {
	case dispatcher.rejectSlots <- struct{}{}:
		go dispatcher.runRejections(&job)
		return true
	default:
		return false
	}
}

func (dispatcher *Dispatcher) startQueuedRejection() {
	select {
	case dispatcher.rejectSlots <- struct{}{}:
		go dispatcher.runRejections(nil)
	default:
	}
}

func (dispatcher *Dispatcher) runRejections(first *interactionRejection) {
	defer func() {
		<-dispatcher.rejectSlots
		if len(dispatcher.rejectQueue) != 0 {
			dispatcher.startQueuedRejection()
		}
	}()
	if first != nil {
		dispatcher.runRejection(*first)
	}
	for dispatcher.rejectQueue != nil {
		select {
		case job := <-dispatcher.rejectQueue:
			dispatcher.runRejection(job)
		default:
			return
		}
	}
}

func (dispatcher *Dispatcher) runRejection(job interactionRejection) {
	defer func() {
		if recovered := recover(); recovered != nil {
			dispatcher.interactionLogger().Error("interaction overload response panicked", slog.String("kind", job.kind), slog.Any("panic", recovered), slog.String("stack", string(debug.Stack())))
		}
	}()
	job.run()
}

func (dispatcher *Dispatcher) interactionLogger() *slog.Logger {
	if dispatcher != nil && dispatcher.logger != nil {
		return dispatcher.logger
	}
	return slog.Default()
}
