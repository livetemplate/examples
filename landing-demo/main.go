// Minimal LiveTemplate counter, sized for a landing-page demo. The whole
// app fits in this file; the template is a single counter.tmpl. Per-session
// ephemeral state — each visitor has their own counter, and their own tabs
// stay in sync over WebSocket via the `lvt:"persist"` field tag.
package main

import (
	"log"
	"net/http"
	"os"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

type CounterController struct{}

type CounterState struct {
	Count int `json:"count" lvt:"persist"`
}

func (c *CounterController) Increment(s CounterState, ctx *livetemplate.Context) (CounterState, error) {
	s.Count++
	return s, nil
}

func (c *CounterController) Decrement(s CounterState, ctx *livetemplate.Context) (CounterState, error) {
	s.Count--
	return s, nil
}

func (c *CounterController) Reset(s CounterState, ctx *livetemplate.Context) (CounterState, error) {
	s.Count = 0
	return s, nil
}

func main() {
	tmpl := livetemplate.Must(livetemplate.New("counter",
		livetemplate.WithParseFiles("counter.tmpl"),
	))
	handler := tmpl.Handle(&CounterController{}, livetemplate.AsState(&CounterState{}))

	mux := http.NewServeMux()
	mux.Handle("/", handler)
	mux.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)
	mux.HandleFunc("/livetemplate.css", e2etest.ServeCSS)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("landing-demo listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
