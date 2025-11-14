package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	e2etest "github.com/livetemplate/lvt/testing"
)

// TestAvatarUploadE2E tests the avatar upload app end-to-end with a real browser
func TestAvatarUploadE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	// Get free ports for server and Chrome debugging
	serverPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for server: %v", err)
	}

	debugPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for Chrome: %v", err)
	}

	// Start avatar-upload server
	serverCmd := e2etest.StartTestServer(t, "main.go", serverPort)
	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Kill()
		}
	}()

	// Start Docker Chrome container
	_ = e2etest.StartDockerChrome(t, debugPort)
	defer e2etest.StopDockerChrome(t, debugPort)

	// Connect to Docker Chrome via remote debugging
	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancel()

	// Set timeout for the entire test
	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	t.Run("Initial Load", func(t *testing.T) {
		var initialHTML string

		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second), // Wait for WebSocket init
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"), // Validate no raw template expressions
			chromedp.OuterHTML(`body`, &initialHTML, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}

		// Verify initial state
		if !strings.Contains(initialHTML, "Profile Settings") {
			t.Error("Page title not found")
		}
		if !strings.Contains(initialHTML, "John Doe") {
			t.Error("Initial name not found")
		}
		if !strings.Contains(initialHTML, "john@example.com") {
			t.Error("Initial email not found")
		}

		t.Log("✅ Initial page load verified")
	})

	t.Run("Upload Avatar - Live Update", func(t *testing.T) {
		// Create a test image file
		testImagePath, err := createTestImage(t)
		if err != nil {
			t.Fatalf("Failed to create test image: %v", err)
		}
		defer os.Remove(testImagePath)

		var successText string

		err = chromedp.Run(ctx,
			// Upload the file
			chromedp.SetUploadFiles(`input[type="file"]`, []string{testImagePath}, chromedp.ByQuery),

			// Wait for the "Upload complete!" message to appear LIVE (not after reload!)
			// This is the key test - it should appear immediately via WebSocket update
			// Increased timeout for file processing
			chromedp.WaitVisible(`.success`, chromedp.ByQuery),

			// Get the success message text to verify it's correct
			chromedp.TextContent(`.success`, &successText, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to upload and verify: %v", err)
		}

		// Verify the success message text
		if !strings.Contains(successText, "Upload complete") {
			t.Errorf("Expected success message to contain 'Upload complete', got: %s", successText)
		}

		t.Log("✅ Upload live update verified - message appeared without page reload")
	})

	t.Run("Upload Progress Display", func(t *testing.T) {
		// Create another test image
		testImagePath, err := createTestImage(t)
		if err != nil {
			t.Fatalf("Failed to create test image: %v", err)
		}
		defer os.Remove(testImagePath)

		var progressHTML string

		err = chromedp.Run(ctx,
			// Clear any previous uploads by reloading
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(3*time.Second),

			// Upload the file
			chromedp.SetUploadFiles(`input[type="file"]`, []string{testImagePath}, chromedp.ByQuery),

			// Wait for upload to complete
			chromedp.WaitVisible(`.success`, chromedp.ByQuery),

			// Get the upload preview HTML
			chromedp.OuterHTML(`.upload-preview`, &progressHTML, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to check progress display: %v", err)
		}

		// Verify progress elements are present
		if !strings.Contains(progressHTML, "upload-entry") {
			t.Error("Upload entry not found")
		}
		if !strings.Contains(progressHTML, "100%") {
			t.Error("100% progress not found")
		}
		if !strings.Contains(progressHTML, "✅ Upload complete!") {
			t.Error("Success message not found in HTML")
		}

		t.Log("✅ Upload progress display verified")
	})

	t.Run("WebSocket Connection", func(t *testing.T) {
		// Verify WebSocket client is initialized
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`console.log('WebSocket test'); 'logged'`, nil),
			e2etest.WaitFor(`typeof LiveTemplateClient !== 'undefined'`, 3*time.Second),
		)

		if err != nil {
			t.Fatalf("Failed to check WebSocket: %v", err)
		}

		t.Log("✅ WebSocket connection working")
	})
}

// createTestImage creates a small test PNG image file
func createTestImage(t *testing.T) (string, error) {
	// Simple 1x1 PNG (red pixel)
	// PNG signature + IHDR + IDAT + IEND
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0x99, 0x63, 0xF8, 0x0F, 0x00, 0x00,
		0x01, 0x01, 0x00, 0x05, 0x18, 0x0D, 0xA8, 0xDB,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, // IEND chunk
		0xAE, 0x42, 0x60, 0x82,
	}

	tmpFile, err := os.CreateTemp("", "test-avatar-*.png")
	if err != nil {
		return "", err
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(pngData); err != nil {
		return "", err
	}

	absPath, err := filepath.Abs(tmpFile.Name())
	if err != nil {
		return "", err
	}

	return absPath, nil
}
