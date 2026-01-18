package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
)

func (a *App) webhookHandler(w http.ResponseWriter, r *http.Request) {
	key := webhookKeyFromPath(r.URL.Path)
	// Ensure r.Body is not nil for io.ReadAll
	if r.Body == nil {
		r.Body = http.NoBody
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "Error reading request body", http.StatusInternalServerError)
		return
	}
	defer r.Body.Close()

	event := a.storeEvent(r, key, string(body))
	a.broadcastEvent(event)
	config := a.getResponseConfig(key)

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
		body, err := io.ReadAll(r.Body)
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
