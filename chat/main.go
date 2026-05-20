package main

import (
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// ChatController is a singleton holding shared data (messages, users).
// With per-connection state, the controller is the single source of truth
// for data that all tabs need to see. Each tab has its own ChatState clone.
type ChatController struct {
	mu            sync.RWMutex
	messages      []Message
	users         map[string]bool // username → online
	totalMessages int
}

// ChatState is per-connection — each tab gets its own independent copy.
// CurrentUser is never shared across tabs.
type ChatState struct {
	Messages      []Message `json:"messages"`
	CurrentUser   string    `json:"current_user" lvt:"persist"`
	OnlineCount   int       `json:"online_count"`
	TotalMessages int       `json:"total_messages"`
}

type Message struct {
	ID        int    `json:"id"`
	Username  string `json:"username"`
	Text      string `json:"text"`
	Timestamp string `json:"timestamp"`
}

// Mount runs once per session group. Subscribes the self-topic so peer tabs
// receive the UserJoined / NewMessage / UserLeft dispatches Publish'd below.
func (c *ChatController) Mount(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	if err := ctx.Subscribe(ctx.SelfTopic()); err != nil {
		return state, err
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	state.Messages = c.copyMessages()
	state.TotalMessages = c.totalMessages
	state.OnlineCount = c.countOnline()
	return state, nil
}

// OnConnect is called every WebSocket connection (every tab).
// Each tab starts with no user — must join independently.
func (c *ChatController) OnConnect(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state.CurrentUser = ""
	state.Messages = c.copyMessages()
	state.TotalMessages = c.totalMessages
	state.OnlineCount = c.countOnline()
	return state, nil
}

// Join handles the "join" action when a user joins in this tab.
// Sets CurrentUser on this connection only, then broadcasts to other tabs.
func (c *ChatController) Join(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	username := ctx.GetString("username")
	if username == "" {
		return state, nil
	}

	c.mu.Lock()
	state.CurrentUser = username
	c.users[username] = true
	state.OnlineCount = c.countOnline()
	c.mu.Unlock()

	// Tell other tabs someone joined. We propagate Publish's error rather than
	// log-and-swallow because the only errors it can return are programmer
	// errors (empty SelfTopic from a misconfigured Authenticator, or the
	// per-action publish cap exceeded). Surfacing them loudly is a feature.
	// Same pattern applies to every Publish call site in this file.
	if err := ctx.Publish(ctx.SelfTopic(), "UserJoined", nil); err != nil {
		return state, err
	}
	return state, nil
}

// UserJoined is dispatched on other connections when someone joins.
// Each tab refreshes its online count from the controller.
func (c *ChatController) UserJoined(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state.OnlineCount = c.countOnline()
	return state, nil
}

// Send handles the "send" action to send a chat message.
// Adds message to shared store, updates this tab, broadcasts to others.
func (c *ChatController) Send(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	text := ctx.GetString("message")
	if text == "" || state.CurrentUser == "" {
		return state, nil
	}

	c.mu.Lock()
	c.totalMessages++
	msg := Message{
		ID:        c.totalMessages,
		Username:  state.CurrentUser,
		Text:      text,
		Timestamp: time.Now().Format("15:04:05"),
	}
	c.messages = append(c.messages, msg)
	state.Messages = c.copyMessages()
	state.TotalMessages = c.totalMessages
	c.mu.Unlock()

	// Tell other tabs about the new message
	if err := ctx.Publish(ctx.SelfTopic(), "NewMessage", nil); err != nil {
		return state, err
	}
	return state, nil
}

// NewMessage is dispatched on other connections when a message is sent.
// Each tab reloads messages from the controller.
func (c *ChatController) NewMessage(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state.Messages = c.copyMessages()
	state.TotalMessages = c.totalMessages
	return state, nil
}

// Leave handles the "leave" action.
func (c *ChatController) Leave(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	if state.CurrentUser == "" {
		return state, nil
	}

	c.mu.Lock()
	delete(c.users, state.CurrentUser)
	state.CurrentUser = ""
	state.OnlineCount = c.countOnline()
	c.mu.Unlock()

	if err := ctx.Publish(ctx.SelfTopic(), "UserLeft", nil); err != nil {
		return state, err
	}
	return state, nil
}

// UserLeft is dispatched on other connections when someone leaves.
func (c *ChatController) UserLeft(state ChatState, ctx *livetemplate.Context) (ChatState, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	state.OnlineCount = c.countOnline()
	return state, nil
}

func (c *ChatController) countOnline() int {
	count := 0
	for _, online := range c.users {
		if online {
			count++
		}
	}
	return count
}

func (c *ChatController) copyMessages() []Message {
	msgs := make([]Message, len(c.messages))
	copy(msgs, c.messages)
	return msgs
}

func main() {
	log.Println("chat starting...")

	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	controller := &ChatController{
		users: make(map[string]bool),
	}

	initialState := &ChatState{}

	tmpl := livetemplate.Must(livetemplate.New("chat", envConfig.ToOptions()...))

	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)
	http.HandleFunc("/livetemplate.css", e2etest.ServeCSS)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	log.Printf("Chat server starting on http://localhost:%s", port)
	log.Println("Open multiple tabs — each tab joins independently")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
