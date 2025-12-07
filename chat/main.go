package main

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/livetemplate/livetemplate"
)

// ChatController holds shared state (message store) with mutex protection.
// This is a singleton that holds dependencies and shared state.
type ChatController struct {
	mu            sync.RWMutex
	messages      []Message
	users         map[string]*User
	totalMessages int
}

// ChatState is pure data, cloned per session.
// Contains only serializable fields - the view of the chat for this user.
type ChatState struct {
	Messages      []Message      `json:"messages"`
	Users         map[string]*User `json:"users"`
	CurrentUser   string         `json:"current_user"`
	OnlineCount   int            `json:"online_count"`
	TotalMessages int            `json:"total_messages"`
}

type Message struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

type User struct {
	Username string    `json:"username"`
	JoinedAt time.Time `json:"joined_at"`
	IsOnline bool      `json:"is_online"`
}

// Mount is called when a new session is created - loads current messages
func (c *ChatController) Mount(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Copy current state to the session
	state.Messages = make([]Message, len(c.messages))
	copy(state.Messages, c.messages)

	state.Users = make(map[string]*User)
	for k, v := range c.users {
		userCopy := *v
		state.Users[k] = &userCopy
	}

	state.TotalMessages = c.totalMessages
	state.OnlineCount = c.countOnline()
	return state, nil
}

// Send handles the "send" action to send a chat message
func (c *ChatController) Send(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var data struct {
		Message string `json:"message"`
	}

	if err := ctx.Bind(&data); err != nil {
		log.Printf("Failed to bind message data: %v", err)
		return state, nil
	}

	if data.Message == "" {
		return state, nil
	}

	c.totalMessages++
	msg := Message{
		ID:        c.totalMessages,
		Username:  state.CurrentUser,
		Text:      data.Message,
		Timestamp: time.Now().Format("15:04:05"),
	}

	c.messages = append(c.messages, msg)

	// Update session state
	state.Messages = make([]Message, len(c.messages))
	copy(state.Messages, c.messages)
	state.TotalMessages = c.totalMessages

	// Auto-broadcast handles syncing to other tabs automatically
	return state, nil
}

// Join handles the "join" action when a user joins the chat
func (c *ChatController) Join(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var data struct {
		Username string `json:"username"`
	}

	if err := ctx.Bind(&data); err != nil {
		log.Printf("Failed to bind join data: %v", err)
		return state, nil
	}

	if data.Username == "" {
		return state, nil
	}

	state.CurrentUser = data.Username

	if _, exists := c.users[data.Username]; !exists {
		c.users[data.Username] = &User{
			Username: data.Username,
			JoinedAt: time.Now(),
			IsOnline: true,
		}
	}

	// Update session state
	state.Users = make(map[string]*User)
	for k, v := range c.users {
		userCopy := *v
		state.Users[k] = &userCopy
	}
	state.OnlineCount = c.countOnline()

	return state, nil
}

// Leave handles the "leave" action when a user leaves the chat
func (c *ChatController) Leave(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if state.CurrentUser != "" {
		if user, exists := c.users[state.CurrentUser]; exists {
			user.IsOnline = false
		}
	}

	// Update session state
	state.Users = make(map[string]*User)
	for k, v := range c.users {
		userCopy := *v
		state.Users[k] = &userCopy
	}
	state.OnlineCount = c.countOnline()

	return state, nil
}

func (c *ChatController) countOnline() int {
	count := 0
	for _, user := range c.users {
		if user.IsOnline {
			count++
		}
	}
	return count
}

func main() {
	log.Println("chat starting...")

	// Load configuration from environment variables
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create controller (singleton, holds shared state with mutex)
	controller := &ChatController{
		users:    make(map[string]*User),
		messages: []Message{},
	}

	// Create initial state (pure data, cloned per session)
	initialState := &ChatState{
		Users:    make(map[string]*User),
		Messages: []Message{},
	}

	// Create template with environment-based configuration
	// Uses default AnonymousAuthenticator - each browser gets its own session (via cookie)
	// Tabs in same browser share state
	// Configure via LVT_* environment variables (e.g., LVT_DEV_MODE=true)
	tmpl := livetemplate.Must(livetemplate.New("chat", envConfig.ToOptions()...))

	// Mount handler with Controller+State pattern
	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))

	// Serve client library
	http.HandleFunc("/livetemplate-client.js", serveClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	log.Printf("🚀 Chat server starting on http://localhost:%s", port)
	log.Println("📝 Open multiple browser tabs to see automatic syncing")
	log.Println("💬 Messages appear instantly in all tabs of the same browser")
	log.Println("🌐 Each browser has its own isolated chat session")
	log.Println()

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}

func serveClientLibrary(w http.ResponseWriter, r *http.Request) {
	paths := []string{
		"livetemplate-client.js",
		"../client/dist/livetemplate-client.browser.js",
		"../../client/dist/livetemplate-client.browser.js",
	}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err == nil {
			w.Header().Set("Content-Type", "application/javascript")
			w.Write(content)
			return
		}
	}

	http.Error(w, "Client library not found. For production, use CDN: https://cdn.jsdelivr.net/npm/@livefir/livetemplate-client/dist/livetemplate-client.browser.js", http.StatusNotFound)
}
