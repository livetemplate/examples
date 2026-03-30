package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	e2etest "github.com/livetemplate/lvt/testing"
)

func TestMain(m *testing.M) {
	e2etest.CleanupChromeContainers()
	code := m.Run()
	e2etest.CleanupChromeContainers()
	os.Exit(code)
}

func TestTodosProgressiveE2E(t *testing.T) {
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

	e2etest.StartTestServer(t, "main.go", serverPort)

	if err := e2etest.StartDockerChrome(t, debugPort); err != nil {
		t.Fatalf("Failed to start Docker Chrome: %v", err)
	}
	defer e2etest.StopDockerChrome(t, debugPort)

	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancel()

	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	err = chromedp.Run(ctx,
		chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
		e2etest.WaitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
	)
	if err != nil {
		t.Fatalf("Failed to load page: %v", err)
	}

	t.Run("InitialLoad", func(t *testing.T) {
		var html string
		var count int
		var hasDone bool
		err := chromedp.Run(ctx,
			chromedp.OuterHTML("body", &html, chromedp.ByQuery),
			chromedp.Evaluate(`document.querySelectorAll('tbody tr').length`, &count),
			chromedp.Evaluate(`document.querySelector('tbody tr:nth-child(3) s') !== null`, &hasDone),
		)
		if err != nil {
			t.Fatalf("Failed to get initial state: %v", err)
		}

		if !strings.Contains(html, "Todos (2 remaining)") {
			t.Errorf("Expected 'Todos (2 remaining)' heading")
		}
		if !strings.Contains(html, "Read the progressive complexity guide") {
			t.Error("First todo not found")
		}
		if !strings.Contains(html, "Try zero-attribute forms") {
			t.Error("Second todo not found")
		}
		if !strings.Contains(html, "Add lvt-* only when needed") {
			t.Error("Third todo not found")
		}
		if count != 3 {
			t.Errorf("Expected 3 items, got %d", count)
		}
		if !hasDone {
			t.Error("Third item should have 'done' class")
		}
	})

	t.Run("AddTodo", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.liveTemplateClient.send({action: 'submit', data: {Title: 'Buy groceries'}})`, nil),
			e2etest.WaitFor(`document.querySelectorAll('tbody tr').length === 4`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to add todo: %v", err)
		}

		var html string
		if err := chromedp.Run(ctx, chromedp.OuterHTML("body", &html, chromedp.ByQuery)); err != nil {
			t.Fatalf("Failed to get HTML: %v", err)
		}
		if !strings.Contains(html, "Buy groceries") {
			t.Error("New todo 'Buy groceries' not found")
		}
		if !strings.Contains(html, "3 remaining") {
			t.Errorf("Active count should be 3")
		}
	})

	t.Run("ToggleDone", func(t *testing.T) {
		var itemID string
		if err := chromedp.Run(ctx, chromedp.Evaluate(
			`document.querySelector('tbody tr:nth-child(1) input[name="id"]').value`, &itemID)); err != nil {
			t.Fatalf("Failed to get item ID: %v", err)
		}

		err := chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`window.liveTemplateClient.send({action: 'toggle', data: {id: '%s'}})`, itemID), nil),
			e2etest.WaitFor(`document.querySelector('tbody tr:nth-child(1) s') !== null`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to toggle done: %v", err)
		}

		var heading string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('h1').textContent`, &heading)); err != nil {
			t.Fatalf("Failed to get heading: %v", err)
		}
		if !strings.Contains(heading, "2 remaining") {
			t.Errorf("Expected '2 remaining', got %q", heading)
		}
	})

	t.Run("ToggleUndo", func(t *testing.T) {
		var itemID string
		if err := chromedp.Run(ctx, chromedp.Evaluate(
			`document.querySelector('tbody tr:nth-child(1) input[name="id"]').value`, &itemID)); err != nil {
			t.Fatalf("Failed to get item ID: %v", err)
		}

		err := chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`window.liveTemplateClient.send({action: 'toggle', data: {id: '%s'}})`, itemID), nil),
			e2etest.WaitFor(`document.querySelector('tbody tr:nth-child(1) s') === null`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to undo toggle: %v", err)
		}

		var heading string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelector('h1').textContent`, &heading)); err != nil {
			t.Fatalf("Failed to get heading: %v", err)
		}
		if !strings.Contains(heading, "3 remaining") {
			t.Errorf("Expected '3 remaining', got %q", heading)
		}
	})

	t.Run("DeleteTodo", func(t *testing.T) {
		var itemID string
		if err := chromedp.Run(ctx, chromedp.Evaluate(
			`document.querySelector('tbody tr:last-child input[name="id"]').value`, &itemID)); err != nil {
			t.Fatalf("Failed to get item ID: %v", err)
		}

		err := chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`window.liveTemplateClient.send({action: 'delete', data: {id: '%s'}})`, itemID), nil),
			e2etest.WaitFor(`document.querySelectorAll('tbody tr').length === 3`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to delete todo: %v", err)
		}

		var html string
		if err := chromedp.Run(ctx, chromedp.OuterHTML("body", &html, chromedp.ByQuery)); err != nil {
			t.Fatalf("Failed to get HTML: %v", err)
		}
		if strings.Contains(html, "Buy groceries") {
			t.Error("Deleted todo should not be present")
		}
	})

	t.Run("FilterActive", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.liveTemplateClient.send({action: 'filter', data: {filter: 'active'}})`, nil),
			e2etest.WaitFor(`document.querySelectorAll('tbody tr').length === 2`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to filter active: %v", err)
		}

		var html string
		if err := chromedp.Run(ctx, chromedp.OuterHTML("tbody", &html, chromedp.ByQuery)); err != nil {
			t.Fatalf("Failed to get HTML: %v", err)
		}
		if strings.Contains(html, "Add lvt-* only when needed") {
			t.Error("Done item should not be visible in active filter")
		}
	})

	t.Run("FilterDone", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.liveTemplateClient.send({action: 'filter', data: {filter: 'done'}})`, nil),
			e2etest.WaitFor(`document.querySelectorAll('tbody tr').length === 1`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to filter done: %v", err)
		}

		var html string
		if err := chromedp.Run(ctx, chromedp.OuterHTML("tbody", &html, chromedp.ByQuery)); err != nil {
			t.Fatalf("Failed to get HTML: %v", err)
		}
		if !strings.Contains(html, "Add lvt-* only when needed") {
			t.Error("Done item should be visible in done filter")
		}
	})

	t.Run("FilterAll", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.liveTemplateClient.send({action: 'filter', data: {filter: 'all'}})`, nil),
			e2etest.WaitFor(`document.querySelectorAll('tbody tr').length === 3`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to filter all: %v", err)
		}
	})

	t.Run("LiveTemplateWrapper", func(t *testing.T) {
		var html string
		if err := chromedp.Run(ctx, chromedp.OuterHTML(`[data-lvt-id]`, &html, chromedp.ByQuery)); err != nil {
			t.Fatalf("Failed to find LiveTemplate wrapper: %v", err)
		}
		if !strings.Contains(html, "data-lvt-id") {
			t.Error("LiveTemplate wrapper not preserved")
		}
	})
}
