// Progressive Complexity Demo: Profile Editor
//
// Demonstrates LiveTemplate's Tier 1 (Standard HTML) — ZERO lvt-* attributes.
// The form auto-submits to the conventional Submit() method.
// Validation errors display via .lvt.HasError and .lvt.Error helpers.
package main

import (
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"

	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

var validate = validator.New()

type ProfileState struct {
	DisplayName string
	Email       string
	Bio         string
	Saved       bool
}

type ProfileController struct {
	Logger *slog.Logger
}

// Submit handles the default form submission (no button name, no form name).
func (c *ProfileController) Submit(state ProfileState, ctx *livetemplate.Context) (ProfileState, error) {
	var input struct {
		DisplayName string `json:"DisplayName" validate:"required,min=2,max=50"`
		Email       string `json:"Email" validate:"required,email"`
		Bio         string `json:"Bio" validate:"max=500"`
	}
	if err := ctx.BindAndValidate(&input, validate); err != nil {
		state.Saved = false
		return state, err
	}

	state.DisplayName = input.DisplayName
	state.Email = input.Email
	state.Bio = input.Bio
	state.Saved = true
	c.Logger.Info("Profile saved",
		slog.String("name", input.DisplayName),
		slog.String("email", input.Email))
	return state, nil
}

func main() {
	controller := &ProfileController{Logger: slog.Default()}

	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatal(err)
	}

	opts := append(envConfig.ToOptions(), livetemplate.WithStatePersistence())
	tmpl, err := livetemplate.New("profile", opts...)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := tmpl.ParseFiles("profile.tmpl"); err != nil {
		log.Fatal(err)
	}

	handler := tmpl.Handle(controller, livetemplate.AsState(&ProfileState{
		DisplayName: "Jane Doe",
		Email:       "jane@example.com",
		Bio:         "Go developer and open source enthusiast.",
	}))

	http.Handle("/", handler)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}
	fmt.Printf("Profile demo running at http://localhost:%s\n", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
