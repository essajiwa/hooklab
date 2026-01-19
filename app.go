package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/expr-lang/expr"
)

// App holds the application configuration and dependencies.
type App struct {
	responses   map[string]ResponseConfig
	rules       map[string][]Rule // rules per webhook key
	mu          sync.Mutex
	events      []Event
	lastID      int
	ruleLastID  int
	subscribers map[chan Event]struct{}
}

type ResponseConfig struct {
	Response    interface{}
	ResponseRaw string
	StatusCode  int
}

// Rule represents a conditional response rule
type Rule struct {
	ID         string      `json:"id"`
	Name       string      `json:"name"`
	Condition  string      `json:"condition"` // expr expression, e.g., "body.amount > 100"
	Response   interface{} `json:"response"`
	StatusCode int         `json:"statusCode"`
	Priority   int         `json:"priority"` // Lower = higher priority
	Enabled    bool        `json:"enabled"`
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

// getKeys returns all known webhook keys from events, responses, and rules
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

// Rule management methods

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

func (a *App) setRules(key string, rules []Rule) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.rules == nil {
		a.rules = make(map[string][]Rule)
	}
	a.rules[key] = rules
}

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

// evaluateRules checks all rules for a key and returns the matching response
// Returns nil if no rule matches
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
