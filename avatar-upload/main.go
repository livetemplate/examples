package main

import (
	"context"
	"embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/livetemplate/livetemplate"
)

//go:embed *.tmpl
var templates embed.FS

// ProfileStore manages user profile with avatar upload
type ProfileStore struct {
	Name       string
	Email      string
	AvatarPath string
	AvatarURL  string
}

// AllowUploads configures avatar upload
func (s *ProfileStore) AllowUploads() map[string]livetemplate.UploadConfig {
	return map[string]livetemplate.UploadConfig{
		"avatar": {
			Accept:      []string{"image/jpeg", "image/png", "image/gif"},
			MaxFileSize: 5 * 1024 * 1024, // 5MB
			MaxEntries:  1,                // Single file
			AutoUpload:  false,            // Manual upload on form submit
			ChunkSize:   256 * 1024,       // 256KB chunks
		},
	}
}

// ConsumeUpload processes uploaded avatar
func (s *ProfileStore) ConsumeUpload(ctx context.Context, name string, entries []*livetemplate.UploadEntry) error {
	if name != "avatar" {
		return nil
	}

	// Create uploads directory if it doesn't exist
	uploadsDir := "uploads"
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return fmt.Errorf("failed to create uploads directory: %w", err)
	}

	for _, entry := range entries {
		// Generate permanent filename
		ext := filepath.Ext(entry.ClientName)
		permanentPath := filepath.Join(uploadsDir, fmt.Sprintf("avatar-%s%s", entry.ID, ext))

		// Move from temp to permanent location
		if err := os.Rename(entry.TempPath, permanentPath); err != nil {
			// If rename fails (different filesystem), try copy
			if err := copyFile(entry.TempPath, permanentPath); err != nil {
				return fmt.Errorf("failed to save avatar: %w", err)
			}
			os.Remove(entry.TempPath) // Clean up temp file
		}

		// Update store with new avatar
		s.AvatarPath = permanentPath
		s.AvatarURL = "/" + permanentPath

		log.Printf("Avatar saved: %s (original: %s, size: %d bytes)", permanentPath, entry.ClientName, entry.ClientSize)
	}

	return nil
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// UpdateProfile handles profile update form submission
func (s *ProfileStore) UpdateProfile(ctx context.Context, data livetemplate.ActionData) error {
	name, _ := data.String("name")
	email, _ := data.String("email")

	s.Name = name
	s.Email = email

	log.Printf("Profile updated: name=%s, email=%s", s.Name, s.Email)
	return nil
}

func main() {
	// Parse port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Create LiveTemplate instance
	lt := livetemplate.Must(livetemplate.New("avatar-upload",
		livetemplate.WithTemplateFS(templates),
		livetemplate.WithDevMode(true),
	)

	// Create initial store
	store := &ProfileStore{
		Name:  "John Doe",
		Email: "john@example.com",
	}

	// Create handler with upload support
	handler := lt.NewHandler(store)

	// Serve static files (for uploaded avatars)
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	// Mount the LiveTemplate handler
	http.Handle("/", handler)

	// Start server
	addr := ":" + port
	log.Printf("🚀 Avatar upload example running at http://localhost%s", addr)
	log.Printf("📸 Upload an avatar to see the upload feature in action!")
	log.Printf("📁 Uploaded files will be saved to ./uploads/")

	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatal(err)
	}
}
