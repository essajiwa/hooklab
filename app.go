package main

import (
	"net/http"
	"sync"
	"time"
)

// App holds the application configuration and dependencies.
type App struct {
	responses   map[string]ResponseConfig
	mu          sync.Mutex
	events      []Event
	lastID      int
	subscribers map[chan Event]struct{}
}

type ResponseConfig struct {
	Response    interface{}
	ResponseRaw string
	StatusCode  int
}

type Event struct {
	ID        int                 `json:"id"`
	Timestamp time.Time           `json:"timestamp"`
	Method    string              `json:"method"`
	Path      string              `json:"path"`
	Key       string              `json:"key"`
	Headers   map[string][]string `json:"headers"`
	Body      string              `json:"body"`
}

type EventsResponse struct {
	Events []Event `json:"events"`
}

func (a *App) storeEvent(r *http.Request, key, body string) Event {
	a.mu.Lock()
	defer a.mu.Unlock()

	a.lastID++
	event := Event{
		ID:        a.lastID,
		Timestamp: time.Now(),
		Method:    r.Method,
		Path:      r.URL.Path,
		Key:       key,
		Headers:   r.Header,
		Body:      body,
	}

	const maxEvents = 50
	a.events = append([]Event{event}, a.events...)
	if len(a.events) > maxEvents {
		a.events = a.events[:maxEvents]
	}

	return event
}

func (a *App) getResponseConfig(key string) ResponseConfig {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.responses == nil {
		a.responses = make(map[string]ResponseConfig)
	}

	if config, ok := a.responses[key]; ok {
		return config
	}

	// Return default config if key not found
	if defaultConfig, ok := a.responses["default"]; ok {
		return defaultConfig
	}

	// Fallback if no default exists
	return ResponseConfig{
		Response:   map[string]string{"result": "ok"},
		StatusCode: 200,
	}
}

func (a *App) setResponseConfig(key string, config ResponseConfig) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.responses == nil {
		a.responses = make(map[string]ResponseConfig)
	}
	if key == "" {
		key = "default"
	}
	a.responses[key] = config
}

func (a *App) addSubscriber() chan Event {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.subscribers == nil {
		a.subscribers = make(map[chan Event]struct{})
	}

	ch := make(chan Event, 1)
	a.subscribers[ch] = struct{}{}
	return ch
}

func (a *App) removeSubscriber(ch chan Event) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.subscribers[ch]; !ok {
		return
	}
	delete(a.subscribers, ch)
	close(ch)
}

func (a *App) broadcastEvent(event Event) {
	a.mu.Lock()
	defer a.mu.Unlock()

	for ch := range a.subscribers {
		select {
		case ch <- event:
		default:
		}
	}
}

func (a *App) closeSubscribers() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for ch := range a.subscribers {
		close(ch)
	}
	a.subscribers = make(map[chan Event]struct{})
}
