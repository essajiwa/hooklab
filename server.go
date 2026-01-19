package main

// This file contains the HTTP server setup and route registration.

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

//go:embed web/*
var webFS embed.FS

// newServer creates and configures the HTTP server with all routes.
// It registers webhook handlers, API endpoints, and serves static files from the embedded filesystem.
func newServer(app *App, port int) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", app.webhookHandler)
	mux.HandleFunc("/webhook/", app.webhookHandler)
	mux.HandleFunc("/api/events", app.eventsHandler)
	mux.HandleFunc("/api/stream", app.eventsStreamHandler)
	mux.HandleFunc("/api/response", app.responseHandler)
	mux.HandleFunc("/api/response/", app.responseHandler)
	mux.HandleFunc("/api/rules", app.rulesHandler)
	mux.HandleFunc("/api/keys", app.keysHandler)

	webDir, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}
	mux.Handle("/", http.FileServer(http.FS(webDir)))

	server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	return server, nil
}
