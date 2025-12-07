package main

import (
	"log"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// CounterController is a singleton that holds dependencies.
type CounterController struct{}

// CounterState is pure data, cloned per session.
type CounterState struct {
	Title       string `json:"title"`
	Counter     int    `json:"counter"`
	LastUpdated string `json:"last_updated"`
}

// Increment handles the "increment" action
func (c *CounterController) Increment(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Counter++
	state.LastUpdated = formatTime()
	return state, nil
}

// Decrement handles the "decrement" action
func (c *CounterController) Decrement(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Counter--
	state.LastUpdated = formatTime()
	return state, nil
}

// Reset handles the "reset" action
func (c *CounterController) Reset(state CounterState, ctx *livetemplate.Context) (CounterState, error) {
	state.Counter = 0
	state.LastUpdated = formatTime()
	return state, nil
}

func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}

func main() {
	// ============================================================
	// CONFIGURATION - Load from environment variables
	// ============================================================

	// Load configuration from environment variables (LVT_* prefix)
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// ============================================================
	// OBSERVABILITY SETUP - Production-ready logging and metrics
	// ============================================================

	// Setup structured logging (JSON for production, Text for development)
	// Log level is controlled by LVT_LOG_LEVEL environment variable
	var handler slog.Handler
	var level slog.Level

	// Parse log level from config
	switch envConfig.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	if os.Getenv("ENV") == "production" {
		// JSON format for production (easy to parse by log aggregators)
		handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	} else {
		// Text format for development (human-readable)
		handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: level,
		})
	}

	logger := slog.New(handler)
	slog.SetDefault(logger)

	logger.Info("LiveTemplate Counter Server starting with observability enabled",
		"log_level", envConfig.LogLevel,
		"metrics_enabled", envConfig.MetricsEnabled,
		"dev_mode", envConfig.DevMode)

	// Note: For production metrics, integrate with your preferred metrics system
	// (Prometheus, StatsD, DataDog, etc.) by instrumenting the Change() method
	if envConfig.MetricsEnabled {
		logger.Info("Metrics collection enabled - integrate with your metrics backend")
	}

	// ============================================================
	// APPLICATION SETUP - With environment-based configuration
	// ============================================================

	// Create controller (singleton)
	controller := &CounterController{}

	// Create initial state (pure data, cloned per session)
	initialState := &CounterState{
		Title:       "Live Counter (with Observability)",
		Counter:     0,
		LastUpdated: formatTime(),
	}

	// Create template with environment-based configuration
	// Template operations are now automatically logged!
	// Configuration is loaded from LVT_* environment variables
	tmpl := livetemplate.Must(livetemplate.New("counter", envConfig.ToOptions()...))

	// Mount handler with Controller+State pattern
	// All actions and WebSocket events are now logged and metered!
	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))

	// Serve client library (development only - use CDN in production)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	// ============================================================
	// SERVER START
	// ============================================================

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	logger.Info("Server starting", "port", port, "url", "http://localhost:"+port)
	logger.Info("Try these URLs:",
		"counter", "http://localhost:"+port,
	)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		logger.Error("Server failed to start", "error", err)
		os.Exit(1)
	}
}
