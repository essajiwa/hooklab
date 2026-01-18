package main

import (
	"encoding/json"
	"net/http"
	"time"
)

func (a *App) eventsStreamHandler(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	keepAlive := time.NewTicker(25 * time.Second)
	defer keepAlive.Stop()

	a.eventsStreamLoop(w, r, flusher, keepAlive.C)
}

func (a *App) eventsStreamLoop(w http.ResponseWriter, r *http.Request, flusher http.Flusher, ticks <-chan time.Time) {
	subscriber := a.addSubscriber()
	defer a.removeSubscriber(subscriber)

	for {
		select {
		case <-r.Context().Done():
			return
		case <-ticks:
			_, _ = w.Write([]byte(": ping\n\n"))
			flusher.Flush()
		case event, ok := <-subscriber:
			if !ok {
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(payload)
			_, _ = w.Write([]byte("\n\n"))
			flusher.Flush()
		}
	}
}
