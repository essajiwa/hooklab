package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/expr-lang/expr"
)

// maxBodySize limits request body to 1MB to prevent DoS
const maxBodySize = 1 << 20 // 1MB

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

func webhookKeyFromPath(path string) string {
	key := strings.TrimPrefix(path, "/webhook")
	key = strings.TrimPrefix(key, "/")
	if key == "" {
		return "default"
	}
	return key
}

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

func (a *App) keysHandler(w http.ResponseWriter, r *http.Request) {
	keys := a.getKeys()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"keys": keys,
	}); err != nil {
		http.Error(w, "Error creating response", http.StatusInternalServerError)
	}
}

// Rules API handlers

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
