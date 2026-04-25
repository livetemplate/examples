package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	e2etest "github.com/livetemplate/lvt/testing"
)

// TestCrossHandlerNavigation verifies that SPA navigation between different
// LiveTemplate handlers works correctly. Each pattern is a separate handler
// with its own data-lvt-id, so navigating between them requires the client
// to disconnect the old WebSocket and reconnect to the new handler.
func TestCrossHandlerNavigation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	baseURL := e2etest.GetChromeTestURL(serverPort)

	t.Run("Index_to_Pattern", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			// Start at the index page
			chromedp.Navigate(baseURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h2`, chromedp.ByQuery),

			// Click on "Edit Row" link
			chromedp.Click(`a[href="/patterns/forms/edit-row"]`, chromedp.ByQuery),

			// Wait for the Edit Row page content to appear
			e2etest.WaitForText(`h3`, "Edit Row", 10*time.Second),
			chromedp.OuterHTML(`body`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Navigation from index to Edit Row failed: %v", err)
		}
		if !strings.Contains(html, "Joe Smith") {
			t.Error("Edit Row content 'Joe Smith' not found after navigation")
		}
		if !strings.Contains(html, "Edit Row") {
			t.Error("Edit Row heading not found after navigation")
		}

		// Verify URL was updated
		var currentURL string
		chromedp.Run(ctx, chromedp.Location(&currentURL))
		if !strings.HasSuffix(currentURL, "/patterns/forms/edit-row") {
			t.Errorf("Expected URL ending in /patterns/forms/edit-row, got %s", currentURL)
		}
	})

	t.Run("Pattern_Back_to_Index", func(t *testing.T) {
		// We should already be on the Edit Row page from the previous test
		var html string
		err := chromedp.Run(ctx,
			// Click "Patterns" nav link to go back to index
			chromedp.Click(`nav a[href="/"]`, chromedp.ByQuery),

			// Wait for index page content
			e2etest.WaitForText(`h2`, "LiveTemplate Patterns", 10*time.Second),
			chromedp.OuterHTML(`body`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Navigation from pattern back to index failed: %v", err)
		}
		if !strings.Contains(html, "Forms &amp; Editing") {
			t.Error("Index page category 'Forms & Editing' not found")
		}

		// Verify URL was updated back to root
		var currentURL string
		chromedp.Run(ctx, chromedp.Location(&currentURL))
		if !strings.HasSuffix(currentURL, fmt.Sprintf(":%d/", serverPort)) {
			t.Errorf("Expected URL ending in :%d/, got %s", serverPort, currentURL)
		}
	})

	t.Run("Navigate_Between_Two_Patterns", func(t *testing.T) {
		// Start fresh at index
		err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h2`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load index: %v", err)
		}

		// Navigate to Click To Edit
		var html1 string
		err = chromedp.Run(ctx,
			chromedp.Click(`a[href="/patterns/forms/click-to-edit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`h3`, "Click To Edit", 10*time.Second),
			chromedp.OuterHTML(`article`, &html1, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Navigation to Click To Edit failed: %v", err)
		}
		if !strings.Contains(html1, "john@example.com") {
			t.Error("Click To Edit content not found")
		}

		// Navigate back to index via "Patterns" nav link
		err = chromedp.Run(ctx,
			chromedp.Click(`nav a[href="/"]`, chromedp.ByQuery),
			e2etest.WaitForText(`h2`, "LiveTemplate Patterns", 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Navigation back to index failed: %v", err)
		}

		// Now navigate to Bulk Update (different pattern)
		var html2 string
		err = chromedp.Run(ctx,
			chromedp.Click(`a[href="/patterns/forms/bulk-update"]`, chromedp.ByQuery),
			e2etest.WaitForText(`h3`, "Bulk Update", 10*time.Second),
			chromedp.OuterHTML(`article`, &html2, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Navigation to Bulk Update failed: %v", err)
		}
		if !strings.Contains(html2, "Joe Smith") {
			t.Error("Bulk Update content not found")
		}

		// Verify no stale content from Click To Edit
		if strings.Contains(html2, "john@example.com") {
			t.Error("Stale Click To Edit content still present on Bulk Update page")
		}
	})

	t.Run("Browser_Back_Button", func(t *testing.T) {
		// Start fresh at index
		err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h2`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load index: %v", err)
		}

		// Navigate forward to Click To Edit
		err = chromedp.Run(ctx,
			chromedp.Click(`a[href="/patterns/forms/click-to-edit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`h3`, "Click To Edit", 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Forward navigation failed: %v", err)
		}

		// Press browser Back button
		err = chromedp.Run(ctx,
			chromedp.NavigateBack(),
			e2etest.WaitForText(`h2`, "LiveTemplate Patterns", 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Back button navigation failed: %v", err)
		}

		// Verify we're on the index page
		var html string
		err = chromedp.Run(ctx,
			chromedp.OuterHTML(`body`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to read page after back: %v", err)
		}
		if !strings.Contains(html, "Forms &amp; Editing") {
			t.Error("Index page content not found after back button")
		}

		// Press browser Forward button
		err = chromedp.Run(ctx,
			chromedp.NavigateForward(),
			e2etest.WaitForText(`h3`, "Click To Edit", 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Forward button navigation failed: %v", err)
		}
	})

	t.Run("Title_Updates_On_Navigation", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to load index: %v", err)
		}

		// Verify index title
		var title string
		chromedp.Run(ctx, chromedp.Title(&title))
		if !strings.Contains(title, "LiveTemplate Patterns") {
			t.Errorf("Expected index title to contain 'LiveTemplate Patterns', got %q", title)
		}

		// Navigate to Click To Edit
		err = chromedp.Run(ctx,
			chromedp.Click(`a[href="/patterns/forms/click-to-edit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`h3`, "Click To Edit", 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Navigation failed: %v", err)
		}

		// Verify title updated
		chromedp.Run(ctx, chromedp.Title(&title))
		if !strings.Contains(title, "Click To Edit") {
			t.Errorf("Expected title to contain 'Click To Edit', got %q", title)
		}

		// Navigate back and verify title restores
		err = chromedp.Run(ctx,
			chromedp.NavigateBack(),
			e2etest.WaitForText(`h2`, "LiveTemplate Patterns", 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Back navigation failed: %v", err)
		}

		chromedp.Run(ctx, chromedp.Title(&title))
		if !strings.Contains(title, "LiveTemplate Patterns") {
			t.Errorf("Expected title to restore to 'LiveTemplate Patterns', got %q", title)
		}
	})

	t.Run("WebSocket_Works_After_Back_Button", func(t *testing.T) {
		// Navigate to Reset Input, then back, then forward, then test WebSocket
		err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.Click(`a[href="/patterns/forms/reset-input"]`, chromedp.ByQuery),
			e2etest.WaitForText(`h3`, "Reset User Input", 10*time.Second),
			e2etest.WaitForWebSocketReady(10*time.Second),
			// Go back
			chromedp.NavigateBack(),
			e2etest.WaitForText(`h2`, "LiveTemplate Patterns", 10*time.Second),
			// Go forward to Reset Input again
			chromedp.NavigateForward(),
			e2etest.WaitForText(`h3`, "Reset User Input", 10*time.Second),
			e2etest.WaitForWebSocketReady(10*time.Second),
		)
		if err != nil {
			t.Fatalf("Back/forward navigation failed: %v", err)
		}

		// Verify WebSocket works by submitting a form
		var html string
		err = chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="message"]`, "After back-forward", chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "After back-forward", 5*time.Second),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Form submission after back/forward failed: %v", err)
		}
		if !strings.Contains(html, "After back-forward") {
			t.Error("Message not found after back/forward navigation")
		}
	})

	t.Run("Index_To_Delete_Row_No_Stale_Dom", func(t *testing.T) {
		// Regression for cross-handler nav leaving stale index DOM: index's
		// 7 category <h4>s must NOT remain after clicking into Delete Row.
		// The later WaitFor on row count is load-bearing — it ensures the
		// WebSocket's initial tree update has been applied before we check,
		// so any treeState merge bug actually shows up.
		err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h2`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelectorAll('article').length >= 7`, 3*time.Second),
			chromedp.Click(`a[href="/patterns/lists/delete-row"]`, chromedp.ByQuery),
			e2etest.WaitForText(`h3`, "Delete Row", 10*time.Second),
			e2etest.WaitFor(`document.querySelectorAll('tbody tr[data-key]').length === 5`, 5*time.Second),
			e2etest.WaitForWebSocketReady(10*time.Second),
		)
		if err != nil {
			var articleCount, rowCount, leakedCategoryHeaders int
			var wrapperHTML string
			_ = chromedp.Run(ctx,
				chromedp.Evaluate(`document.querySelectorAll('[data-lvt-id] article').length`, &articleCount),
				chromedp.Evaluate(`document.querySelectorAll('tbody tr[data-key]').length`, &rowCount),
				chromedp.Evaluate(`document.querySelectorAll('[data-lvt-id] h4').length`, &leakedCategoryHeaders),
				chromedp.OuterHTML(`[data-lvt-id]`, &wrapperHTML, chromedp.ByQuery),
			)
			t.Fatalf("Navigation bug: articles=%d rows=%d leakedH4=%d. Wrapper HTML:\n%s\nError: %v",
				articleCount, rowCount, leakedCategoryHeaders, wrapperHTML, err)
		}
		// Assert: exactly 1 article, exactly 5 rows, 0 leaked <h4> headers.
		var articleCount, leakedCategoryHeaders int
		_ = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('[data-lvt-id] article').length`, &articleCount),
			chromedp.Evaluate(`document.querySelectorAll('[data-lvt-id] h4').length`, &leakedCategoryHeaders),
		)
		if leakedCategoryHeaders > 0 {
			t.Errorf("Stale <h4> category headers leaked from index: %d present", leakedCategoryHeaders)
		}
		if articleCount != 1 {
			t.Errorf("Expected exactly 1 <article>, got %d", articleCount)
		}
	})

	t.Run("Index_To_Modal_Dialog_No_Stale_Dom", func(t *testing.T) {
		// Regression: navigating from index to a Session 4 navigation pattern
		// must not leak any index-page category names ("Forms & Editing",
		// "Lists & Data", etc.) into the new pattern's wrapper. The category
		// strings are specific enough that any presence is a regression
		// signal — Modal Dialog renders its own h4 ("Edit profile") inside
		// the dialog, so a raw h4-count check would false-positive.
		err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h2`, chromedp.ByQuery),
			chromedp.Click(`a[href="/patterns/navigation/modal-dialog"]`, chromedp.ByQuery),
			e2etest.WaitForText(`h3`, "Modal Dialog", 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Index → Modal Dialog navigation failed: %v", err)
		}
		var leakedText string
		_ = chromedp.Run(ctx, chromedp.Evaluate(`(() => {
			const wrapper = document.querySelector('[data-lvt-id]');
			if (!wrapper) return "no-wrapper";
			const text = wrapper.textContent || "";
			const leaks = ["Forms & Editing", "Lists & Data", "Search & Filtering", "Loading & Progress", "Visual Feedback", "Real-Time"];
			return leaks.find(s => text.includes(s)) || "";
		})()`, &leakedText))
		if leakedText != "" {
			t.Errorf("Stale index category text %q leaked into Modal Dialog wrapper", leakedText)
		}
	})

	t.Run("Tabs_Same_Pathname_Uses_WebSocket", func(t *testing.T) {
		// On the Tabs pattern, clicking a tab link is a same-pathname
		// query-string change. The client routes it via WebSocket
		// (__navigate__) instead of an HTTP fetch — verify by overriding
		// window.fetch and counting hits during the click.
		err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL+"/patterns/navigation/tabs"),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`a[href="?tab=overview"][aria-current="page"]`, chromedp.ByQuery),
			chromedp.Evaluate(`(() => {
				window.__crossNavHits = 0;
				window.__origCrossFetch = window.fetch;
				window.fetch = function(input, init) {
					try {
						const u = typeof input === 'string' ? input : input.url;
						if (u && u.includes('/patterns/navigation/tabs')) window.__crossNavHits++;
					} catch (e) {}
					return window.__origCrossFetch.apply(window, arguments);
				};
			})()`, nil),
			chromedp.Click(`a[href="?tab=settings"]`, chromedp.ByQuery),
			e2etest.WaitFor(`!!document.querySelector('a[href="?tab=settings"][aria-current="page"]')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Tab switch failed: %v", err)
		}
		var hits int
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.__crossNavHits`, &hits)); err != nil {
			t.Fatalf("HTTP hit count read failed: %v", err)
		}
		_ = chromedp.Run(ctx, chromedp.Evaluate(`(() => { window.fetch = window.__origCrossFetch; })()`, nil))
		if hits != 0 {
			t.Errorf("Tab switch must use WebSocket __navigate__, got %d HTTP hits", hits)
		}
	})

	t.Run("SPA_Navigation_Cross_Pathname_Reconnects", func(t *testing.T) {
		// From the SPA Navigation page, clicking a cross-pathname link to
		// another pattern must change the wrapper's data-lvt-id (the new
		// handler is a separate handler, so it gets a fresh wrapper id) and
		// surface the new pattern's content.
		err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL+"/patterns/navigation/spa-navigation"),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`a[href="/patterns/navigation/tabs"]`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load SPA Navigation page: %v", err)
		}
		var oldID, newID string
		err = chromedp.Run(ctx,
			chromedp.AttributeValue(`[data-lvt-id]`, "data-lvt-id", &oldID, nil, chromedp.ByQuery),
			chromedp.Click(`a[href="/patterns/navigation/tabs"]`, chromedp.ByQuery),
			e2etest.WaitForText(`h3`, "Tabs (HATEOAS)", 10*time.Second),
			e2etest.WaitForWebSocketReady(10*time.Second),
			chromedp.AttributeValue(`[data-lvt-id]`, "data-lvt-id", &newID, nil, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Cross-pathname click + reconnect failed: %v", err)
		}
		if oldID == "" || newID == "" {
			t.Errorf("data-lvt-id values should be present (old=%q, new=%q)", oldID, newID)
		}
		if oldID == newID {
			t.Errorf("Cross-pathname navigation must reconnect with a fresh wrapper id; got same id %q before and after", oldID)
		}
	})

	t.Run("WebSocket_Works_After_Navigation", func(t *testing.T) {
		// Start fresh at index
		err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h2`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load index: %v", err)
		}

		// Navigate to Reset Input
		err = chromedp.Run(ctx,
			chromedp.Click(`a[href="/patterns/forms/reset-input"]`, chromedp.ByQuery),
			e2etest.WaitForText(`h3`, "Reset User Input", 10*time.Second),
			// Wait for WebSocket to reconnect on the new handler
			e2etest.WaitForWebSocketReady(10*time.Second),
		)
		if err != nil {
			t.Fatalf("Navigation to Reset Input failed: %v", err)
		}

		// Submit a form — this requires a working WebSocket connection
		var html string
		err = chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="message"]`, "Cross-handler test", chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Cross-handler test", 5*time.Second),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Form submission after cross-handler nav failed: %v", err)
		}
		if !strings.Contains(html, "Cross-handler test") {
			t.Error("Submitted message not found — WebSocket not reconnected")
		}
	})
}
