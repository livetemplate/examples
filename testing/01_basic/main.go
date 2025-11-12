package main

import (
	"log"
	"net/http"
	"os"

	"github.com/livetemplate/livetemplate"
	lvttest "github.com/livetemplate/lvt/testing"
)

type PageState struct {
	Title   string
	Message string
	Count   int
}

// Change implements livetemplate.Store interface
func (s *PageState) Change(ctx *livetemplate.ActionContext) error {
	// No actions for this static page
	return nil
}

func main() {
	// Create template (will auto-discover welcome.tmpl)
	tmpl := livetemplate.Must(livetemplate.New("welcome"))

	// Create state
	state := &PageState{
		Title:   "Welcome",
		Message: "Hello from LiveTemplate!",
		Count:   42,
	}

	// Mount handler
	http.Handle("/", tmpl.Handle(state))

	// Serve client library
	http.HandleFunc("/livetemplate-client.js", lvttest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatal(err)
	}
}
