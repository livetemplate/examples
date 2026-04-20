package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

var validate = validator.New()

type DialogController struct{}

type DialogState struct {
	Items []Item
}

type Item struct {
	ID    string
	Title string
}

type AddInput struct {
	Title string `json:"title" validate:"required,min=3"`
}

func (c *DialogController) Mount(state DialogState, ctx *livetemplate.Context) (DialogState, error) {
	if len(state.Items) == 0 {
		state.Items = []Item{
			{ID: "1", Title: "Learn LiveTemplate"},
			{ID: "2", Title: "Build a dialog example"},
			{ID: "3", Title: "Write E2E tests"},
		}
	}
	return state, nil
}

func (c *DialogController) Add(state DialogState, ctx *livetemplate.Context) (DialogState, error) {
	var input AddInput
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		return state, err
	}
	id := fmt.Sprintf("%d", time.Now().UnixNano())
	state.Items = append(state.Items, Item{ID: id, Title: input.Title})
	return state, nil
}

func (c *DialogController) Delete(state DialogState, ctx *livetemplate.Context) (DialogState, error) {
	id := ctx.GetString("value")
	for i, item := range state.Items {
		if item.ID == id {
			state.Items = append(state.Items[:i], state.Items[i+1:]...)
			break
		}
	}
	return state, nil
}

func main() {
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

	controller := &DialogController{}
	initialState := &DialogState{}

	opts := envConfig.ToOptions()
	tmpl := livetemplate.Must(livetemplate.New("dialog-patterns", opts...))
	liveHandler := tmpl.Handle(controller, livetemplate.AsState(initialState))

	mux := http.NewServeMux()
	mux.Handle("/", liveHandler)
	mux.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)
	mux.HandleFunc("/livetemplate.css", e2etest.ServeCSS)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

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
