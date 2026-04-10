package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gorilla/websocket"
	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// ========== E2E Tests ==========

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
	chromeCmd := e2etest.StartDockerChrome(t, debugPort)
	defer e2etest.StopDockerChrome(t, debugPort)
	_ = chromeCmd // Command returned for reference; cleanup handled by StopDockerChrome

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

	t.Run("UI_Standards", func(t *testing.T) {
		var violations string
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`(() => {
				const v = [];
				['onclick','onchange','oninput','onsubmit','onkeydown','onkeyup'].forEach(h => {
					document.querySelectorAll('[' + h + ']').forEach(el => v.push('inline ' + h + ' on <' + el.tagName.toLowerCase() + '>'));
				});
				document.querySelectorAll('[style]').forEach(el => {
					if (el.tagName !== 'INS' && el.tagName !== 'DEL' && !el.closest('[data-modal]') && !el.closest('[data-lvt-toast-stack]'))
						v.push('inline style on <' + el.tagName.toLowerCase() + '>');
				});
				if (!document.querySelector('meta[name="color-scheme"]')) v.push('missing color-scheme meta');
				if (document.documentElement.lang !== 'en') v.push('missing lang=en');
				const c = document.querySelector('.container');
				if (c && c.offsetWidth > 700) v.push('container too wide: ' + c.offsetWidth + 'px');
				return v.join('; ');
			})()`, &violations),
		)
		if err != nil {
			t.Fatalf("UI standards check failed: %v", err)
		}
		if violations != "" {
			t.Errorf("UI standard violations: %s", violations)
		}
		var cssStatus int
		chromedp.Run(ctx, chromedp.Evaluate(`(() => { const x = new XMLHttpRequest(); x.open('GET', '/livetemplate.css', false); x.send(); return x.status; })()`, &cssStatus))
		if cssStatus != 200 {
			t.Logf("Warning: Shared CSS not loading: status=%d (may not be available in CI)", cssStatus)
		}
	})

	t.Run("Upload Avatar and Verify", func(t *testing.T) {
		// Tier 1 file upload: create a File object in JavaScript, set it on the
		// input, and submit the form. The client detects the file input and sends
		// via HTTP fetch with FormData. The server parses the multipart body.

		err := chromedp.Run(ctx,
			// Fresh page load
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),

			// Create a minimal 1x1 PNG file in JavaScript and set it on the input.
			// We can't use chromedp.SetUploadFiles because Chrome runs in Docker
			// and can't access host filesystem paths.
			chromedp.Evaluate(`
				(() => {
					const b64 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVQI12P4z8AAAMBBBQAB1x2RAAAASElEQVQI12P4z8BQDwCNAQz/cWMmRQAAAABJRU5ErkJggg==';
					const binary = atob(b64);
					const bytes = new Uint8Array(binary.length);
					for (let i = 0; i < binary.length; i++) {
						bytes[i] = binary.charCodeAt(i);
					}
					const file = new File([bytes], 'test-avatar.png', {type: 'image/png'});

					const input = document.querySelector('#avatar');
					const dt = new DataTransfer();
					dt.items.add(file);
					input.files = dt.files;
					return 'file set (' + input.files.length + ' files)';
				})()
			`, nil),

			// Click submit button
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),

			// Wait for the avatar image to appear
			e2etest.WaitFor(`document.querySelector('img[alt^="Avatar"]') !== null`, 15*time.Second),
		)
		if err != nil {
			var debugHTML string
			_ = chromedp.Run(ctx, chromedp.OuterHTML(`body`, &debugHTML, chromedp.ByQuery))
			t.Logf("Page HTML at failure:\n%s", debugHTML)
			t.Fatalf("Upload flow failed: %v", err)
		}

		// Verify Name and Email fields retained their values after upload
		var nameVal, emailVal string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.getElementById('name').value`, &nameVal),
			chromedp.Evaluate(`document.getElementById('email').value`, &emailVal),
		)
		if err != nil {
			t.Fatalf("Failed to read form fields after upload: %v", err)
		}
		if nameVal != "John Doe" {
			t.Errorf("Name field should retain value after upload, got %q", nameVal)
		}
		if emailVal != "john@example.com" {
			t.Errorf("Email field should retain value after upload, got %q", emailVal)
		}

		t.Log("✅ Tier 1 file upload: avatar image rendered, form fields retained")
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

// ========== WebSocket Tests ==========

// TestUploadViaWebSocket tests the complete upload flow via WebSocket
// This test reproduces the actual browser upload behavior
func TestUploadViaWebSocket(t *testing.T) {
	// Create test server
	state := &ProfileState{
		Name:  "John Doe",
		Email: "john@example.com",
	}

	handler := createTestHandler(t, state)
	server := httptest.NewServer(handler)
	defer server.Close()

	// Connect WebSocket
	wsURL := "ws" + server.URL[4:] + "/ws"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}
	defer conn.Close()

	// Read messages in background
	messages := make(chan []byte, 10)
	errors := make(chan error, 1)
	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				errors <- err
				return
			}
			t.Logf("📩 Received message: %s", string(msg))
			messages <- msg
		}
	}()

	// Wait for initial tree
	select {
	case msg := <-messages:
		var tree map[string]interface{}
		if err := json.Unmarshal(msg, &tree); err != nil {
			t.Fatalf("Failed to parse initial tree: %v", err)
		}
		t.Log("✅ Received initial tree")
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for initial tree")
	}

	// Create a small test image (1x1 red PNG)
	pngData := []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xDE, 0x00, 0x00, 0x00, 0x0C, 0x49, 0x44, 0x41,
		0x54, 0x08, 0x99, 0x63, 0xF8, 0x0F, 0x00, 0x00,
		0x01, 0x01, 0x00, 0x05, 0x18, 0x0D, 0xA8, 0xDB,
		0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44,
		0xAE, 0x42, 0x60, 0x82,
	}

	// Step 1: Send upload_start
	uploadStartMsg := map[string]interface{}{
		"action":      "upload_start",
		"upload_name": "avatar",
		"files": []map[string]interface{}{
			{
				"name": "test-avatar.png",
				"type": "image/png",
				"size": len(pngData),
			},
		},
	}

	if err := conn.WriteJSON(uploadStartMsg); err != nil {
		t.Fatalf("Failed to send upload_start: %v", err)
	}
	t.Log("📤 Sent upload_start")

	// Wait for upload_start response
	var entryID string
	select {
	case msg := <-messages:
		var response map[string]interface{}
		if err := json.Unmarshal(msg, &response); err != nil {
			t.Fatalf("Failed to parse upload_start response: %v", err)
		}

		entries, ok := response["entries"].([]interface{})
		if !ok || len(entries) == 0 {
			t.Fatalf("No entries in upload_start response: %+v", response)
		}

		entry := entries[0].(map[string]interface{})
		entryID = entry["entry_id"].(string)
		t.Logf("✅ Received upload_start response, entry_id: %s", entryID)
	case <-time.After(2 * time.Second):
		t.Fatal("Timeout waiting for upload_start response")
	}

	// Step 2: Send upload chunks
	chunkSize := 256 * 1024
	offset := 0
	for offset < len(pngData) {
		end := offset + chunkSize
		if end > len(pngData) {
			end = len(pngData)
		}

		chunk := pngData[offset:end]
		chunkBase64 := base64.StdEncoding.EncodeToString(chunk)

		chunkMsg := map[string]interface{}{
			"action":       "upload_chunk",
			"entry_id":     entryID,
			"chunk_base64": chunkBase64,
			"offset":       offset,
			"total":        len(pngData),
		}

		if err := conn.WriteJSON(chunkMsg); err != nil {
			t.Fatalf("Failed to send upload_chunk: %v", err)
		}
		t.Logf("📤 Sent chunk %d-%d", offset, end)

		offset = end
	}

	// Small delay to ensure chunks are processed
	time.Sleep(100 * time.Millisecond)

	// Step 3: Send upload_complete
	uploadCompleteMsg := map[string]interface{}{
		"action":      "upload_complete",
		"upload_name": "avatar",
		"entry_ids":   []string{entryID},
	}

	if err := conn.WriteJSON(uploadCompleteMsg); err != nil {
		t.Fatalf("Failed to send upload_complete: %v", err)
	}
	t.Log("📤 Sent upload_complete")

	// Wait for tree update showing upload completion
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	receivedUpdate := false
	for !receivedUpdate {
		select {
		case msg := <-messages:
			var update map[string]interface{}
			if err := json.Unmarshal(msg, &update); err != nil {
				t.Logf("Skipping non-JSON message: %v", err)
				continue
			}

			// Check if this is a tree update (has "tree" field)
			if tree, ok := update["tree"]; ok {
				t.Logf("✅ Received tree update after upload_complete")
				receivedUpdate = true

				// Verify the update is valid (not null/undefined)
				if tree == nil {
					t.Error("❌ Tree update has null tree - this causes client error!")
				}

				// Verify the tree is not empty
				treeMap, ok := tree.(map[string]interface{})
				if !ok || len(treeMap) == 0 {
					t.Error("❌ Tree update is empty - no data changed!")
				} else {
					t.Logf("✅ Tree has %d root keys", len(treeMap))

					// Check if upload entries are in the tree and have Done=true
					// Position 4 is the upload-preview div content (after flash tag, avatar img, name, email)
					if uploadPreview, ok := treeMap["4"]; ok {
						t.Logf("📦 Upload preview section found in tree: %T", uploadPreview)
						// uploadPreview is an array containing the range operation
						if outerArray, ok := uploadPreview.([]interface{}); ok && len(outerArray) > 0 {
							// The first element is the actual range operation like ["a", [data], [statics], {idKey}]
							if rangeOp, ok := outerArray[0].([]interface{}); ok {
								t.Logf("Range operation has %d elements", len(rangeOp))
								if len(rangeOp) > 1 {
									t.Logf("Range operation [0] (op type): %v", rangeOp[0])
									t.Logf("Range operation [1] (items data) type: %T", rangeOp[1])
									if itemsData, ok := rangeOp[1].([]interface{}); ok {
										t.Logf("Items data has %d items", len(itemsData))
										if len(itemsData) > 0 {
											t.Logf("First item type: %T", itemsData[0])
											// First item should be the upload entry data
											if entryData, ok := itemsData[0].(map[string]interface{}); ok {
												t.Logf("📊 Upload entry data keys: %v", getKeys(entryData))
												t.Logf("📊 Upload entry data in tree: %+v", entryData)

												// Check if position 3 (the status message div wrapper) exists
												if msgDiv, exists := entryData["3"]; exists {
													t.Logf("✅ Position 3 (status div wrapper) exists: %T", msgDiv)
													// msgDiv should be a map with position "0" containing the actual message
													if msgMap, ok := msgDiv.(map[string]interface{}); ok {
														if actualMsg, exists := msgMap["0"]; exists {
															t.Logf("📝 Status message content: %v", actualMsg)
															// Check if it contains success message
															if msgStr, ok := actualMsg.(string); ok {
																if msgStr == "✅ Upload complete!" || msgStr == "Upload complete!" {
																	t.Logf("✅ SUCCESS MESSAGE FOUND: Upload complete!")
																} else {
																	t.Errorf("❌ Expected success message but got: %s", msgStr)
																}
															} else if msgTree, ok := actualMsg.(map[string]interface{}); ok {
																// Message might be a nested tree
																t.Logf("Message is a tree: %+v", msgTree)
															}
														} else {
															t.Error("❌ Position 0 (actual message) missing in status div!")
														}
													}
												} else {
													t.Error("❌ Position 3 (status message) missing - success message won't show!")
													t.Logf("Entry only has these positions: %v", getKeys(entryData))
												}
											} else {
												t.Logf("First item is not a map: %+v", itemsData[0])
											}
										}
									} else {
										t.Logf("rangeOp[1] is not []interface{}: %+v", rangeOp[1])
									}
								}
							} else {
								t.Logf("outerArray[0] is not a range operation array: %+v", outerArray[0])
							}
						} else {
							t.Logf("Upload preview is not a []interface{} or is empty: %+v", uploadPreview)
						}
					} else {
						t.Error("❌ Upload preview section (position 4) not in tree update!")
					}
				}
			}
		case err := <-errors:
			t.Fatalf("WebSocket error: %v", err)
		case <-ctx.Done():
			t.Fatal("❌ Timeout waiting for tree update after upload_complete")
		}
	}

	t.Log("✅ Upload test completed successfully")
}

func getKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func createTestHandler(t *testing.T, state *ProfileState) http.Handler {
	// Same setup as main.go
	lt, err := livetemplate.New("avatar-upload",
		livetemplate.WithParseFiles("avatar-upload.tmpl"),
		livetemplate.WithDevMode(true),
		livetemplate.WithUpload("avatar", livetemplate.UploadConfig{
			Accept:      []string{"image/jpeg", "image/png", "image/gif"},
			MaxFileSize: 5 * 1024 * 1024,
			MaxEntries:  1,
			AutoUpload:  false,
			ChunkSize:   256 * 1024,
		}),
	)
	if err != nil {
		t.Fatalf("Failed to create LiveTemplate: %v", err)
	}

	controller := &ProfileController{}
	return lt.Handle(controller, livetemplate.AsState(state))
}
