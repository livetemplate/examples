package main

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

type NotepadController struct {
	mu    sync.RWMutex
	notes map[string]NotepadState // userID -> latest state
}

type NotepadState struct {
	Username  string `json:"username"`
	Content   string `json:"content" lvt:"persist"`
	SavedAt   string `json:"saved_at" lvt:"persist"`
	CharCount int    `json:"char_count" lvt:"persist"`
}

func (c *NotepadController) Mount(state NotepadState, ctx *livetemplate.Context) (NotepadState, error) {
	// Subscribe the self-topic so peer tabs of the same user receive the
	// Refresh dispatch from Save's Publish below — multi-device sync.
	if err := ctx.Subscribe(ctx.SelfTopic()); err != nil {
		return state, err
	}
	state.Username = ctx.UserID()
	c.mu.RLock()
	if saved, ok := c.notes[ctx.UserID()]; ok {
		state.Content = saved.Content
		state.CharCount = saved.CharCount
		state.SavedAt = saved.SavedAt
	}
	c.mu.RUnlock()
	return state, nil
}

func (c *NotepadController) Save(state NotepadState, ctx *livetemplate.Context) (NotepadState, error) {
	state.Content = ctx.GetString("content")
	state.CharCount = utf8.RuneCountInString(state.Content)
	state.SavedAt = time.Now().Format("15:04:05")

	c.mu.Lock()
	c.notes[ctx.UserID()] = state
	c.mu.Unlock()

	if err := ctx.Publish(ctx.SelfTopic(), "Refresh", nil); err != nil {
		return state, err
	}
	return state, nil
}

func (c *NotepadController) Change(state NotepadState, ctx *livetemplate.Context) (NotepadState, error) {
	if ctx.Has("content") {
		state.Content = ctx.GetString("content")
		state.CharCount = utf8.RuneCountInString(state.Content)
	}
	return state, nil
}

func (c *NotepadController) Refresh(state NotepadState, ctx *livetemplate.Context) (NotepadState, error) {
	c.mu.RLock()
	if saved, ok := c.notes[ctx.UserID()]; ok {
		state.Content = saved.Content
		state.CharCount = saved.CharCount
		state.SavedAt = saved.SavedAt
	}
	c.mu.RUnlock()
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

	auth := livetemplate.NewBasicAuthenticator(func(username, password string) (bool, error) {
		return password == "demo", nil
	})

	opts := envConfig.ToOptions()
	opts = append(opts, livetemplate.WithAuthenticator(auth))

	controller := &NotepadController{
		notes: make(map[string]NotepadState),
	}
	initialState := &NotepadState{}

	tmpl := livetemplate.Must(livetemplate.New("notepad", opts...))

	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)
	http.HandleFunc("/livetemplate.css", e2etest.ServeCSS)

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
