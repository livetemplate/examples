package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// TodoController is a singleton that holds dependencies.
// In this example, we don't have external dependencies like a database.
type TodoController struct {
	validate *validator.Validate
}

// TodoState is pure data, cloned per session.
// It contains all the state needed to render the template.
type TodoState struct {
	Title string `json:"title"`
	Items []Todo `json:"items"`
	// Form input values (preserved on validation errors)
	InputTitle string `json:"input_title"`
}

// Todo represents a single todo item.
type Todo struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Completed bool   `json:"completed"`
	CreatedAt string `json:"created_at"`
}

// AddInput is the input struct for the Add action.
type AddInput struct {
	Title string `json:"title" validate:"required,min=3,max=100"`
}

// Mount is called once when a session is created.
func (c *TodoController) Mount(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	state.Title = "Progressive Enhancement Todo List"

	// Pre-populate with sample items if empty
	if len(state.Items) == 0 {
		state.Items = []Todo{
			{ID: "1", Title: "Learn about progressive enhancement", Completed: true, CreatedAt: formatTime()},
			{ID: "2", Title: "Try the app without JavaScript", Completed: false, CreatedAt: formatTime()},
			{ID: "3", Title: "Enable JavaScript and see the difference", Completed: false, CreatedAt: formatTime()},
		}
	}

	// Check for flash messages from URL (after redirect)
	if success := ctx.GetString("success"); success != "" {
		ctx.SetFlash("success", success)
	}
	if errorMsg := ctx.GetString("error"); errorMsg != "" {
		ctx.SetFlash("error", errorMsg)
	}

	return state, nil
}

// Add handles adding a new todo item.
func (c *TodoController) Add(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input AddInput
	if err := ctx.BindAndValidate(&input, c.validate); err != nil {
		// Preserve the input value so user doesn't have to retype
		state.InputTitle = ctx.GetString("title")
		return state, err
	}

	// Create new todo
	newID := fmt.Sprintf("%d", time.Now().UnixNano())
	state.Items = append(state.Items, Todo{
		ID:        newID,
		Title:     strings.TrimSpace(input.Title),
		Completed: false,
		CreatedAt: formatTime(),
	})

	// Clear the input field
	state.InputTitle = ""

	// Set flash message for success
	ctx.SetFlash("success", fmt.Sprintf("Added: %s", input.Title))

	return state, nil
}

// Toggle handles toggling a todo's completed status.
func (c *TodoController) Toggle(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	id := ctx.GetString("id")
	for i := range state.Items {
		if state.Items[i].ID == id {
			state.Items[i].Completed = !state.Items[i].Completed
			break
		}
	}
	return state, nil
}

// Delete handles deleting a todo item.
func (c *TodoController) Delete(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	id := ctx.GetString("id")
	for i := range state.Items {
		if state.Items[i].ID == id {
			state.Items = append(state.Items[:i], state.Items[i+1:]...)
			ctx.SetFlash("success", "Item deleted")
			break
		}
	}
	return state, nil
}

func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func main() {
	log.Println("Progressive Enhancement Example starting...")

	// Load configuration from environment variables
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create controller with validator
	controller := &TodoController{
		validate: validator.New(),
	}

	// Create initial state
	initialState := &TodoState{}

	// Create template with configuration
	// Progressive enhancement is enabled by default
	tmpl := livetemplate.Must(livetemplate.New("progressive-enhancement", envConfig.ToOptions()...))

	// Mount handler
	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))

	// Serve client library (development only)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on http://localhost:%s", port)
	log.Println("")
	log.Println("Try it:")
	log.Println("  1. Open in browser with JavaScript ENABLED - uses WebSocket for instant updates")
	log.Println("  2. Disable JavaScript and refresh - uses HTTP form submissions with page reloads")
	log.Println("  3. Both modes work identically, just with different performance characteristics")
	log.Println("")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
