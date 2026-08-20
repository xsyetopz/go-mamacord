package evaluation

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	starlarkgo "go.starlark.net/starlark"
)

const stepLimitReason = "mamacord execution step limit exceeded"

var errExecutionDeadline = errors.New("mamacord execution deadline exceeded")

type Execution struct {
	thread       *starlarkgo.Thread
	context      context.Context
	cancel       context.CancelFunc
	stopWatcher  func() bool
	stepLimitHit atomic.Bool
}

func (execution *Execution) Thread() *starlarkgo.Thread { return execution.thread }
func (execution *Execution) Context() context.Context   { return execution.context }

func New(
	parent context.Context,
	name string,
	steps uint64,
	timeoutDuration time.Duration,
	printLimit int,
	printBytes int,
	print func(string),
) *Execution {
	if parent == nil {
		parent = context.Background()
	}
	runContext, cancel := context.WithTimeoutCause(parent, timeoutDuration, errExecutionDeadline)
	execution := &Execution{context: runContext, cancel: cancel}
	thread := &starlarkgo.Thread{Name: name}
	thread.SetMaxExecutionSteps(steps)
	thread.OnMaxSteps = func(thread *starlarkgo.Thread) {
		execution.stepLimitHit.Store(true)
		thread.Cancel(stepLimitReason)
	}
	thread.Print = boundedPrint(printLimit, printBytes, print)
	execution.thread = thread
	execution.stopWatcher = context.AfterFunc(runContext, func() {
		reason := "mamacord execution canceled"
		if errors.Is(context.Cause(runContext), errExecutionDeadline) {
			reason = errExecutionDeadline.Error()
		}
		thread.Cancel(reason)
	})
	return execution
}

func (execution *Execution) Close() {
	if execution == nil {
		return
	}
	if execution.stopWatcher != nil {
		execution.stopWatcher()
	}
	if execution.cancel != nil {
		execution.cancel()
	}
}

func (execution *Execution) Finish(phase, source string, err error) error {
	if execution == nil {
		if err == nil {
			return nil
		}
		return NewRuntimeError(ErrorInvocation, phase, source, SanitizeError(err))
	}
	classified := execution.Classify(phase, source, err)
	execution.Close()
	return classified
}

func (execution *Execution) Classify(phase, source string, err error) error {
	if err == nil {
		return nil
	}
	if execution.stepLimitHit.Load() {
		return NewRuntimeError(ErrorStepLimit, phase, source, errors.New("execution step limit exceeded"))
	}
	cause := context.Cause(execution.context)
	if errors.Is(cause, errExecutionDeadline) {
		return NewRuntimeError(ErrorDeadline, phase, source, context.DeadlineExceeded)
	}
	if cause != nil {
		return NewRuntimeError(ErrorCanceled, phase, source, cause)
	}
	return NewRuntimeError(errorKindForPhase(phase), phase, source, SanitizeError(err))
}

func errorKindForPhase(phase string) ErrorKind {
	switch phase {
	case "initialize":
		return ErrorInitialize
	case "setup":
		return ErrorSetup
	default:
		return ErrorInvocation
	}
}

func boundedPrint(maxMessages, maxBytes int, sink func(string)) func(*starlarkgo.Thread, string) {
	count := 0
	return func(_ *starlarkgo.Thread, message string) {
		if sink == nil || maxMessages == 0 || count >= maxMessages {
			return
		}
		count++
		sink(truncateUTF8(strings.TrimSpace(message), maxBytes))
	}
}

func truncateUTF8(value string, maxBytes int) string {
	if maxBytes <= 0 {
		return ""
	}
	if len(value) <= maxBytes {
		return value
	}
	value = value[:maxBytes]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value
}
