// Minimal LiveTemplate counter, sized for a landing-page demo. The whole
// app fits in this file; the template is a single counter.tmpl. Per-session
// state means each visitor has their own counter; explicit Publish-to-
// SelfTopic calls keep their WebSocket-connected tabs in step.
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

// Mount subscribes the self-topic so peer tabs receive the Increment /
// Decrement / Reset dispatches Publish'd from the actions below.
func (c *CounterController) Mount(s CounterState, ctx *livetemplate.Context) (CounterState, error) {
	if err := ctx.Subscribe(ctx.SelfTopic()); err != nil {
		return s, err
	}
	return s, nil
}

func (c *CounterController) Increment(s CounterState, ctx *livetemplate.Context) (CounterState, error) {
	s.Count++
	if err := ctx.Publish(ctx.SelfTopic(), "Increment", nil); err != nil {
		return s, err
	}
	return s, nil
}

func (c *CounterController) Decrement(s CounterState, ctx *livetemplate.Context) (CounterState, error) {
	// Clamp at zero — a public landing-page demo showing "Count: -7"
	// reads as broken even though the math is fine.
	if s.Count > 0 {
		s.Count--
	}
	if err := ctx.Publish(ctx.SelfTopic(), "Decrement", nil); err != nil {
		return s, err
	}
	return s, nil
}

func (c *CounterController) Reset(s CounterState, ctx *livetemplate.Context) (CounterState, error) {
	s.Count = 0
	if err := ctx.Publish(ctx.SelfTopic(), "Reset", nil); err != nil {
		return s, err
	}
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
