package main

import (
	"log"
	"net/http"
	"os"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// PreviewController demonstrates the Change() method convention.
// Having a Change() method causes the server to include "change" in the
// capabilities metadata, enabling Tier 1 auto-inferred input bindings.
type PreviewController struct{}

type PreviewState struct {
	Name    string `json:"name" lvt:"persist"`
	Preview string `json:"preview" lvt:"persist"`
}

func preview(name string) string {
	return "Hello, " + name + "!"
}

func (c *PreviewController) Change(state PreviewState, ctx *livetemplate.Context) (PreviewState, error) {
	// Only update Preview, not Name. The input value is managed by the browser
	// while the user types. Setting state.Name would cause the tree diff to
	// patch the input's value attribute, resetting the cursor position.
	name := ctx.GetString("Name")
	state.Preview = preview(name)
	return state, nil
}

func (c *PreviewController) Submit(state PreviewState, ctx *livetemplate.Context) (PreviewState, error) {
	state.Name = ctx.GetString("Name")
	state.Preview = "Saved: " + state.Name
	return state, nil
}

func main() {
	log.Println("LiveTemplate Live Preview Server starting...")

	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	controller := &PreviewController{}
	initialState := &PreviewState{Preview: preview("")}

	opts := envConfig.ToOptions()
	tmpl := livetemplate.Must(livetemplate.New("preview", opts...))
	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)
	http.HandleFunc("/livetemplate.css", e2etest.ServeCSS)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on http://localhost:%s", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
