package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/gorilla/websocket"
	e2etest "github.com/livetemplate/lvt/testing"
)

func TestMain(m *testing.M) {
	e2etest.CleanupChromeContainers()
	code := m.Run()
	e2etest.CleanupChromeContainers()
	os.Exit(code)
}

// startServer starts the live-preview server on a free port and registers
// cleanup via t.Cleanup. Returns the port and a log buffer for debugging.
func startServer(t *testing.T) (int, *e2etest.SafeBuffer) {
	t.Helper()

	port, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port: %v", err)
	}

	portStr := fmt.Sprintf("%d", port)
	cmd := exec.Command("go", "run", "main.go")
	cmd.Env = append([]string{"PORT=" + portStr}, cmd.Environ()...)

	logs := e2etest.NewSafeBuffer()
	cmd.Stdout = logs
	cmd.Stderr = logs

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	t.Cleanup(func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
		t.Logf("=== SERVER LOGS ===\n%s", logs.String())
	})

	serverURL := fmt.Sprintf("http://localhost:%s", portStr)
	for i := 0; i < 50; i++ {
		if resp, err := http.Get(serverURL); err == nil {
			resp.Body.Close()
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	return port, logs
}

// dialWebSocket connects to the server's WebSocket endpoint and registers cleanup.
func dialWebSocket(t *testing.T, port int) *websocket.Conn {
	t.Helper()

	wsURL := fmt.Sprintf("ws://localhost:%d/", port)
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("Failed to connect WebSocket: %v", err)
	}

	t.Cleanup(func() {
		if err := conn.Close(); err != nil {
			t.Logf("WebSocket close error: %v", err)
		}
	})

	return conn
}

// TestWebSocketCapabilities verifies the initial WebSocket render includes
// "change" in the capabilities metadata.
func TestWebSocketCapabilities(t *testing.T) {
	port, _ := startServer(t)
	conn := dialWebSocket(t, port)

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	t.Logf("Initial message: %s", msg)

	var response map[string]interface{}
	if err := json.Unmarshal(msg, &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	meta, ok := response["meta"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected meta field in initial render")
	}

	caps, ok := meta["capabilities"].([]interface{})
	if !ok {
		// Capabilities field requires livetemplate/livetemplate#253.
		// Skip gracefully when running against a release without it.
		t.Skip("capabilities not present in meta — requires livetemplate#253")
	}

	if len(caps) != 1 || caps[0] != "change" {
		t.Errorf("Expected capabilities=[\"change\"], got %v", caps)
	}
}

// TestWebSocketChangeAction verifies the Change() method works via WebSocket.
func TestWebSocketChangeAction(t *testing.T) {
	port, _ := startServer(t)
	conn := dialWebSocket(t, port)

	// Discard initial render
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("Failed to read initial message: %v", err)
	}

	action := []byte(`{"action":"change","data":{"Name":"World"}}`)
	if err := conn.WriteMessage(websocket.TextMessage, action); err != nil {
		t.Fatalf("Failed to send change action: %v", err)
	}

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("Failed to read response: %v", err)
	}

	t.Logf("Change response: %s", msg)

	if !strings.Contains(string(msg), "Hello, World!") {
		t.Errorf("Expected response to contain 'Hello, World!', got: %s", msg)
	}
}

// TestLivePreviewE2E tests the live preview app end-to-end with a real browser.
func TestLivePreviewE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	serverPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for server: %v", err)
	}

	debugPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for Chrome: %v", err)
	}

	serverCmd := e2etest.StartTestServer(t, "main.go", serverPort)
	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Kill()
		}
	}()

	chromeCmd := e2etest.StartDockerChrome(t, debugPort)
	defer e2etest.StopDockerChrome(t, debugPort)
	_ = chromeCmd

	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	t.Run("Initial Load", func(t *testing.T) {
		var html string

		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`body`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}

		if !strings.Contains(html, "Live Preview") {
			t.Error("Page title not found")
		}
		if !strings.Contains(html, "Hello, !") {
			t.Error("Initial preview not found")
		}
	})

	t.Run("WebSocket Connection", func(t *testing.T) {
		err := chromedp.Run(ctx,
			e2etest.WaitFor(`typeof window.liveTemplateClient !== 'undefined'`, 3*time.Second),
		)
		if err != nil {
			t.Fatalf("WebSocket client not initialized: %v", err)
		}
	})

	t.Run("LiveTemplate Wrapper", func(t *testing.T) {
		var html string

		err := chromedp.Run(ctx,
			chromedp.OuterHTML(`[data-lvt-id]`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to find LiveTemplate wrapper: %v", err)
		}

		if !strings.Contains(html, "data-lvt-id") {
			t.Error("LiveTemplate wrapper not found")
		}
	})
}
