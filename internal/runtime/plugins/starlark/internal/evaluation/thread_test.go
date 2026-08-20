package evaluation

import (
	"context"
	"errors"
	"testing"
	"time"
	"unicode/utf8"
)

func TestThreadExecutionClassifiesLimits(t *testing.T) {
	t.Parallel()

	step := New(context.Background(), "step", 10, time.Second, 0, 0, nil)
	step.thread.OnMaxSteps(step.thread)
	if err := step.Finish("invoke", "//:plugin.star", errors.New("canceled")); !IsErrorKind(err, ErrorStepLimit) {
		t.Fatalf("step classification: %v", err)
	}

	deadline := New(context.Background(), "deadline", 100, time.Millisecond, 0, 0, nil)
	<-deadline.context.Done()
	if err := deadline.Finish("invoke", "//:plugin.star", errors.New("canceled")); !IsErrorKind(err, ErrorDeadline) {
		t.Fatalf("deadline classification: %v", err)
	}

	parent, cancel := context.WithCancel(context.Background())
	cancel()
	caller := New(parent, "caller", 100, time.Second, 0, 0, nil)
	if err := caller.Finish("invoke", "//:plugin.star", errors.New("canceled")); !IsErrorKind(err, ErrorCanceled) {
		t.Fatalf("caller classification: %v", err)
	}
}

func TestBoundedPrint(t *testing.T) {
	t.Parallel()
	var messages []string
	print := boundedPrint(2, 5, func(message string) { messages = append(messages, message) })
	print(nil, "  héllo  ")
	print(nil, "two")
	print(nil, "ignored")
	if len(messages) != 2 || messages[0] != "héll" || messages[1] != "two" {
		t.Fatalf("messages: %#v", messages)
	}
	if !utf8.ValidString(messages[0]) {
		t.Fatal("truncated print is invalid UTF-8")
	}
}
