package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// CounterController demonstrates the Controller+State pattern.
// The Controller is a singleton that holds dependencies (none in this simple example).
type CounterController struct{}

// CounterState is pure data, cloned per session.
type CounterState struct {
	Title       string `json:"title"`
	Counter     int    `json:"counter"`
	LastUpdated string `json:"last_updated"`
}

func (c *CounterController) Increment(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Counter++
	state.LastUpdated = formatTime()
	return state, nil
}

func (c *CounterController) Decrement(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Counter--
	state.LastUpdated = formatTime()
	return state, nil
}

func (c *CounterController) Reset(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Counter = 0
	state.LastUpdated = formatTime()
	return state, nil
}

func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func main() {
	// --- Observability: structured logging ---
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}
	if err := envConfig.Validate(); err != nil {
		slog.Error("Invalid configuration", "error", err)
		os.Exit(1)
	}

	var level slog.Level
	switch envConfig.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	if os.Getenv("ENV") == "production" {
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(handler))

	// --- Application setup ---
	controller := &CounterController{}
	initialState := &CounterState{
		Title:       "Live Counter",
		Counter:     0,
		LastUpdated: formatTime(),
	}

	opts := envConfig.ToOptions()
	tmpl := livetemplate.Must(livetemplate.New("counter", opts...))
	liveHandler := tmpl.Handle(controller, livetemplate.AsState(initialState))

	mux := http.NewServeMux()
	mux.Handle("/", liveHandler)
	mux.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// --- Graceful shutdown ---
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
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

	shutdownTimeout := envConfig.ShutdownTimeout
	if shutdownTimeout == 0 {
		shutdownTimeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	slog.Info("Shutting down HTTP server...")
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("HTTP shutdown error", "error", err)
	}

	if s, ok := liveHandler.(interface{ Shutdown(context.Context) error }); ok {
		slog.Info("Shutting down WebSocket connections...")
		if err := s.Shutdown(ctx); err != nil {
			slog.Error("LiveHandler shutdown error", "error", err)
		}
	}

	slog.Info("Shutdown complete")
}
