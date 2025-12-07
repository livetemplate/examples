package main

import (
	"log"
	"net/http"
	"os"

	"github.com/livetemplate/livetemplate"
	lvttest "github.com/livetemplate/lvt/testing"
)

// PageState represents the page data (cloned per session)
type PageState struct {
	Title   string
	Message string
	Count   int
}

// PageController handles page actions (singleton, holds dependencies)
type PageController struct{}

// No action methods needed for this static page

func main() {
	// Create template (will auto-discover welcome.tmpl)
	tmpl := livetemplate.Must(livetemplate.New("welcome"))

	// Create controller and initial state
	controller := &PageController{}
	initialState := &PageState{
		Title:   "Welcome",
		Message: "Hello from LiveTemplate!",
		Count:   42,
	}

	// Mount handler with Controller+State pattern
	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))

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
