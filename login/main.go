package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// AuthState represents the authentication state for a session.
// It implements SessionAware to receive WebSocket connection events.
type AuthState struct {
	Username      string
	IsLoggedIn    bool
	Error         string
	ServerMessage string    // Message sent from server via WebSocket
	LoginTime     time.Time // When user logged in

	// For server-initiated updates
	session livetemplate.Session
	mu      sync.Mutex
}

// Login handles the "login" action
func (s *AuthState) Login(ctx *livetemplate.ActionContext) error {
	username := ctx.GetString("username")
	password := ctx.GetString("password")

	// Simple validation
	if username == "" || password == "" {
		s.Error = "Username and password are required"
		return nil
	}

	// Demo: accept any username with password "secret"
	if password != "secret" {
		s.Error = "Invalid credentials"
		return nil
	}

	// Clear error and set logged in state
	s.Error = ""
	s.Username = username
	s.IsLoggedIn = true
	s.LoginTime = time.Now()
	s.ServerMessage = "" // Will be set when WebSocket connects

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
		return fmt.Errorf("failed to set cookie: %w", err)
	}

	// Redirect to dashboard (page will load, then WebSocket connects)
	return ctx.Redirect("/", http.StatusSeeOther)
}

// Logout handles the "logout" action
func (s *AuthState) Logout(ctx *livetemplate.ActionContext) error {
	s.mu.Lock()
	s.Username = ""
	s.IsLoggedIn = false
	s.Error = ""
	s.ServerMessage = ""
	s.mu.Unlock()

	// Delete session cookie
	err := ctx.DeleteCookie("session_token")
	if err != nil {
		return fmt.Errorf("failed to delete cookie: %w", err)
	}

	// Redirect to login page
	return ctx.Redirect("/", http.StatusSeeOther)
}

// ServerWelcome handles the "serverWelcome" action (server-initiated welcome messages).
// This is triggered by TriggerAction from sendWelcomeMessage.
func (s *AuthState) ServerWelcome(ctx *livetemplate.ActionContext) error {
	message := ctx.GetString("message")
	s.mu.Lock()
	s.ServerMessage = message
	s.mu.Unlock()
	return nil
}

// OnConnect is called when a WebSocket connection is established.
// This implements livetemplate.SessionAware interface.
func (s *AuthState) OnConnect(ctx context.Context, session livetemplate.Session) error {
	s.mu.Lock()
	s.session = session
	isLoggedIn := s.IsLoggedIn
	username := s.Username
	s.mu.Unlock()

	log.Printf("WebSocket connected (user: %s, logged_in: %v)", username, isLoggedIn)

	// If user is logged in, send a welcome message from the server
	// This demonstrates server-initiated updates after WebSocket connects
	if isLoggedIn {
		go s.sendWelcomeMessage()
	}

	return nil
}

// OnDisconnect is called when a WebSocket connection is closed.
// This implements livetemplate.SessionAware interface.
func (s *AuthState) OnDisconnect() {
	s.mu.Lock()
	defer s.mu.Unlock()

	log.Printf("WebSocket disconnected (user: %s)", s.Username)
	s.session = nil
}

// sendWelcomeMessage sends a server-initiated welcome message via WebSocket.
// This demonstrates pushing updates from server to client without user action.
func (s *AuthState) sendWelcomeMessage() {
	// Small delay so the page fully renders first
	time.Sleep(500 * time.Millisecond)

	s.mu.Lock()
	session := s.session
	isLoggedIn := s.IsLoggedIn
	username := s.Username
	s.mu.Unlock()

	if session == nil || !isLoggedIn {
		return
	}

	// Trigger server-initiated action that will call Change() with the welcome data
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

	// Create initial state
	state := &AuthState{}

	// Create template with environment-based configuration
	tmpl := livetemplate.Must(livetemplate.New("auth", envConfig.ToOptions()...))

	// Set up handler
	http.Handle("/", tmpl.Handle(state))

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
