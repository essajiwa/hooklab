package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net/http"
)

//go:embed web/*
var webFS embed.FS

func newServer(app *App, port int) (*http.Server, error) {
	mux := http.NewServeMux()
	mux.HandleFunc("/webhook", app.webhookHandler)
	mux.HandleFunc("/webhook/", app.webhookHandler)
	mux.HandleFunc("/api/events", app.eventsHandler)
	mux.HandleFunc("/api/stream", app.eventsStreamHandler)
	mux.HandleFunc("/api/response", app.responseHandler)
	mux.HandleFunc("/api/response/", app.responseHandler)
	mux.HandleFunc("/api/rules", app.rulesHandler)

	webDir, err := fs.Sub(webFS, "web")
	if err != nil {
		return nil, err
	}
	mux.Handle("/", http.FileServer(http.FS(webDir)))

	server := &http.Server{Addr: fmt.Sprintf(":%d", port), Handler: mux}
	return server, nil
}
