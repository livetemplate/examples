package main

import (
	"log"
	"net/http"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

var validate = validator.New()

func main() {
	log.Println("LiveTemplate Todo App starting...")

	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	dbPath := GetDBPath()
	queries, dbErr := InitDB(dbPath)
	if dbErr != nil {
		log.Fatalf("Failed to initialize database: %v", dbErr)
	}
	defer CloseDB()

	controller := &TodoController{
		Queries: queries,
	}

	auth := livetemplate.NewBasicAuthenticator(func(username, password string) (bool, error) {
		users := map[string]string{
			"alice": "password",
			"bob":   "password",
		}
		pass, ok := users[username]
		return ok && pass == password, nil
	})

	initialState := &TodoState{
		Title:       "Todo App",
		CurrentPage: DefaultPage,
		PageSize:    DefaultPageSize,
		LastUpdated: formatTime(),
	}

	opts := append(envConfig.ToOptions(), livetemplate.WithAuthenticator(auth))
	tmpl := livetemplate.Must(livetemplate.New("todos", opts...))

	http.Handle("/", tmpl.Handle(controller, livetemplate.AsState(initialState)))
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on http://localhost:%s", port)
	log.Println("Demo users: alice/password, bob/password")

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
