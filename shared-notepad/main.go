package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// NotepadController is a stateless singleton — all state lives in NotepadState
// and is shared across tabs via WithSharedState.
type NotepadController struct{}

// NotepadState is shared across all tabs of the same authenticated user.
// WithSharedState means any change auto-broadcasts to all other tabs.
type NotepadState struct {
	Username  string `json:"username"`
	Content   string `json:"content"`
	SavedAt   string `json:"saved_at"`
	CharCount int    `json:"char_count"`
}

// Mount initializes the notepad for a new session.
func (c *NotepadController) Mount(state NotepadState, ctx *livetemplate.Context) (NotepadState, error) {
	state.Username = ctx.UserID()
	return state, nil
}

// Save persists the note (triggered by the Save button).
// WithSharedState auto-broadcasts to all other tabs of this user.
func (c *NotepadController) Save(state NotepadState, ctx *livetemplate.Context) (NotepadState, error) {
	state.Content = ctx.GetString("content")
	state.CharCount = len([]rune(state.Content))
	state.SavedAt = time.Now().Format("15:04:05")
	return state, nil
}

func main() {
	log.Println("shared-notepad starting...")

	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// BasicAuth: each user gets their own isolated session group.
	// The library's ChallengeAuthenticator sends WWW-Authenticate header
	// automatically, triggering the browser's login dialog.
	auth := livetemplate.NewBasicAuthenticator(func(username, password string) (bool, error) {
		// Demo: accept any username with password "demo"
		return password == "demo", nil
	})

	opts := envConfig.ToOptions()
	opts = append(opts,
		livetemplate.WithAuthenticator(auth),
		livetemplate.WithSharedState(), // All tabs of the same user share state
	)

	controller := &NotepadController{}
	initialState := &NotepadState{}

	tmpl := livetemplate.Must(livetemplate.New("notepad", opts...))

	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8097"
	}

	log.Printf("Shared Notepad starting on http://localhost:%s", port)
	log.Println("Login with any username, password: demo")
	log.Println("Open multiple tabs — Save syncs across all tabs of the same user")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
