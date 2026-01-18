# Hooklab Architecture

## Purpose
Hooklab is a Go HTTP webhook server that listens on port **8080**, records incoming requests, and returns a configurable JSON response. It ships with an embedded React + Tailwind monitoring UI.

## High-Level Overview
- **Single binary** starts an HTTP server on `:8080` (configurable via `-port`).
- **Webhook routes** (`/webhook` and `/webhook/{key}`) accept **all HTTP methods**.
- **Per-key response config** — each key has its own JSON response + status code.
- **Configurable response** via API or `-response` flag (sets default key).
- **Embedded UI** served from `/` via `go:embed` with key selector.
- **Realtime stream** via SSE (`/api/stream`) with heartbeat pings.
- **Graceful shutdown** on SIGINT/SIGTERM with SSE cleanup.

## Request Flow
1. **Startup**
   - Parse `-response` and `-port` flags.
   - Initialize default response config in `App.responses` map.
   - Register handlers for `/webhook`, `/webhook/`, `/api/events`, `/api/stream`, `/api/response`, `/api/response/`, and `/`.
   - Start HTTP server.

2. **Request Handling**
   - Extract key from path (`/webhook/{key}` → key, `/webhook` → "default").
   - Store headers + body as an event with key association.
   - Broadcast event via SSE.
   - Respond with JSON from `App.responses[key]` (falls back to default).

3. **Shutdown**
   - Listen for OS signals (SIGINT/SIGTERM).
   - Close SSE subscribers and shutdown server with a timeout context.

## Components
- **`app.go`**: `App` state, `ResponseConfig` per key, events, subscriber management.
- **`handlers.go`**: Webhook + events + response handlers, key extraction helpers.
- **`sse.go`**: SSE handler + stream loop (heartbeat + events).
- **`server.go`**: Embedded web assets and server wiring.
- **`main.go`**: Flags, startup, graceful shutdown.

## Configuration
- `-response`: JSON string for default key (default: `{"result":"ok"}`).
- `-port`: HTTP server port (default: `8080`).
- `/api/response?key={key}` (GET): returns `{ response, statusCode, key }`.
- `/api/response?key={key}` (POST): accepts `{ response, statusCode }` to update config for that key.
- `/api/events?key={key}` (GET): filter events by key.

## Testing
Tests cover (84%+ coverage):
- Default and custom response bodies.
- Per-key response config and fallback logic.
- Status code responses and response config endpoints.
- Key extraction from path and query params.
- SSE stream loop (ping + event) and subscriber cleanup.
- Error handling for body read, JSON decoding, and JSON write failures.

## Extension Ideas
- Add route-specific handlers (e.g., `/health`).
- Add structured logging and request IDs.
- Add request size limits and JSON validation.
- Persist event history and response configs to storage.
- Add key management API (list, delete keys).
