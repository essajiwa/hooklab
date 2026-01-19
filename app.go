package main

// This file contains the core application state and business logic for Hooklab.
// It manages webhook events, response configurations, rules, and SSE subscribers.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/expr-lang/expr"
)

// App holds the application state including webhook events, response configurations,
// conditional rules, and SSE subscribers. All fields are protected by a mutex for
// concurrent access safety.
type App struct {
	responses   map[string]ResponseConfig
	rules       map[string][]Rule // rules per webhook key
	mu          sync.Mutex
	events      []Event
	lastID      int
	ruleLastID  int
	subscribers map[chan Event]struct{}
}

// ResponseConfig defines the response to return for a webhook request.
// Response can be any JSON-serializable value, and StatusCode is the HTTP status.
type ResponseConfig struct {
	Response    interface{} // JSON response body
	ResponseRaw string      // Raw JSON string of the response
	StatusCode  int         // HTTP status code (e.g., 200, 404)
}

// Rule represents a conditional response rule that can override the default response
// based on request content. Rules are evaluated using the expr expression language.
type Rule struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Condition  string      `json:"condition"` // expr expression, e.g., "body.amount > 100"
	Response   interface{} `json:"response"`
	StatusCode int         `json:"statusCode"`
	Priority   int         `json:"priority"` // Lower = higher priority
	Enabled    bool        `json:"enabled"`
}

// Event represents a captured webhook request with all its metadata.
// Events are stored in memory and broadcast to SSE subscribers in real-time.
type Event struct {
	ID        int                 `json:"id"`        // Unique event identifier
	Timestamp time.Time           `json:"timestamp"` // When the event was received
	Method    string              `json:"method"`    // HTTP method (GET, POST, etc.)
	Path      string              `json:"path"`      // Request path
	Key       string              `json:"key"`       // Webhook key from path
	Headers   map[string][]string `json:"headers"`   // Request headers
	Body      string              `json:"body"`      // Request body
}

// EventsResponse is the JSON response structure for the /api/events endpoint.
type EventsResponse struct {
	Events []Event `json:"events"`
}

// storeEvent captures an incoming webhook request and stores it in memory.
// It maintains a maximum of 50 events, discarding the oldest when the limit is reached.
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

// getResponseConfig returns the response configuration for the given webhook key.
// If no configuration exists for the key, it falls back to "default", then to a
// hardcoded fallback response.
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

// setResponseConfig stores a response configuration for the given webhook key.
// An empty key defaults to "default".
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

// addSubscriber creates a new SSE subscriber channel and registers it.
// Events will be broadcast to this channel until removeSubscriber is called.
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

// removeSubscriber unregisters an SSE subscriber and closes its channel.
func (a *App) removeSubscriber(ch chan Event) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.subscribers[ch]; !ok {
		return
	}
	delete(a.subscribers, ch)
	close(ch)
}

// broadcastEvent sends an event to all registered SSE subscribers.
// Non-blocking: if a subscriber's channel is full, the event is dropped for that subscriber.
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

// closeSubscribers closes all SSE subscriber channels during shutdown.
func (a *App) closeSubscribers() {
	a.mu.Lock()
	defer a.mu.Unlock()

	for ch := range a.subscribers {
		close(ch)
	}
	a.subscribers = make(map[chan Event]struct{})
}

// getKeys returns a sorted list of all known webhook keys.
// Keys are collected from events, responses, and rules. The "default" key is always included.
func (a *App) getKeys() []string {
	a.mu.Lock()
	defer a.mu.Unlock()

	keySet := make(map[string]struct{})

	// Add keys from events
	for _, event := range a.events {
		keySet[event.Key] = struct{}{}
	}

	// Add keys from responses
	for key := range a.responses {
		keySet[key] = struct{}{}
	}

	// Add keys from rules
	for key := range a.rules {
		keySet[key] = struct{}{}
	}

	// Always include "default"
	keySet["default"] = struct{}{}

	keys := make([]string, 0, len(keySet))
	for key := range keySet {
		keys = append(keys, key)
	}

	sort.Strings(keys)
	return keys
}

// getRules returns all rules for the given webhook key, sorted by priority (ascending).
// Lower priority values are evaluated first.
func (a *App) getRules(key string) []Rule {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.rules == nil {
		return []Rule{}
	}

	rules := a.rules[key]
	if rules == nil {
		return []Rule{}
	}

	// Return sorted by priority
	sorted := make([]Rule, len(rules))
	copy(sorted, rules)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Priority < sorted[j].Priority
	})
	return sorted
}

// setRules replaces all rules for the given webhook key.
func (a *App) setRules(key string, rules []Rule) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.rules == nil {
		a.rules = make(map[string][]Rule)
	}
	a.rules[key] = rules
}

// addRule adds a new rule for the given webhook key and assigns it a unique ID.
func (a *App) addRule(key string, rule Rule) Rule {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.rules == nil {
		a.rules = make(map[string][]Rule)
	}

	a.ruleLastID++
	rule.ID = fmt.Sprintf("rule_%d", a.ruleLastID)

	a.rules[key] = append(a.rules[key], rule)
	return rule
}

// updateRule updates an existing rule by ID. Returns true if the rule was found and updated.
func (a *App) updateRule(key string, ruleID string, updated Rule) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.rules == nil {
		return false
	}

	rules := a.rules[key]
	for i, r := range rules {
		if r.ID == ruleID {
			updated.ID = ruleID
			rules[i] = updated
			a.rules[key] = rules
			return true
		}
	}
	return false
}

// deleteRule removes a rule by ID. Returns true if the rule was found and deleted.
func (a *App) deleteRule(key string, ruleID string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.rules == nil {
		return false
	}

	rules := a.rules[key]
	for i, r := range rules {
		if r.ID == ruleID {
			a.rules[key] = append(rules[:i], rules[i+1:]...)
			return true
		}
	}
	return false
}

// evaluateRules checks all enabled rules for a key and returns the first matching response.
// Rules are evaluated in priority order. The expression environment includes:
//   - body: parsed JSON body (or raw string if not valid JSON)
//   - method: HTTP method string
//   - headers: map of header names to values
//
// Returns nil if no rule matches.
func (a *App) evaluateRules(key string, body string, method string, headers map[string][]string) (*ResponseConfig, error) {
	rules := a.getRules(key)

	// Parse body as JSON for expression evaluation
	var bodyData interface{}
	if body != "" {
		if err := json.Unmarshal([]byte(body), &bodyData); err != nil {
			// If body is not valid JSON, use it as a string
			bodyData = body
		}
	}

	// Build environment for expression evaluation
	env := map[string]interface{}{
		"body":    bodyData,
		"method":  method,
		"headers": headers,
	}

	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}

		// Compile and evaluate the expression
		program, err := expr.Compile(rule.Condition, expr.Env(env), expr.AsBool())
		if err != nil {
			continue // Skip invalid expressions
		}

		result, err := expr.Run(program, env)
		if err != nil {
			continue
		}

		if matched, ok := result.(bool); ok && matched {
			return &ResponseConfig{
				Response:   rule.Response,
				StatusCode: rule.StatusCode,
			}, nil
		}
	}

	return nil, nil // No rule matched
}
