package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// CounterState demonstrates the simplified Controller pattern.
//
// Instead of implementing the Store interface with a Change() method that
// switches on action names, we use automatic method dispatch. Each action
// maps directly to a method:
//
//   - "increment" → Increment(ctx)
//   - "decrement" → Decrement(ctx)
//   - "reset"     → Reset(ctx)
//
// Actions are matched case-insensitively and support both camelCase and snake_case.
type CounterState struct {
	Title       string `json:"title"`
	Counter     int    `json:"counter"`
	LastUpdated string `json:"last_updated"`
}

// Increment handles the "increment" action
func (s *CounterState) Increment(ctx *livetemplate.ActionContext) error {
	s.Counter++
	s.LastUpdated = formatTime()
	return nil
}

// Decrement handles the "decrement" action
func (s *CounterState) Decrement(ctx *livetemplate.ActionContext) error {
	s.Counter--
	s.LastUpdated = formatTime()
	return nil
}

// Reset handles the "reset" action
func (s *CounterState) Reset(ctx *livetemplate.ActionContext) error {
	s.Counter = 0
	s.LastUpdated = formatTime()
	return nil
}

func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func main() {
	log.Println("LiveTemplate Counter Server starting...")

	// Load configuration from environment variables
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create initial state
	state := &CounterState{
		Title:       "Live Counter",
		Counter:     0,
		LastUpdated: formatTime(),
	}

	// Create template with environment-based configuration
	// Configuration is loaded from LVT_* environment variables
	tmpl := livetemplate.Must(livetemplate.New("counter", envConfig.ToOptions()...))

	// Mount handler - auto-handles initial page, WebSocket, and HTTP actions
	http.Handle("/", tmpl.Handle(state))

	// Serve client library (development only - use CDN in production)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on http://localhost:%s", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
