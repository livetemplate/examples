package main

import (
	"context"
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
	_ "modernc.org/sqlite"
)

type Todo struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Done  bool   `json:"done"`
}

// TodoController holds the database dependency.
type TodoController struct {
	DB *sql.DB
}

// TodoState is rebuilt from the database on every request.
type TodoState struct {
	Items []Todo `json:"items"`
}

func (c *TodoController) Mount(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	rows, err := c.DB.Query("SELECT id, title, done FROM todos ORDER BY id")
	if err != nil {
		return state, err
	}
	defer rows.Close()

	state.Items = nil
	for rows.Next() {
		var t Todo
		if err := rows.Scan(&t.ID, &t.Title, &t.Done); err != nil {
			return state, err
		}
		state.Items = append(state.Items, t)
	}
	return state, nil
}

func (c *TodoController) Submit(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	title := ctx.GetString("title")
	if title == "" {
		return state, livetemplate.FieldError{Field: "title", Message: "Title is required"}
	}

	result, err := c.DB.Exec("INSERT INTO todos (title, done) VALUES (?, 0)", title)
	if err != nil {
		return state, err
	}

	id, _ := result.LastInsertId()
	state.Items = append(state.Items, Todo{ID: int(id), Title: title, Done: false})
	return state, nil
}

func (c *TodoController) Toggle(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	id, _ := strconv.Atoi(ctx.GetString("value"))
	_, err := c.DB.Exec("UPDATE todos SET done = NOT done WHERE id = ?", id)
	if err != nil {
		return state, err
	}

	for i := range state.Items {
		if state.Items[i].ID == id {
			state.Items[i].Done = !state.Items[i].Done
			break
		}
	}
	return state, nil
}

func (c *TodoController) Delete(state TodoState, ctx *livetemplate.Context) (TodoState, error) {
	id, _ := strconv.Atoi(ctx.GetString("value"))
	_, err := c.DB.Exec("DELETE FROM todos WHERE id = ?", id)
	if err != nil {
		return state, err
	}

	for i := range state.Items {
		if state.Items[i].ID == id {
			state.Items = append(state.Items[:i], state.Items[i+1:]...)
			break
		}
	}
	return state, nil
}

func initDB() *sql.DB {
	db, err := sql.Open("sqlite", "file:todos.db?cache=shared&mode=rwc")
	if err != nil {
		slog.Error("Failed to open database", "error", err)
		os.Exit(1)
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS todos (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		done BOOLEAN NOT NULL DEFAULT 0
	)`)
	if err != nil {
		slog.Error("Failed to create table", "error", err)
		os.Exit(1)
	}
	return db
}

func main() {
	db := initDB()
	defer db.Close()

	controller := &TodoController{DB: db}

	opts := []livetemplate.Option{}
	if os.Getenv("LVT_WEBSOCKET_DISABLED") == "true" {
		opts = append(opts, livetemplate.WithWebSocketDisabled())
	}

	tmpl := livetemplate.Must(livetemplate.New("todos", opts...))
	handler := tmpl.Handle(controller, livetemplate.AsState(&TodoState{}),
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
	server.Shutdown(ctx)
}
