package discordruntime

import (
	"io"
	"log/slog"
	"runtime"
	"testing"
	"time"
)

func TestInteractionDispatchBoundsConcurrencyAndRecoversPanics(t *testing.T) {
	bot := &Bot{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), interactionSlots: make(chan struct{}, 1), interactionRejectSlots: make(chan struct{}, 1), interactionRejectQueue: make(chan interactionRejection, 4)}
	release := make(chan struct{})
	entered := make(chan struct{})
	if !bot.launchInteraction("first", func() { close(entered); <-release }, nil) {
		t.Fatal("first launch rejected")
	}
	<-entered
	if bot.launchInteraction("overflow", func() {}, nil) {
		t.Fatal("overflow launch accepted")
	}
	close(release)
	deadline := time.Now().Add(time.Second)
	for len(bot.interactionSlots) != 0 && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if len(bot.interactionSlots) != 0 {
		t.Fatal("slot was not released")
	}
	panicked := make(chan struct{})
	fallback := make(chan struct{})
	if !bot.launchInteraction("panic", func() { defer close(panicked); panic("boom") }, func() { close(fallback) }) {
		t.Fatal("panic launch rejected")
	}
	<-panicked
	select {
	case <-fallback:
	case <-time.After(time.Second):
		t.Fatal("panic fallback did not run")
	}
	for len(bot.interactionSlots) != 0 && time.Now().Before(deadline) {
		runtime.Gosched()
	}
	if len(bot.interactionSlots) != 0 {
		t.Fatal("panic did not release slot")
	}
}

func TestInteractionDispatchAcknowledgesOverloadThroughBoundedPath(t *testing.T) {
	bot := &Bot{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), interactionSlots: make(chan struct{}, 1), interactionRejectSlots: make(chan struct{}, 1), interactionRejectQueue: make(chan interactionRejection, 4)}
	release := make(chan struct{})
	entered := make(chan struct{})
	if !bot.launchInteraction("active", func() { close(entered); <-release }, nil) {
		t.Fatal("active rejected")
	}
	<-entered
	rejected := make(chan struct{})
	if bot.launchInteraction("overflow", func() {}, func() { close(rejected) }) {
		t.Fatal("overflow launched")
	}
	select {
	case <-rejected:
	case <-time.After(time.Second):
		t.Fatal("overload response did not run")
	}
	close(release)
}

func TestInteractionDispatchQueuesOverloadAcknowledgements(t *testing.T) {
	bot := &Bot{
		logger:                 slog.New(slog.NewTextHandler(io.Discard, nil)),
		interactionSlots:       make(chan struct{}, 1),
		interactionRejectSlots: make(chan struct{}, 1),
		interactionRejectQueue: make(chan interactionRejection, 4),
	}
	activeRelease := make(chan struct{})
	activeEntered := make(chan struct{})
	if !bot.launchInteraction("active", func() { close(activeEntered); <-activeRelease }, nil) {
		t.Fatal("active rejected")
	}
	<-activeEntered

	firstRelease := make(chan struct{})
	firstEntered := make(chan struct{})
	if bot.launchInteraction("overflow-1", func() {}, func() { close(firstEntered); <-firstRelease }) {
		t.Fatal("first overflow launched")
	}
	<-firstEntered

	acknowledged := make(chan string, 2)
	for _, kind := range []string{"overflow-2", "overflow-3"} {
		kind := kind
		if bot.launchInteraction(kind, func() {}, func() { acknowledged <- kind }) {
			t.Fatalf("%s launched", kind)
		}
	}
	close(firstRelease)
	for range 2 {
		select {
		case <-acknowledged:
		case <-time.After(time.Second):
			t.Fatal("queued overload acknowledgement did not run")
		}
	}
	close(activeRelease)
}
