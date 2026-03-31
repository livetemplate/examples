package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// BookmarkController is a singleton that holds dependencies.
type BookmarkController struct{}

// BookmarkState is pure data, cloned per session.
type BookmarkState struct {
	Title     string     `json:"title"`
	Bookmarks []Bookmark `json:"bookmarks"`
	Count     int        `json:"count"`
}

// Bookmark represents a single bookmark.
type Bookmark struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Label string `json:"label"`
}

// Mount is called once when a session is created.
func (c *BookmarkController) Mount(state BookmarkState, ctx *livetemplate.Context) (BookmarkState, error) {
	state.Title = "Bookmarks (WebSocket Disabled)"
	return state, nil
}

// Add handles adding a new bookmark.
func (c *BookmarkController) Add(state BookmarkState, ctx *livetemplate.Context) (BookmarkState, error) {
	label := strings.TrimSpace(ctx.GetString("label"))
	url := strings.TrimSpace(ctx.GetString("url"))

	if label == "" {
		return state, livetemplate.NewFieldError("label", fmt.Errorf("label is required"))
	}
	if url == "" {
		return state, livetemplate.NewFieldError("url", fmt.Errorf("url is required"))
	}

	state.Bookmarks = append(state.Bookmarks, Bookmark{
		ID:    fmt.Sprintf("%d", time.Now().UnixNano()),
		URL:   url,
		Label: label,
	})
	state.Count++
	ctx.SetFlash("success", "Added: "+label)
	return state, nil
}

// Delete handles removing a bookmark.
func (c *BookmarkController) Delete(state BookmarkState, ctx *livetemplate.Context) (BookmarkState, error) {
	id := ctx.GetString("id")
	deleteIndex := -1
	for i, b := range state.Bookmarks {
		if b.ID == id {
			deleteIndex = i
			break
		}
	}

	if deleteIndex >= 0 {
		ctx.SetFlash("success", "Deleted: "+state.Bookmarks[deleteIndex].Label)
		state.Bookmarks = append(state.Bookmarks[:deleteIndex], state.Bookmarks[deleteIndex+1:]...)
		state.Count--
	} else {
		ctx.SetFlash("error", "Bookmark not found")
	}
	return state, nil
}

func main() {
	log.Println("WebSocket Disabled Example starting...")

	controller := &BookmarkController{}
	initialState := &BookmarkState{}

	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	opts := envConfig.ToOptions()
	opts = append(opts, livetemplate.WithWebSocketDisabled())

	tmpl := livetemplate.Must(livetemplate.New("ws-disabled", opts...))

	mux := http.NewServeMux()
	mux.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))
	mux.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on http://localhost:%s", port)
	log.Println("")
	log.Println("WebSocket is disabled. The client library uses HTTP fetch for actions")
	log.Println("and applies tree-based DOM updates without page reloads.")
	log.Println("")

	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
