package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestHandler(t *testing.T) {
	defaultResponse := map[string]string{"result": "ok"}
	app := &App{}
	app.setResponseConfig("default", ResponseConfig{Response: defaultResponse, StatusCode: http.StatusOK})

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	app.webhookHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var jsonStr = []byte(`{"title":"buy cheese and bread for breakfast"}`)
	req, err = http.NewRequest("POST", "/", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	app.webhookHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expected := `{"result":"ok"}`
	if strings.TrimSpace(rr.Body.String()) != expected {
		t.Errorf("handler returned unexpected body: got %v want %v",
			rr.Body.String(), expected)
	}

	customResponse := map[string]string{"status": "pending"}
	appWithCustomResponse := &App{}
	appWithCustomResponse.setResponseConfig("alpha", ResponseConfig{Response: customResponse, StatusCode: http.StatusOK})
	req, err = http.NewRequest("POST", "/webhook/alpha", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr = httptest.NewRecorder()
	appWithCustomResponse.webhookHandler(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler with custom response returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	expectedCustom := `{"status":"pending"}`
	if strings.TrimSpace(rr.Body.String()) != expectedCustom {
		t.Errorf("handler with custom response returned unexpected body: got %v want %v",
			rr.Body.String(), expectedCustom)
	}

	req, err = http.NewRequest("POST", "/", &errorReader{})
	if err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()
	app.webhookHandler(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code for body read error: got %v want %v",
			status, http.StatusInternalServerError)
	}

	req, err = http.NewRequest("POST", "/", bytes.NewBuffer(jsonStr))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	errorWriter := &errorResponseWriter{}
	app.webhookHandler(errorWriter, req)

	if status := errorWriter.status; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code for JSON encode error: got %v want %v",
			status, http.StatusInternalServerError)
	}
}

func TestWebhookHandlerStatusCode(t *testing.T) {
	app := &App{}
	app.setResponseConfig("default", ResponseConfig{Response: map[string]string{"ok": "true"}, StatusCode: http.StatusAccepted})
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(`{"ok":true}`))
	res := httptest.NewRecorder()

	app.webhookHandler(res, req)

	if status := res.Code; status != http.StatusAccepted {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusAccepted)
	}
}

func TestResponseHandler(t *testing.T) {
	app := &App{}
	app.setResponseConfig("alpha", ResponseConfig{Response: map[string]string{"hello": "world"}, StatusCode: http.StatusCreated})

	getReq := httptest.NewRequest(http.MethodGet, "/api/response?key=alpha", nil)
	getRes := httptest.NewRecorder()
	app.responseHandler(getRes, getReq)

	if status := getRes.Code; status != http.StatusOK {
		t.Errorf("response handler returned wrong status: got %v want %v", status, http.StatusOK)
	}

	var getPayload map[string]interface{}
	if err := json.Unmarshal(getRes.Body.Bytes(), &getPayload); err != nil {
		t.Fatalf("failed to parse response payload: %v", err)
	}

	if getPayload["statusCode"].(float64) != float64(http.StatusCreated) {
		t.Errorf("response handler returned wrong status code: got %v want %v", getPayload["statusCode"], http.StatusCreated)
	}

	postBody := `{"response":{"status":"ok"},"statusCode":202}`
	postReq := httptest.NewRequest(http.MethodPost, "/api/response?key=alpha", bytes.NewBufferString(postBody))
	postRes := httptest.NewRecorder()
	app.responseHandler(postRes, postReq)

	if status := postRes.Code; status != http.StatusOK {
		t.Errorf("response handler post returned wrong status: got %v want %v", status, http.StatusOK)
	}

	if config := app.getResponseConfig("alpha"); config.StatusCode != http.StatusAccepted {
		t.Errorf("response handler did not update status code: got %v want %v", config.StatusCode, http.StatusAccepted)
	}
}

func TestEventsHandler(t *testing.T) {
	app := &App{events: []Event{
		{ID: 1, Method: http.MethodPost, Path: "/webhook/alpha", Key: "alpha"},
		{ID: 2, Method: http.MethodPost, Path: "/webhook/beta", Key: "beta"},
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	res := httptest.NewRecorder()
	app.eventsHandler(res, req)

	if status := res.Code; status != http.StatusOK {
		t.Errorf("events handler returned wrong status: got %v want %v", status, http.StatusOK)
	}

	var payload EventsResponse
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse events response: %v", err)
	}
	if len(payload.Events) != 2 {
		t.Errorf("events handler returned unexpected payload: %+v", payload.Events)
	}

	filteredReq := httptest.NewRequest(http.MethodGet, "/api/events?key=alpha", nil)
	filteredRes := httptest.NewRecorder()
	app.eventsHandler(filteredRes, filteredReq)

	var filteredPayload EventsResponse
	if err := json.Unmarshal(filteredRes.Body.Bytes(), &filteredPayload); err != nil {
		t.Fatalf("failed to parse filtered events response: %v", err)
	}
	if len(filteredPayload.Events) != 1 || filteredPayload.Events[0].ID != 1 {
		t.Errorf("events handler returned unexpected payload: %+v", payload.Events)
	}
}

func TestResponseHandlerErrors(t *testing.T) {
	app := &App{}
	app.setResponseConfig("default", ResponseConfig{Response: map[string]string{"ok": "true"}, StatusCode: http.StatusOK})

	badBody := httptest.NewRequest(http.MethodPost, "/api/response", bytes.NewBufferString("{"))
	badRes := httptest.NewRecorder()
	app.responseHandler(badRes, badBody)
	if status := badRes.Code; status != http.StatusBadRequest {
		t.Errorf("response handler returned wrong status for invalid JSON: got %v want %v", status, http.StatusBadRequest)
	}

	errorReq := httptest.NewRequest(http.MethodPost, "/api/response", &errorReader{})
	errorRes := httptest.NewRecorder()
	app.responseHandler(errorRes, errorReq)
	if status := errorRes.Code; status != http.StatusInternalServerError {
		t.Errorf("response handler returned wrong status for read error: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestEventsStreamHandlerUnsupported(t *testing.T) {
	app := &App{}
	req := httptest.NewRequest(http.MethodGet, "/api/stream", nil)
	res := &noFlushWriter{}
	app.eventsStreamHandler(res, req)
	if status := res.status; status != http.StatusInternalServerError {
		t.Errorf("events stream handler returned wrong status: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestCloseSubscribers(t *testing.T) {
	app := &App{subscribers: make(map[chan Event]struct{})}
	ch := app.addSubscriber()
	app.closeSubscribers()
	app.removeSubscriber(ch)
}

func TestEventsStreamLoop(t *testing.T) {
	app := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/stream", nil).WithContext(ctx)
	writer := &sseWriter{}
	flusher := writer
	ticks := make(chan time.Time, 1)

	done := make(chan struct{})
	go func() {
		app.eventsStreamLoop(writer, req, flusher, ticks)
		close(done)
	}()

	for i := 0; i < 10; i++ {
		app.mu.Lock()
		subscriberCount := len(app.subscribers)
		app.mu.Unlock()
		if subscriberCount > 0 {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	ticks <- time.Now()
	app.broadcastEvent(Event{ID: 1, Method: http.MethodPost, Path: "/webhook", Key: "default"})
	time.Sleep(20 * time.Millisecond)
	cancel()
	app.closeSubscribers()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("events stream loop did not exit")
	}

	output := writer.buffer.String()
	if !strings.Contains(output, ": ping") {
		t.Errorf("expected ping in output, got %q", output)
	}
	if !strings.Contains(output, "data:") {
		t.Errorf("expected event data in output, got %q", output)
	}
}

func TestNewServer(t *testing.T) {
	app := &App{}
	server, err := newServer(app, 9090)
	if err != nil {
		t.Fatalf("newServer returned error: %v", err)
	}
	if server.Addr != ":9090" {
		t.Errorf("newServer returned wrong addr: got %v", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("newServer returned nil handler")
	}
}

func TestStoreEventMaxLimit(t *testing.T) {
	app := &App{}
	for i := 0; i < 60; i++ {
		req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
		app.storeEvent(req, "default", "body")
	}
	app.mu.Lock()
	count := len(app.events)
	app.mu.Unlock()
	if count != 50 {
		t.Errorf("storeEvent did not limit events: got %v want 50", count)
	}
}

func TestGetResponseConfigFallbacks(t *testing.T) {
	app := &App{}
	config := app.getResponseConfig("nonexistent")
	if config.StatusCode != 200 {
		t.Errorf("getResponseConfig fallback wrong status: got %v want 200", config.StatusCode)
	}

	app.setResponseConfig("default", ResponseConfig{Response: "default", StatusCode: 201})
	config = app.getResponseConfig("nonexistent")
	if config.StatusCode != 201 {
		t.Errorf("getResponseConfig default fallback wrong status: got %v want 201", config.StatusCode)
	}

	app.setResponseConfig("specific", ResponseConfig{Response: "specific", StatusCode: 202})
	config = app.getResponseConfig("specific")
	if config.StatusCode != 202 {
		t.Errorf("getResponseConfig specific wrong status: got %v want 202", config.StatusCode)
	}
}

func TestSetResponseConfigEmptyKey(t *testing.T) {
	app := &App{}
	app.setResponseConfig("", ResponseConfig{Response: "empty", StatusCode: 200})
	config := app.getResponseConfig("default")
	if config.Response != "empty" {
		t.Errorf("setResponseConfig empty key should set default: got %v", config.Response)
	}
}

func TestResponseHandlerMethodNotAllowed(t *testing.T) {
	app := &App{}
	req := httptest.NewRequest(http.MethodDelete, "/api/response", nil)
	res := httptest.NewRecorder()
	app.responseHandler(res, req)
	if status := res.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("response handler wrong status for DELETE: got %v want %v", status, http.StatusMethodNotAllowed)
	}
}

func TestResponseHandlerPathKey(t *testing.T) {
	app := &App{}
	app.setResponseConfig("pathkey", ResponseConfig{Response: "pathkey", StatusCode: 203})

	req := httptest.NewRequest(http.MethodGet, "/api/response/pathkey", nil)
	res := httptest.NewRecorder()
	app.responseHandler(res, req)

	var payload map[string]interface{}
	json.Unmarshal(res.Body.Bytes(), &payload)
	if payload["key"] != "pathkey" {
		t.Errorf("response handler path key wrong: got %v want pathkey", payload["key"])
	}
}

func TestWebhookKeyFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/webhook", "default"},
		{"/webhook/", "default"},
		{"/webhook/alpha", "alpha"},
		{"/webhook/alpha/beta", "alpha/beta"},
	}
	for _, tt := range tests {
		got := webhookKeyFromPath(tt.path)
		if got != tt.want {
			t.Errorf("webhookKeyFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestResponseKeyFromRequest(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/response/pathkey?key=querykey", nil)
	got := responseKeyFromRequest(req)
	if got != "querykey" {
		t.Errorf("responseKeyFromRequest query param: got %q want querykey", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/response/pathkey", nil)
	got = responseKeyFromRequest(req)
	if got != "pathkey" {
		t.Errorf("responseKeyFromRequest path: got %q want pathkey", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/response", nil)
	got = responseKeyFromRequest(req)
	if got != "default" {
		t.Errorf("responseKeyFromRequest default: got %q want default", got)
	}
}

func TestWebhookHandlerNilBody(t *testing.T) {
	app := &App{}
	app.setResponseConfig("default", ResponseConfig{Response: "ok", StatusCode: 200})
	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	req.Body = nil
	res := httptest.NewRecorder()
	app.webhookHandler(res, req)
	if status := res.Code; status != http.StatusOK {
		t.Errorf("webhook handler nil body wrong status: got %v want 200", status)
	}
}

func TestRemoveSubscriberNotExists(t *testing.T) {
	app := &App{subscribers: make(map[chan Event]struct{})}
	ch := make(chan Event)
	app.removeSubscriber(ch)
}

func TestBroadcastEventNoSubscribers(t *testing.T) {
	app := &App{}
	app.broadcastEvent(Event{ID: 1})
}

func TestResponseHandlerPostWithoutStatusCode(t *testing.T) {
	app := &App{}
	app.setResponseConfig("default", ResponseConfig{Response: "old", StatusCode: 201})

	postBody := `{"response":"new"}`
	req := httptest.NewRequest(http.MethodPost, "/api/response", bytes.NewBufferString(postBody))
	res := httptest.NewRecorder()
	app.responseHandler(res, req)

	config := app.getResponseConfig("default")
	if config.StatusCode != 201 {
		t.Errorf("response handler should keep status code: got %v want 201", config.StatusCode)
	}
	if config.Response != "new" {
		t.Errorf("response handler should update response: got %v want new", config.Response)
	}
}

func TestRemoveSubscriberExists(t *testing.T) {
	app := &App{subscribers: make(map[chan Event]struct{})}
	ch := app.addSubscriber()
	app.removeSubscriber(ch)
	app.mu.Lock()
	_, exists := app.subscribers[ch]
	app.mu.Unlock()
	if exists {
		t.Error("removeSubscriber should have removed the channel")
	}
}

func TestEventsStreamHandlerWithFlusher(t *testing.T) {
	app := &App{}
	app.setResponseConfig("default", ResponseConfig{Response: "ok", StatusCode: 200})

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/api/stream", nil).WithContext(ctx)

	res := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		app.eventsStreamHandler(res, req)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("eventsStreamHandler did not exit")
	}

	if ct := res.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("wrong content type: got %v want text/event-stream", ct)
	}
}

func TestEventsStreamLoopMarshalError(t *testing.T) {
	app := &App{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/stream", nil).WithContext(ctx)
	writer := &sseWriter{}
	ticks := make(chan time.Time)

	done := make(chan struct{})
	go func() {
		app.eventsStreamLoop(writer, req, writer, ticks)
		close(done)
	}()

	time.Sleep(10 * time.Millisecond)

	app.mu.Lock()
	for ch := range app.subscribers {
		select {
		case ch <- Event{ID: 1}:
		default:
		}
	}
	app.mu.Unlock()

	time.Sleep(10 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(100 * time.Millisecond):
		t.Fatal("eventsStreamLoop did not exit")
	}
}

// errorEventsWriter simulates JSON encode error for events
type errorEventsWriter struct {
	header http.Header
	count  int
}

func (ew *errorEventsWriter) Header() http.Header {
	if ew.header == nil {
		ew.header = make(http.Header)
	}
	return ew.header
}

func (ew *errorEventsWriter) Write(p []byte) (int, error) {
	ew.count++
	if ew.count > 1 {
		return 0, errors.New("simulated write error")
	}
	return len(p), nil
}

func (ew *errorEventsWriter) WriteHeader(statusCode int) {}

func TestEventsHandlerEncodeError(t *testing.T) {
	app := &App{events: []Event{{ID: 1}}}
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	res := &errorEventsWriter{}
	app.eventsHandler(res, req)
}

func TestEventsHandlerFilteredEncodeError(t *testing.T) {
	app := &App{events: []Event{{ID: 1, Key: "alpha"}}}
	req := httptest.NewRequest(http.MethodGet, "/api/events?key=alpha", nil)
	res := &errorEventsWriter{}
	app.eventsHandler(res, req)
}

func TestResponseHandlerGetEncodeError(t *testing.T) {
	app := &App{}
	app.setResponseConfig("default", ResponseConfig{Response: "ok", StatusCode: 200})
	req := httptest.NewRequest(http.MethodGet, "/api/response", nil)
	res := &errorResponseWriter{}
	app.responseHandler(res, req)
}

func TestResponseHandlerPostEncodeError(t *testing.T) {
	app := &App{}
	app.setResponseConfig("default", ResponseConfig{Response: "ok", StatusCode: 200})
	req := httptest.NewRequest(http.MethodPost, "/api/response", bytes.NewBufferString(`{"response":"new"}`))
	res := &errorResponseWriter{}
	app.responseHandler(res, req)
}

func TestWebhookHandlerZeroStatusCode(t *testing.T) {
	app := &App{}
	app.setResponseConfig("default", ResponseConfig{Response: "ok", StatusCode: 0})
	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewBufferString(`{}`))
	res := httptest.NewRecorder()
	app.webhookHandler(res, req)
	if status := res.Code; status != http.StatusOK {
		t.Errorf("webhook handler zero status: got %v want 200", status)
	}
}

func TestEventsHandlerNoEvents(t *testing.T) {
	app := &App{events: []Event{}}
	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	res := httptest.NewRecorder()
	app.eventsHandler(res, req)

	var payload EventsResponse
	json.Unmarshal(res.Body.Bytes(), &payload)
	if len(payload.Events) != 0 {
		t.Errorf("events should be empty: got %v", len(payload.Events))
	}
}

func TestEventsHandlerFilteredNoMatch(t *testing.T) {
	app := &App{events: []Event{{ID: 1, Key: "alpha"}}}
	req := httptest.NewRequest(http.MethodGet, "/api/events?key=beta", nil)
	res := httptest.NewRecorder()
	app.eventsHandler(res, req)

	var payload EventsResponse
	json.Unmarshal(res.Body.Bytes(), &payload)
	if len(payload.Events) != 0 {
		t.Errorf("filtered events should be empty: got %v", len(payload.Events))
	}
}

func TestEventsHandlerMultipleFilteredEvents(t *testing.T) {
	app := &App{events: []Event{
		{ID: 1, Key: "alpha"},
		{ID: 2, Key: "beta"},
		{ID: 3, Key: "alpha"},
	}}
	req := httptest.NewRequest(http.MethodGet, "/api/events?key=alpha", nil)
	res := httptest.NewRecorder()
	app.eventsHandler(res, req)

	var payload EventsResponse
	json.Unmarshal(res.Body.Bytes(), &payload)
	if len(payload.Events) != 2 {
		t.Errorf("filtered events count wrong: got %v want 2", len(payload.Events))
	}
}

func TestEventsHandlerWriteError(t *testing.T) {
	app := &App{}
	app.storeEvent(httptest.NewRequest(http.MethodPost, "/webhook", nil), "default", "test")

	req := httptest.NewRequest(http.MethodGet, "/api/events", nil)
	w := &errorResponseWriter{}

	app.eventsHandler(w, req)

	if w.status != http.StatusInternalServerError {
		t.Errorf("expected status 500 on write error, got %d", w.status)
	}
}

func TestEventsHandlerWithKeyWriteError(t *testing.T) {
	app := &App{}
	app.storeEvent(httptest.NewRequest(http.MethodPost, "/webhook/mykey", nil), "mykey", "test")

	req := httptest.NewRequest(http.MethodGet, "/api/events?key=mykey", nil)
	w := &errorResponseWriter{}

	app.eventsHandler(w, req)

	if w.status != http.StatusInternalServerError {
		t.Errorf("expected status 500 on write error, got %d", w.status)
	}
}

func TestKeysHandler(t *testing.T) {
	app := &App{}

	req := httptest.NewRequest(http.MethodGet, "/api/keys", nil)
	res := httptest.NewRecorder()
	app.keysHandler(res, req)

	if res.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.Code)
	}

	var payload map[string][]string
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	keys := payload["keys"]
	if len(keys) != 1 || keys[0] != "default" {
		t.Errorf("expected keys to contain only 'default', got %v", keys)
	}
}

func TestKeysHandlerWithMultipleKeys(t *testing.T) {
	app := &App{}

	app.setResponseConfig("key1", ResponseConfig{Response: map[string]string{"test": "1"}, StatusCode: 200})
	app.setResponseConfig("key2", ResponseConfig{Response: map[string]string{"test": "2"}, StatusCode: 200})
	app.storeEvent(httptest.NewRequest(http.MethodPost, "/webhook/key3", nil), "key3", "test")
	app.addRule("key4", Rule{Name: "test", Condition: "true", Enabled: true})

	req := httptest.NewRequest(http.MethodGet, "/api/keys", nil)
	res := httptest.NewRecorder()
	app.keysHandler(res, req)

	if res.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.Code)
	}

	var payload map[string][]string
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	keys := payload["keys"]
	expectedKeys := []string{"default", "key1", "key2", "key3", "key4"}
	if len(keys) != len(expectedKeys) {
		t.Errorf("expected %d keys, got %d: %v", len(expectedKeys), len(keys), keys)
	}

	for _, expected := range expectedKeys {
		found := false
		for _, k := range keys {
			if k == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected key '%s' not found in %v", expected, keys)
		}
	}
}

func TestKeysHandlerWriteError(t *testing.T) {
	app := &App{}

	req := httptest.NewRequest(http.MethodGet, "/api/keys", nil)
	w := &errorResponseWriter{}

	app.keysHandler(w, req)

	if w.status != http.StatusInternalServerError {
		t.Errorf("expected status 500 on write error, got %d", w.status)
	}
}

// ==================== Body Size Limit Tests ====================

func TestWebhookHandlerBodySizeLimit(t *testing.T) {
	app := &App{}
	app.setResponseConfig("default", ResponseConfig{Response: map[string]string{"result": "ok"}, StatusCode: 200})

	// Create a body larger than maxBodySize (1MB)
	largeBody := strings.Repeat("x", maxBodySize+1)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(largeBody))
	res := httptest.NewRecorder()

	app.webhookHandler(res, req)

	// Should still succeed but body is truncated to maxBodySize
	if res.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.Code)
	}

	// Verify the stored event has truncated body
	if len(app.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(app.events))
	}
	if len(app.events[0].Body) != maxBodySize {
		t.Errorf("expected body length %d, got %d", maxBodySize, len(app.events[0].Body))
	}
}

func TestResponseHandlerBodySizeLimit(t *testing.T) {
	app := &App{}

	// Create a body larger than maxBodySize (1MB)
	largeBody := strings.Repeat("x", maxBodySize+1)

	req := httptest.NewRequest(http.MethodPost, "/api/response?key=test", strings.NewReader(largeBody))
	res := httptest.NewRecorder()

	app.responseHandler(res, req)

	// Should fail with bad request since truncated body is invalid JSON
	if res.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 (invalid JSON after truncation), got %d", res.Code)
	}
}

func TestRulesHandlerPostBodySizeLimit(t *testing.T) {
	app := &App{}

	// Create a body larger than maxBodySize (1MB)
	largeBody := strings.Repeat("x", maxBodySize+1)

	req := httptest.NewRequest(http.MethodPost, "/api/rules?key=test", strings.NewReader(largeBody))
	res := httptest.NewRecorder()

	app.rulesHandler(res, req)

	// Should fail with bad request since truncated body is invalid JSON
	if res.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 (invalid JSON after truncation), got %d", res.Code)
	}
}

func TestRulesHandlerPutBodySizeLimit(t *testing.T) {
	app := &App{}
	app.addRule("test", Rule{Name: "Test", Condition: "true", Enabled: true})
	rules := app.getRules("test")
	ruleID := rules[0].ID

	// Create a body larger than maxBodySize (1MB)
	largeBody := strings.Repeat("x", maxBodySize+1)

	req := httptest.NewRequest(http.MethodPut, "/api/rules?key=test&id="+ruleID, strings.NewReader(largeBody))
	res := httptest.NewRecorder()

	app.rulesHandler(res, req)

	// Should fail with bad request since truncated body is invalid JSON
	if res.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 (invalid JSON after truncation), got %d", res.Code)
	}
}

func TestWebhookHandlerWithinBodySizeLimit(t *testing.T) {
	app := &App{}
	app.setResponseConfig("default", ResponseConfig{Response: map[string]string{"result": "ok"}, StatusCode: 200})

	// Create a body exactly at maxBodySize
	body := strings.Repeat("x", maxBodySize)

	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
	res := httptest.NewRecorder()

	app.webhookHandler(res, req)

	if res.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", res.Code)
	}

	// Verify the stored event has full body
	if len(app.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(app.events))
	}
	if len(app.events[0].Body) != maxBodySize {
		t.Errorf("expected body length %d, got %d", maxBodySize, len(app.events[0].Body))
	}
}
