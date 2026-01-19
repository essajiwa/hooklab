package main

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"
)

func TestEventsStreamHandlerNoFlusher(t *testing.T) {
	app := &App{}
	req := httptest.NewRequest("GET", "/api/stream", nil)
	writer := &noFlushWriter{}
	app.eventsStreamHandler(writer, req)
	if writer.status != 500 {
		t.Errorf("expected status 500 for no flusher, got %d", writer.status)
	}
}

func TestEventsStreamLoopHeartbeat(t *testing.T) {
	app := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/api/stream", nil).WithContext(ctx)
	writer := &sseWriter{}
	ticks := make(chan time.Time, 1)

	done := make(chan struct{})
	go func() {
		app.eventsStreamLoop(writer, req, writer, ticks)
		close(done)
	}()

	// Wait for subscriber to be added
	time.Sleep(10 * time.Millisecond)

	// Send a heartbeat tick
	ticks <- time.Now()

	// Wait a bit for heartbeat to be written
	time.Sleep(10 * time.Millisecond)

	// Cancel context to exit loop
	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("eventsStreamLoop did not exit")
	}

	if !contains(writer.buffer.String(), ": ping") {
		t.Errorf("expected ping in output, got: %s", writer.buffer.String())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestEventsStreamLoopEvent(t *testing.T) {
	app := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/api/stream", nil).WithContext(ctx)
	writer := &sseWriter{}
	ticks := make(chan time.Time)

	done := make(chan struct{})
	go func() {
		app.eventsStreamLoop(writer, req, writer, ticks)
		close(done)
	}()

	// Wait for subscriber to be added
	time.Sleep(10 * time.Millisecond)

	// Broadcast an event
	app.broadcastEvent(Event{ID: 42, Key: "test"})

	// Wait a bit for event to be written
	time.Sleep(10 * time.Millisecond)

	// Cancel context to exit loop
	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("eventsStreamLoop did not exit")
	}
}

func TestEventsStreamLoopChannelClosed(t *testing.T) {
	app := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/api/stream", nil).WithContext(ctx)
	writer := &sseWriter{}
	ticks := make(chan time.Time)

	done := make(chan struct{})
	go func() {
		app.eventsStreamLoop(writer, req, writer, ticks)
		close(done)
	}()

	// Wait for subscriber
	time.Sleep(10 * time.Millisecond)

	// Close subscribers which closes the channel
	app.closeSubscribers()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("eventsStreamLoop did not exit when channel closed")
	}
}

func TestEventsStreamLoopContextDone(t *testing.T) {
	app := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest("GET", "/api/stream", nil).WithContext(ctx)
	writer := &sseWriter{}
	ticks := make(chan time.Time)

	done := make(chan struct{})
	go func() {
		app.eventsStreamLoop(writer, req, writer, ticks)
		close(done)
	}()

	// Wait for subscriber
	time.Sleep(10 * time.Millisecond)

	// Cancel context
	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("eventsStreamLoop did not exit when context cancelled")
	}
}

func TestBroadcastEventWithFullChannel(t *testing.T) {
	app := &App{subscribers: make(map[chan Event]struct{})}
	// Create a channel with buffer 1 and fill it
	ch := make(chan Event, 1)
	ch <- Event{ID: 0}
	app.subscribers[ch] = struct{}{}

	// Broadcast should not block even with full channel
	app.broadcastEvent(Event{ID: 1})
	// Test passes if it doesn't deadlock
}
