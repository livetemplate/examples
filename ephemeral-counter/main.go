package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// CounterDB is a thread-safe in-memory counter that acts as the "database".
// With WithEphemeralState(), LiveTemplate state is rebuilt from this store
// on every request — no session persistence needed.
type CounterDB struct {
	mu    sync.Mutex
	value int
}

func (db *CounterDB) Get() int {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.value
}

func (db *CounterDB) Add(delta int) int {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.value += delta
	return db.value
}

func (db *CounterDB) Reset() {
	db.mu.Lock()
	defer db.mu.Unlock()
	db.value = 0
}

type CounterController struct {
	DB *CounterDB
}

type CounterState struct {
	Count int `json:"count"`
}

func (c *CounterController) Mount(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Count = c.DB.Get()
	return state, nil
}

func (c *CounterController) Increment(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Count = c.DB.Add(1)
	return state, nil
}

func (c *CounterController) Decrement(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Count = c.DB.Add(-1)
	return state, nil
}

func (c *CounterController) Reset(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	c.DB.Reset()
	state.Count = 0
	return state, nil
}

func main() {
	controller := &CounterController{DB: &CounterDB{}}

	tmpl := livetemplate.Must(livetemplate.New("counter"))
	handler := tmpl.Handle(controller, livetemplate.AsState(&CounterState{}),
		livetemplate.WithEphemeralState(),
	)

	mux := http.NewServeMux()
	mux.Handle("/", handler)
	mux.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		slog.Info("Server starting", "url", "http://localhost:"+port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Shutdown error", "error", err)
	}
}
