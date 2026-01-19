# Hooklab Architecture

## Purpose
Hooklab is a Go HTTP webhook server that listens on port **8080**, records incoming requests, and returns a configurable JSON response. It ships with an embedded React + Tailwind monitoring UI and a powerful expression-based rule engine.

## High-Level Overview
- **Single binary** starts an HTTP server on `:8080` (configurable via `-port`).
- **Webhook routes** (`/webhook` and `/webhook/{key}`) accept **all HTTP methods**.
- **Per-key response config** — each key has its own JSON response + status code.
- **Rule engine** — expression-based conditional responses per webhook key.
- **Configurable response** via API or `-response` flag (sets default key).
- **Embedded UI** served from `/` via `go:embed` with key selector.
- **Rules UI** at `/rules.html` for managing conditional response rules.
- **Realtime stream** via SSE (`/api/stream`) with heartbeat pings.
- **Graceful shutdown** on SIGINT/SIGTERM with SSE cleanup.

## Request Flow
1. **Startup**
   - Parse `-response` and `-port` flags.
   - Initialize default response config in `App.responses` map.
   - Register handlers for `/webhook`, `/webhook/`, `/api/events`, `/api/stream`, `/api/response`, `/api/response/`, `/api/rules`, `/api/keys`, and `/`.
   - Start HTTP server.

2. **Request Handling**
   - Extract key from path (`/webhook/{key}` → key, `/webhook` → "default").
   - Store headers + body as an event with key association.
   - Broadcast event via SSE.
   - **Evaluate rules** for the key (first matching rule wins).
   - If no rule matches, respond with JSON from `App.responses[key]` (falls back to default).

3. **Shutdown**
   - Listen for OS signals (SIGINT/SIGTERM).
   - Close SSE subscribers and shutdown server with a timeout context.

## Components
- **`app.go`**: `App` state, `ResponseConfig` per key, `Rule` struct, events, subscriber management, rule evaluation with [expr](https://github.com/expr-lang/expr).
- **`handlers.go`**: Webhook + events + response + rules + keys handlers, key extraction helpers.
- **`sse.go`**: SSE handler + stream loop (heartbeat + events).
- **`server.go`**: Embedded web assets and server wiring.
- **`main.go`**: Flags, startup, graceful shutdown.
- **`web/index.html`**: Main monitoring UI with response configuration.
- **`web/rules.html`**: Rule configuration UI with expression editor.

## Configuration
- `-response`: JSON string for default key (default: `{"result":"ok"}`).
- `-port`: HTTP server port (default: `8080`).
- `/api/response?key={key}` (GET): returns `{ response, statusCode, key }`.
- `/api/response?key={key}` (POST): accepts `{ response, statusCode }` to update config for that key.
- `/api/events?key={key}` (GET): filter events by key.
- `/api/keys` (GET): list all known webhook keys (from events, responses, and rules).

## Rule Engine
Rules allow conditional responses based on request data. See [RULES.md](RULES.md) for full documentation.

### Rule Structure
```go
type Rule struct {
    ID         string      // Auto-generated unique ID
    Name       string      // Human-readable name
    Condition  string      // expr expression (e.g., "body.amount > 100")
    Response   interface{} // JSON response to return
    StatusCode int         // HTTP status code
    Priority   int         // Lower = higher priority
    Enabled    bool        // Toggle rule on/off
}
```

### Evaluation Flow
1. Rules are sorted by priority (ascending).
2. Each enabled rule's condition is evaluated against `{ body, method, headers }`.
3. First matching rule's response is returned.
4. If no rule matches, default response config is used.

### API Endpoints
- `GET /api/rules?key={key}` — List rules for a key.
- `POST /api/rules?key={key}` — Create rule (validates expression).
- `PUT /api/rules?key={key}&id={id}` — Update rule.
- `DELETE /api/rules?key={key}&id={id}` — Delete rule.

## Key Management
Webhook keys are automatically tracked from multiple sources:
- **Events**: Keys from received webhook requests.
- **Responses**: Keys with configured response configs.
- **Rules**: Keys with associated rules.

The `/api/keys` endpoint returns all known keys, enabling the UI to show a dropdown of available keys across pages. Keys persist in browser localStorage and sync with the backend on page load.

## Extension Ideas
- Add route-specific handlers (e.g., `/health`).
- Add structured logging and request IDs.
- Add request size limits and JSON validation.
- Persist event history and response configs to storage.
