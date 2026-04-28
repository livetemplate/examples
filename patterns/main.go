package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// IndexController serves the pattern catalog index page.
type IndexController struct{}

// IndexState holds the categorized pattern list for the index page.
type IndexState struct {
	Title      string
	Category   string
	Categories []PatternCategory
}

func indexHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/index.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&IndexController{}, livetemplate.AsState(&IndexState{
		Categories: allPatterns(),
	}))
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
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	baseOpts := envConfig.ToOptions()

	mux := http.NewServeMux()

	// Index page
	mux.Handle("/", indexHandler(baseOpts))

	// Category: Forms & Editing (#1–#7)
	mux.Handle("/patterns/forms/click-to-edit", clickToEditHandler(baseOpts))
	mux.Handle("/patterns/forms/edit-row", editRowHandler(baseOpts))
	mux.Handle("/patterns/forms/inline-validation", inlineValidationHandler(baseOpts))
	mux.Handle("/patterns/forms/bulk-update", bulkUpdateHandler(baseOpts))
	mux.Handle("/patterns/forms/reset-input", resetInputHandler(baseOpts))
	mux.Handle("/patterns/forms/file-upload", fileUploadHandler(baseOpts))
	mux.Handle("/patterns/forms/preserve-inputs", preserveInputsHandler(baseOpts))

	// Category: Lists & Data
	mux.Handle("/patterns/lists/delete-row", deleteRowHandler(baseOpts))
	mux.Handle("/patterns/lists/click-to-load", clickToLoadHandler(baseOpts))
	mux.Handle("/patterns/lists/infinite-scroll", infiniteScrollHandler(baseOpts))
	mux.Handle("/patterns/lists/value-select", valueSelectHandler(baseOpts))
	mux.Handle("/patterns/lists/sortable", sortableHandler(baseOpts))

	// Category: Search & Filtering (#12–#13)
	mux.Handle("/patterns/search/active-search", activeSearchHandler(baseOpts))
	mux.Handle("/patterns/search/url-filters", urlFiltersHandler(baseOpts))

	// Category: Loading & Progress (#14–#16)
	mux.Handle("/patterns/loading/lazy-loading", lazyLoadingHandler(baseOpts))
	mux.Handle("/patterns/loading/progress-bar", progressBarHandler(baseOpts))
	mux.Handle("/patterns/loading/async-operations", asyncOperationsHandler(baseOpts))

	// Category: Dialogs, Tabs & Navigation (#17–#21)
	mux.Handle("/patterns/navigation/modal-dialog", modalDialogHandler(baseOpts))
	mux.Handle("/patterns/navigation/confirm-dialog", confirmDialogHandler(baseOpts))
	mux.Handle("/patterns/navigation/tabs", tabsHandler(baseOpts))
	mux.Handle("/patterns/navigation/spa-navigation", spaNavigationHandler(baseOpts))
	mux.Handle("/patterns/navigation/keyboard-shortcuts", keyboardShortcutsHandler(baseOpts))

	// Category: Visual Feedback (#22–#25)
	mux.Handle("/patterns/feedback/animations", animationsHandler(baseOpts))
	mux.Handle("/patterns/feedback/loading-states", loadingStatesHandler(baseOpts))
	mux.Handle("/patterns/feedback/highlight", highlightHandler(baseOpts))
	mux.Handle("/patterns/feedback/flash-messages", flashMessagesHandler(baseOpts))

	// Category: Real-Time & Multi-User (#26–#31)
	mux.Handle("/patterns/realtime/multi-user-sync", multiUserSyncHandler(baseOpts))
	mux.Handle("/patterns/realtime/broadcasting", broadcastingHandler(baseOpts))
	mux.Handle("/patterns/realtime/presence", presenceHandler(baseOpts))
	mux.Handle("/patterns/realtime/reconnection", reconnectionHandler(baseOpts))
	mux.Handle("/patterns/realtime/live-preview", livePreviewHandler(baseOpts))
	mux.Handle("/patterns/realtime/server-push", serverPushHandler(baseOpts))

	// Client library and CSS (dev mode)
	if localClient := os.Getenv("LVT_LOCAL_CLIENT"); localClient != "" {
		mux.HandleFunc("/livetemplate-client.js", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, localClient)
		})
	} else {
		mux.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)
	}
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
		slog.Info("Patterns server starting", "url", "http://localhost:"+port)
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

	slog.Info("Shutting down server...")
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Shutdown error", "error", err)
	}
	slog.Info("Shutdown complete")
}
