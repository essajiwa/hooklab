package main

// This file contains HTTP handlers for the Hooklab API endpoints.

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/expr-lang/expr"
)

// maxBodySize limits request body to 1MB to prevent DoS attacks.
const maxBodySize = 1 << 20 // 1MB

// webhookHandler handles incoming webhook requests at /webhook and /webhook/{key}.
// It stores the event, broadcasts it to SSE subscribers, evaluates rules, and returns
// the appropriate response.
func (a *App) webhookHandler(w http.ResponseWriter, r *http.Request) {
	key := webhookKeyFromPath(r.URL.Path)
	// Ensure r.Body is not nil for io.ReadAll
	if r.Body == nil {
		r.Body = http.NoBody
	}

	// Read body with size limit
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	event := a.storeEvent(r, key, string(body))
	a.broadcastEvent(event)

	// Try to match a rule first
	ruleConfig, _ := a.evaluateRules(key, string(body), r.Method, r.Header)
	var config ResponseConfig
	if ruleConfig != nil {
		config = *ruleConfig
	} else {
		config = a.getResponseConfig(key)
	}

	// Create JSON response
	w.Header().Set("Content-Type", "application/json")
	if config.StatusCode != 0 {
		w.WriteHeader(config.StatusCode)
	}
	if err := json.NewEncoder(w).Encode(config.Response); err != nil {
		http.Error(w, "Error creating response", http.StatusInternalServerError)
	}
}

// eventsHandler handles GET /api/events requests.
// Returns all stored events, optionally filtered by the "key" query parameter.
func (a *App) eventsHandler(w http.ResponseWriter, r *http.Request) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := r.URL.Query().Get("key")
	if key == "" {
		response := EventsResponse{Events: append([]Event(nil), a.events...)}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			http.Error(w, "Error creating response", http.StatusInternalServerError)
		}
		return
	}

	filtered := make([]Event, 0, len(a.events))
	for _, event := range a.events {
		if event.Key == key {
			filtered = append(filtered, event)
		}
	}
	response := EventsResponse{Events: filtered}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Error creating response", http.StatusInternalServerError)
	}
}

// responseHandler handles GET and POST requests to /api/response.
// GET returns the current response configuration for a key.
// POST updates the response configuration for a key.
func (a *App) responseHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		key := responseKeyFromRequest(r)
		config := a.getResponseConfig(key)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]interface{}{
			"response":   config.Response,
			"statusCode": config.StatusCode,
			"key":        key,
		}); err != nil {
			http.Error(w, "Error creating response", http.StatusInternalServerError)
		}
	case http.MethodPost:
		body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
		if err != nil {
			http.Error(w, "Error reading request body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		var payload map[string]interface{}
		if err := json.Unmarshal(body, &payload); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}

		responseData := payload["response"]
		statusCodeValue, hasStatus := payload["statusCode"]
		key := responseKeyFromRequest(r)
		statusCode := a.getResponseConfig(key).StatusCode
		if hasStatus {
			if floatVal, ok := statusCodeValue.(float64); ok {
				statusCode = int(floatVal)
			}
		}

		a.setResponseConfig(key, ResponseConfig{
			Response:    responseData,
			ResponseRaw: string(body),
			StatusCode:  statusCode,
		})

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "ok"}); err != nil {
			http.Error(w, "Error creating response", http.StatusInternalServerError)
		}
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// webhookKeyFromPath extracts the webhook key from a URL path.
// Returns "default" if no key is specified.
func webhookKeyFromPath(path string) string {
	key := strings.TrimPrefix(path, "/webhook")
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return "default"
	}
	return key
}

// responseKeyFromRequest extracts the response key from a request.
// Checks the "key" query parameter first, then the URL path.
func responseKeyFromRequest(r *http.Request) string {
	if key := r.URL.Query().Get("key"); key != "" {
		return key
	}
	key := strings.TrimPrefix(r.URL.Path, "/api/response")
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return "default"
	}
	return key
}

// keysHandler handles GET /api/keys requests.
// Returns a JSON array of all known webhook keys.
func (a *App) keysHandler(w http.ResponseWriter, r *http.Request) {
	keys := a.getKeys()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"keys": keys,
	}); err != nil {
		http.Error(w, "Error creating response", http.StatusInternalServerError)
	}
}

// rulesHandler handles CRUD operations for conditional response rules at /api/rules.
// Supports GET (list), POST (create), PUT (update), and DELETE operations.
// The "key" query parameter specifies which webhook key's rules to manage.
func (a *App) rulesHandler(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("key")
	if key == "" {
		key = "default"
	}

	switch r.Method {
	case http.MethodGet:
		a.handleGetRules(w, key)
	case http.MethodPost:
		a.handleCreateRule(w, r, key)
	case http.MethodPut:
		a.handleUpdateRule(w, r, key)
	case http.MethodDelete:
		a.handleDeleteRule(w, r, key)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetRules returns all rules for the given webhook key.
func (a *App) handleGetRules(w http.ResponseWriter, key string) {
	rules := a.getRules(key)
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"rules": rules,
		"key":   key,
	}); err != nil {
		http.Error(w, "Error creating response", http.StatusInternalServerError)
	}
}

// handleCreateRule creates a new rule for the given webhook key.
func (a *App) handleCreateRule(w http.ResponseWriter, r *http.Request, key string) {
	rule, ok := a.parseAndValidateRule(w, r)
	if !ok {
		return
	}

	created := a.addRule(key, rule)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(created)
}

// handleUpdateRule updates an existing rule identified by the "id" query parameter.
func (a *App) handleUpdateRule(w http.ResponseWriter, r *http.Request, key string) {
	ruleID := r.URL.Query().Get("id")
	if ruleID == "" {
		http.Error(w, "Rule ID required", http.StatusBadRequest)
		return
	}

	rule, ok := a.parseAndValidateRule(w, r)
	if !ok {
		return
	}

	if a.updateRule(key, ruleID, rule) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	} else {
		http.Error(w, "Rule not found", http.StatusNotFound)
	}
}

// handleDeleteRule removes a rule identified by the "id" query parameter.
func (a *App) handleDeleteRule(w http.ResponseWriter, r *http.Request, key string) {
	ruleID := r.URL.Query().Get("id")
	if ruleID == "" {
		http.Error(w, "Rule ID required", http.StatusBadRequest)
		return
	}

	if a.deleteRule(key, ruleID) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	} else {
		http.Error(w, "Rule not found", http.StatusNotFound)
	}
}

// parseAndValidateRule reads and validates a rule from the request body.
// It validates the expression syntax using the expr library.
// Returns the parsed rule and true on success, or writes an error response and returns false.
func (a *App) parseAndValidateRule(w http.ResponseWriter, r *http.Request) (Rule, bool) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize))
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return Rule{}, false
	}
	defer r.Body.Close()

	var rule Rule
	if err := json.Unmarshal(body, &rule); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return Rule{}, false
	}

	if rule.Condition != "" {
		env := map[string]interface{}{
			"body":    map[string]interface{}{},
			"method":  "",
			"headers": map[string][]string{},
		}
		if _, err := expr.Compile(rule.Condition, expr.Env(env), expr.AsBool()); err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error": "Invalid expression: " + err.Error(),
			})
			return Rule{}, false
		}
	}

	return rule, true
}
