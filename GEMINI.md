# Hooklab

## Overview

Hooklab is a Go HTTP webhook server that accepts all methods at `/webhook` and `/webhook/{key}`, records incoming requests, and returns a configurable JSON response per key. It includes an embedded React + Tailwind monitoring UI served from `/` with key selector and color-coded HTTP methods.

## Prerequisites

- Go 1.18 or later

## Core Commands

**Build the server:**
```sh
go build -o webhook_server .
```

**Run the server:**
```sh
./webhook_server
```

**Run tests:**
```sh
go test -v
```

**Check test coverage:**
```sh
go test -cover
```

## Usage

To send a request to the server, use a tool like `curl`:

```sh
curl -X POST -H "Content-Type: application/json" -d '{"message":"hello world"}' http://localhost:8080/webhook
```

Or use a custom key for per-key response config:
```sh
curl -X POST -H "Content-Type: application/json" -d '{"message":"hello"}' http://localhost:8080/webhook/my-service
```

**Expected Response (default):**
```json
{"result":"ok"}
```

## Web UI

Open the monitoring UI at:

```
http://localhost:8080
```

## Configuration

- `-response`: JSON string for default key (default: `{"result":"ok"}`).
- `-port`: HTTP server port (default: `8080`).

## API Endpoints

- `POST|GET|... /webhook` or `/webhook/{key}`: capture events and return configured response for that key.
- `GET /api/events`: list recent events (use `?key=` to filter).
- `GET /api/stream`: SSE stream of all events.
- `GET /api/response?key={key}`: returns `{ response, statusCode, key }`.
- `POST /api/response?key={key}`: accepts `{ response, statusCode }` to update config for that key.