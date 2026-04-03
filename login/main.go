package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// AuthController holds shared state and dependencies.
// This is a singleton that persists across sessions.
type AuthController struct {
	// For server-initiated updates (per-session)
	sessions map[string]livetemplate.Session
	mu       sync.Mutex
}

// AuthState is pure data, cloned per session.
// Contains only serializable fields for the auth UI.
type AuthState struct {
	Username      string    `lvt:"persist"`
	IsLoggedIn    bool      `lvt:"persist"`
	ServerMessage string
	LoginTime     time.Time `lvt:"persist"`
}

// Login handles the "login" action
func (c *AuthController) Login(state AuthState, ctx *livetemplate.Context) (AuthState, error) {
	username := ctx.GetString("username")
	password := ctx.GetString("password")

	// Field-level validation
	if username == "" {
		return state, livetemplate.NewFieldError("username", fmt.Errorf("username is required"))
	}
	if password == "" {
		return state, livetemplate.NewFieldError("password", fmt.Errorf("password is required"))
	}

	// Demo: accept any username with password "secret"
	if password != "secret" {
		ctx.SetFlash("error", "Invalid credentials")
		return state, nil
	}
	state.Username = username
	state.IsLoggedIn = true
	state.LoginTime = time.Now()
	state.ServerMessage = "" // Will be set when WebSocket connects

	// Set HttpOnly session cookie
	err := ctx.SetCookie(&http.Cookie{
		Name:     "session_token",
		Value:    fmt.Sprintf("session_%s_%d", username, time.Now().Unix()),
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
		MaxAge:   3600, // 1 hour
	})
	if err != nil {
		return state, fmt.Errorf("failed to set cookie: %w", err)
	}

	// Redirect to dashboard (page will load, then WebSocket connects)
	return state, ctx.Redirect("/", http.StatusSeeOther)
}

// Logout handles the "logout" action
func (c *AuthController) Logout(state AuthState, ctx *livetemplate.Context) (AuthState, error) {
	state.Username = ""
	state.IsLoggedIn = false
	state.ServerMessage = ""

	// Delete session cookie
	err := ctx.DeleteCookie("session_token")
	if err != nil {
		return state, fmt.Errorf("failed to delete cookie: %w", err)
	}

	// Redirect to login page
	return state, ctx.Redirect("/", http.StatusSeeOther)
}

// ServerWelcome handles the "serverWelcome" action (server-initiated welcome messages).
// This is triggered by TriggerAction from sendWelcomeMessage.
func (c *AuthController) ServerWelcome(state AuthState, ctx *livetemplate.Context) (AuthState, error) {
	message := ctx.GetString("message")
	state.ServerMessage = message
	return state, nil
}

// OnConnect is called when a WebSocket connection is established.
// This is a lifecycle method on the controller.
func (c *AuthController) OnConnect(state AuthState, ctx *livetemplate.Context) (AuthState, error) {
	session := ctx.Session()

	log.Printf("WebSocket connected (user: %s, logged_in: %v)", state.Username, state.IsLoggedIn)

	// Store session for server-initiated updates
	if state.IsLoggedIn && session != nil {
		c.mu.Lock()
		c.sessions[state.Username] = session
		c.mu.Unlock()

		// Send a welcome message from the server
		// This demonstrates server-initiated updates after WebSocket connects
		go c.sendWelcomeMessage(state.Username, session)
	}

	return state, nil
}

// OnDisconnect is called when a WebSocket connection is closed.
// This is a lifecycle method on the controller.
func (c *AuthController) OnDisconnect() {
	log.Printf("WebSocket disconnected")
}

// sendWelcomeMessage sends a server-initiated welcome message via WebSocket.
// This demonstrates pushing updates from server to client without user action.
func (c *AuthController) sendWelcomeMessage(username string, session livetemplate.Session) {
	// Small delay so the page fully renders first
	time.Sleep(500 * time.Millisecond)

	// Trigger server-initiated action that returns modified state with the welcome data
	// This updates the state and sends the update to all user's connections
	if err := session.TriggerAction("serverWelcome", map[string]interface{}{
		"message": fmt.Sprintf("Welcome %s! This message was pushed from the server at %s",
			username, time.Now().Format("15:04:05")),
	}); err != nil {
		log.Printf("Failed to send welcome message: %v", err)
	} else {
		log.Printf("Server-initiated welcome message sent to %s", username)
	}
}

func main() {
	log.Println("LiveTemplate Login Example starting...")

	// Load configuration from environment variables
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create controller (singleton, holds session references)
	controller := &AuthController{
		sessions: make(map[string]livetemplate.Session),
	}

	// Create initial state (pure data, cloned per session)
	initialState := &AuthState{}

	// Create template with environment-based configuration
	opts := envConfig.ToOptions()
	tmpl := livetemplate.Must(livetemplate.New("auth", opts...))

	// Set up handler with Controller+State pattern
	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))

	// Serve client library (development only - use CDN in production)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	// Get port from environment or default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on http://localhost:%s", port)
	log.Println("Demo credentials: any username, password: 'secret'")
	log.Println("")
	log.Println("Flow:")
	log.Println("  1. Login via HTTP form (sets HttpOnly cookie)")
	log.Println("  2. Redirect to dashboard")
	log.Println("  3. WebSocket connects (authenticated via cookie)")
	log.Println("  4. Server pushes welcome message via WebSocket")
	log.Println("")
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
