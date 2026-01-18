package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	responseJSON := flag.String("response", `{"result":"ok"}`, "JSON string to be returned by the handler")
	port := flag.Int("port", 8080, "Port for the HTTP server")
	flag.Parse()

	var responseData interface{}
	if err := json.Unmarshal([]byte(*responseJSON), &responseData); err != nil {
		log.Fatalf("Invalid JSON for -response flag: %v", err)
	}

	app := &App{}
	app.setResponseConfig("default", ResponseConfig{
		Response:    responseData,
		ResponseRaw: string(*responseJSON),
		StatusCode:  http.StatusOK,
	})

	server, err := newServer(app, *port)
	if err != nil {
		log.Fatalf("Failed to create server: %v", err)
	}

	// Setting up a channel to listen for OS signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Goroutine to start the server
	go func() {
		log.Printf("Server starting on port %d...", *port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Could not start server: %v\n", err)
		}
	}()

	// Waiting for a signal
	<-stop

	log.Println("Server is shutting down...")

	// Create a context with a timeout for the shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Shutdown the server gracefully
	app.closeSubscribers()
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v\n", err)
	}

	log.Println("Server stopped gracefully")
}
