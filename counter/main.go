package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// CounterController demonstrates the Controller+State pattern.
//
// The Controller is a singleton that holds dependencies (none in this simple example).
// Action methods receive state as first parameter and return modified state:
//
//   - "increment" → Increment(state, ctx) (state, error)
//   - "decrement" → Decrement(state, ctx) (state, error)
//   - "reset"     → Reset(state, ctx) (state, error)
//
// Actions are matched case-insensitively and support both camelCase and snake_case.
type CounterController struct{}

// CounterState is pure data, cloned per session.
type CounterState struct {
	Title       string `json:"title"`
	Counter     int    `json:"counter"`
	LastUpdated string `json:"last_updated"`
}

// Increment handles the "increment" action
func (c *CounterController) Increment(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Counter++
	state.LastUpdated = formatTime()
	return state, nil
}

// Decrement handles the "decrement" action
func (c *CounterController) Decrement(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Counter--
	state.LastUpdated = formatTime()
	return state, nil
}

// Reset handles the "reset" action
func (c *CounterController) Reset(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Counter = 0
	state.LastUpdated = formatTime()
	return state, nil
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

	// Create controller (singleton, holds dependencies)
	controller := &CounterController{}

	// Create initial state (pure data, cloned per session)
	initialState := &CounterState{
		Title:       "Live Counter",
		Counter:     0,
		LastUpdated: formatTime(),
	}

	// Create template with environment-based configuration
	// Configuration is loaded from LVT_* environment variables
	tmpl := livetemplate.Must(livetemplate.New("counter", envConfig.ToOptions()...))

	// Mount handler with Controller+State pattern
	// - Controller: singleton with dependencies
	// - State: wrapped with AsState() for per-session cloning
	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))

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
