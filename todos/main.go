// Package main implements a todo application demonstrating the LiveTemplate framework.
//
// The application uses the Controller+State pattern:
//   - Controller (TodoController): singleton holding dependencies, implements actions
//   - State (TodoState): pure data cloned per session, passed to templates
//
// File organization:
//   - main.go: server setup and entry point
//   - state.go: data structures and constants
//   - controller.go: TodoController and action methods
//   - helpers.go: utility functions for sorting, pagination, formatting
//   - db_manager.go: database initialization and migrations
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// validate is the shared validator instance for input validation.
var validate = validator.New()

func main() {
	log.Println("LiveTemplate Todo App starting...")

	// Load configuration from environment variables (LVT_* prefix)
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize SQLite database
	dbPath := GetDBPath()
	queries, dbErr := InitDB(dbPath)
	if dbErr != nil {
		log.Fatalf("Failed to initialize database: %v", dbErr)
	}
	defer CloseDB()

	// Create controller (singleton with database dependency)
	controller := &TodoController{
		Queries: queries,
	}

	// Create initial state (pure data, cloned per session)
	// Todos are loaded via Mount() when each session is created
	initialState := &TodoState{
		Title:       "Todo App",
		CurrentPage: DefaultPage,
		PageSize:    DefaultPageSize,
		LastUpdated: formatTime(),
	}

	// Create template with environment-based configuration
	tmpl := livetemplate.Must(livetemplate.New("todos", envConfig.ToOptions()...))

	// Mount handler with Controller+State pattern
	// - Controller: singleton with database dependency
	// - State: wrapped with AsState() for per-session cloning
	// - Mount() is called when each session is created to load initial todos
	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))

	// Serve client library (development only - use CDN in production)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on http://localhost:%s", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
