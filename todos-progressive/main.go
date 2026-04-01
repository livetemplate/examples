// Progressive Complexity Demo: Todo App
//
// Demonstrates LiveTemplate's Tier 1 (Standard HTML) — ZERO lvt-* attributes.
// All action routing uses standard HTML:
//   - Form auto-submit → Submit() method
//   - button name="X" → X() method
//   - form name="X" → X() method
//
// Works at all three transport levels:
//   - No JS:         POST + PRG pattern (full page reload)
//   - JS + HTTP:     fetch POST + DOM patch
//   - JS + WebSocket: WS message + DOM patch
package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"slices"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

var validate = validator.New()

type Todo struct {
	ID    string
	Title string
	Done  bool
}

type TodoState struct {
	Items        []Todo `lvt:"persist"`
	ActiveFilter string `lvt:"persist"`
}

func (s TodoState) ActiveCount() int {
	count := 0
	for _, item := range s.Items {
		if !item.Done {
			count++
		}
	}
	return count
}

func (s TodoState) FilteredItems() []Todo {
	if s.ActiveFilter == "" || s.ActiveFilter == "all" {
		return s.Items
	}
	var filtered []Todo
	for _, item := range s.Items {
		if s.ActiveFilter == "active" && !item.Done {
			filtered = append(filtered, item)
		} else if s.ActiveFilter == "done" && item.Done {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

type TodoController struct {
	Logger *slog.Logger
}

// Submit handles the default form (no button name, no form name).
func (c *TodoController) Submit(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	var input struct {
		Title string `json:"Title" validate:"required,min=1,max=200"`
	}
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}

	state.Items = append(state.Items, Todo{
		ID:    uuid.New().String(),
		Title: input.Title,
	})
	c.Logger.Info("Todo added", slog.String("title", input.Title))
	return state, nil
}

// Toggle handles <button name="toggle">.
func (c *TodoController) Toggle(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	id := ctx.GetString("id")
	for i := range state.Items {
		if state.Items[i].ID == id {
			state.Items[i].Done = !state.Items[i].Done
			break
		}
	}
	return state, nil
}

// Delete handles <button name="delete">.
func (c *TodoController) Delete(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	id := ctx.GetString("id")
	state.Items = slices.DeleteFunc(state.Items, func(t Todo) bool { return t.ID == id })
	return state, nil
}

// Filter handles <form name="filter">.
func (c *TodoController) Filter(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	state.ActiveFilter = ctx.GetString("filter")
	return state, nil
}

func main() {
	controller := &TodoController{Logger: slog.Default()}

	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatal(err)
	}

	opts := envConfig.ToOptions()
	tmpl, err := livetemplate.New("todos", opts...)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := tmpl.ParseFiles("todos.tmpl"); err != nil {
		log.Fatal(err)
	}

	handler := tmpl.Handle(controller, livetemplate.AsState(&TodoState{
		Items: []Todo{
			{ID: uuid.New().String(), Title: "Read the progressive complexity guide", Done: false},
			{ID: uuid.New().String(), Title: "Try zero-attribute forms", Done: false},
			{ID: uuid.New().String(), Title: "Add lvt-* only when needed", Done: true},
		},
		ActiveFilter: "all",
	}))

	http.Handle("/", handler)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	fmt.Printf("Todo demo running at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
