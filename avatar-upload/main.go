package main

import (
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/livetemplate/livetemplate"
	lvttest "github.com/livetemplate/lvt/testing"
)

//go:embed *.tmpl
var templates embed.FS

// ProfileController is a singleton that holds dependencies.
type ProfileController struct{}

// ProfileState is pure data, cloned per session.
type ProfileState struct {
	Name       string `lvt:"persist"`
	Email      string `lvt:"persist"`
	AvatarPath string `lvt:"persist"`
	AvatarURL  string `lvt:"persist"`
}

// UpdateProfile handles the "UpdateProfile" action for profile update form submission
func (c *ProfileController) UpdateProfile(state ProfileState, ctx *livetemplate.Context) (ProfileState, error) {
	state.Name = ctx.GetString("name")
	state.Email = ctx.GetString("email")

	// Also process avatar if it was uploaded with the form
	if ctx.HasUploads("avatar") {
		var err error
		state, err = c.processAvatarUpload(state, ctx)
		if err != nil {
			return state, err
		}
	}

	ctx.SetFlash("success", "Profile updated")
	log.Printf("Profile updated: name=%s, email=%s", state.Name, state.Email)
	return state, nil
}

// processAvatarUpload handles avatar upload processing
func (c *ProfileController) processAvatarUpload(state ProfileState, ctx *livetemplate.Context) (ProfileState, error) {
	// Get completed uploads from Context
	uploads := ctx.GetCompletedUploads("avatar")
	log.Printf("DEBUG: ProcessAvatarUpload called, found %d completed uploads", len(uploads))
	if len(uploads) == 0 {
		log.Printf("DEBUG: No completed uploads found")
		return state, nil // No uploads to process
	}

	// Create uploads directory if it doesn't exist
	uploadsDir := "uploads"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return state, fmt.Errorf("failed to create uploads directory: %w", err)
	}

	for _, entry := range uploads {
		log.Printf("DEBUG: Processing entry %s, TempPath: %s, exists: %v", entry.ID, entry.TempPath, fileExists(entry.TempPath))

		// Check if temp file exists (may have been processed already by auto-trigger)
		if !fileExists(entry.TempPath) {
			log.Printf("DEBUG: Temp file already processed for entry %s, skipping", entry.ID)
			continue
		}

		// Generate permanent filename
		ext := filepath.Ext(entry.ClientName)
		permanentPath := filepath.Join(uploadsDir, fmt.Sprintf("avatar-%s%s", entry.ID, ext))

		// Move from temp to permanent location
		if err := os.Rename(entry.TempPath, permanentPath); err != nil {
			log.Printf("DEBUG: Rename failed: %v, trying copy", err)
			// If rename fails (different filesystem), try copy
			if err := copyFile(entry.TempPath, permanentPath); err != nil {
				return state, fmt.Errorf("failed to save avatar: %w", err)
			}
			os.Remove(entry.TempPath) // Clean up temp file
		}

		// Update state with new avatar
		state.AvatarPath = permanentPath
		state.AvatarURL = "/" + permanentPath

		log.Printf("Avatar saved: %s (original: %s, size: %d bytes)", permanentPath, entry.ClientName, entry.ClientSize)
	}

	return state, nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func main() {
	// Parse port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create LiveTemplate instance with upload configuration
	lt := livetemplate.Must(livetemplate.New("avatar-upload",
		livetemplate.WithParseFiles("avatar-upload.tmpl"),
		livetemplate.WithDevMode(true),
		// Configure upload using WithUpload option
		livetemplate.WithUpload("avatar", livetemplate.UploadConfig{
			Accept:      []string{"image/jpeg", "image/png", "image/gif"},
			MaxFileSize: 5 * 1024 * 1024, // 5MB
			MaxEntries:  1,                // Single file
		}),
	))

	// Create controller (singleton)
	controller := &ProfileController{}

	// Create initial state (pure data, cloned per session)
	initialState := &ProfileState{
		Name:  "John Doe",
		Email: "john@example.com",
	}

	// Create handler with Controller+State pattern
	handler := lt.Handle(controller, livetemplate.AsState(initialState))

	// Serve static files (for uploaded avatars)
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	// Serve client library
	http.HandleFunc("/livetemplate-client.js", lvttest.ServeClientLibrary)
	http.HandleFunc("/livetemplate.css", lvttest.ServeCSS)

	// Mount the LiveTemplate handler
	http.Handle("/", handler)

	// Start server
	addr := ":" + port
	log.Printf("Avatar upload example running at http://localhost%s", addr)
	log.Printf("Uploaded files will be saved to ./uploads/")

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
