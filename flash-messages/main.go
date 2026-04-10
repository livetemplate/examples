package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// FlashController demonstrates flash messages for page-level notifications.
//
// Flash messages are per-connection and show once (cleared after render).
// They don't affect ResponseMetadata.Success (unlike field validation errors).
//
// Common flash keys: "success", "error", "info", "warning"
type FlashController struct{}

// FlashState holds the demo data.
type FlashState struct {
	Title     string   `json:"title"`
	Items     []string `json:"items" lvt:"persist"`
	ItemCount int      `json:"item_count" lvt:"persist"`
}

// AddItem handles the "add_item" action - demonstrates success flash.
func (c *FlashController) AddItem(state FlashState, ctx *livetemplate.Context) (FlashState, error) {
	item := ctx.GetString("item")

	if item == "" {
		// Field validation error (affects Success: false)
		return state, livetemplate.FieldError{Field: "item", Message: "Item name is required"}
	}

	// Check for duplicates
	for _, existing := range state.Items {
		if existing == item {
			// Use flash for page-level warning (doesn't affect Success)
			ctx.SetFlash("warning", "Item already exists: "+item)
			return state, nil
		}
	}

	state.Items = append(state.Items, item)
	state.ItemCount = len(state.Items)

	// Success flash message
	ctx.SetFlash("success", "Added item: "+item)

	return state, nil
}

// RemoveItem handles the "remove_item" action - demonstrates flash on removal.
func (c *FlashController) RemoveItem(state FlashState, ctx *livetemplate.Context) (FlashState, error) {
	item := ctx.GetString("item")

	// Find and remove
	found := false
	newItems := make([]string, 0, len(state.Items))
	for _, existing := range state.Items {
		if existing == item {
			found = true
		} else {
			newItems = append(newItems, existing)
		}
	}

	if !found {
		ctx.SetFlash("error", "Item not found: "+item)
		return state, nil
	}

	state.Items = newItems
	state.ItemCount = len(state.Items)

	ctx.SetFlash("info", "Removed item: "+item)
	return state, nil
}

// ClearItems handles the "clear_items" action - demonstrates warning flash.
func (c *FlashController) ClearItems(state FlashState, ctx *livetemplate.Context) (FlashState, error) {
	if len(state.Items) == 0 {
		ctx.SetFlash("warning", "No items to clear")
		return state, nil
	}

	count := len(state.Items)
	state.Items = []string{}
	state.ItemCount = 0

	ctx.SetFlash("success", "Cleared all items ("+string(rune('0'+count))+" removed)")
	return state, nil
}

// SimulateError handles the "simulate_error" action - demonstrates error flash.
func (c *FlashController) SimulateError(state FlashState, ctx *livetemplate.Context) (FlashState, error) {
	// Simulate a server error that should be shown as flash
	ctx.SetFlash("error", "Something went wrong! Please try again.")
	return state, nil
}

// Mount initializes state with sample data.
func (c *FlashController) Mount(state FlashState, ctx *livetemplate.Context) (FlashState, error) {
	state.Title = "Flash Messages Demo"
	state.Items = []string{"Apple", "Banana", "Cherry"}
	state.ItemCount = len(state.Items)
	return state, nil
}

func main() {
	log.Println("LiveTemplate Flash Messages Example starting...")

	// Load configuration from environment variables
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create controller (singleton)
	controller := &FlashController{}

	// Create initial state (pure data, cloned per session)
	initialState := &FlashState{}

	// Create template with environment-based configuration
	opts := envConfig.ToOptions()
	tmpl := livetemplate.Must(livetemplate.New("flash", opts...))

	// Mount handler
	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))

	// Health check endpoint for testing
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","timestamp":"` + time.Now().Format(time.RFC3339) + `"}`))
	})

	// Serve client library (development only - use CDN in production)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)
	http.HandleFunc("/livetemplate.css", e2etest.ServeCSS)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on http://localhost:%s", port)
	log.Println("")
	log.Println("Flash Message Types:")
	log.Println("  - success: Green notification (e.g., 'Item added')")
	log.Println("  - error:   Red notification (e.g., 'Something went wrong')")
	log.Println("  - warning: Yellow notification (e.g., 'Item exists')")
	log.Println("  - info:    Blue notification (e.g., 'Item removed')")
	log.Println("")
	log.Println("Note: Flash messages show once and are cleared after each action.")
	log.Println("")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
