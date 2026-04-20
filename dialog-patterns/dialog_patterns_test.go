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

func TestDialogPatternsE2E(t *testing.T) {
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

	t.Run("Initial_Load", func(t *testing.T) {
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

		if !strings.Contains(html, "Dialog Patterns") {
			t.Error("Page title not found")
		}
		if !strings.Contains(html, "Learn LiveTemplate") {
			t.Error("Seed item 'Learn LiveTemplate' not found")
		}
		if !strings.Contains(html, "Build a dialog example") {
			t.Error("Seed item 'Build a dialog example' not found")
		}
		if !strings.Contains(html, "Write E2E tests") {
			t.Error("Seed item 'Write E2E tests' not found")
		}
		if !strings.Contains(html, "3 items") {
			t.Error("Item count '3 items' not found")
		}

		// Dialog should be closed initially
		var dialogOpen bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.getElementById('add-dialog').open`, &dialogOpen),
		)
		if err != nil {
			t.Fatalf("Failed to check dialog state: %v", err)
		}
		if dialogOpen {
			t.Error("Dialog should be closed initially")
		}

		t.Log("✅ Initial page load verified with 3 seed items and closed dialog")
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
		t.Log("✅ UI standards passed")
	})

	t.Run("Open_Dialog", func(t *testing.T) {
		// Click the "Add Item" button which uses command="show-modal" commandfor="add-dialog"
		err := chromedp.Run(ctx,
			chromedp.Click(`button[commandfor="add-dialog"][command="show-modal"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.getElementById('add-dialog').open === true`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to open dialog: %v", err)
		}

		// Verify dialog is visible
		var dialogOpen bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.getElementById('add-dialog').open`, &dialogOpen),
		)
		if err != nil {
			t.Fatalf("Failed to check dialog state: %v", err)
		}
		if !dialogOpen {
			t.Error("Dialog should be open after clicking show-modal button")
		}

		t.Log("✅ Dialog opened via command='show-modal' polyfill")
	})

	t.Run("Close_Dialog_Cancel", func(t *testing.T) {
		// Click the cancel button which uses command="close" commandfor="add-dialog"
		err := chromedp.Run(ctx,
			chromedp.Click(`button[commandfor="add-dialog"][command="close"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.getElementById('add-dialog').open === false`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to close dialog: %v", err)
		}

		var dialogOpen bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.getElementById('add-dialog').open`, &dialogOpen),
		)
		if err != nil {
			t.Fatalf("Failed to check dialog state: %v", err)
		}
		if dialogOpen {
			t.Error("Dialog should be closed after clicking close button")
		}

		// Verify items unchanged
		var html string
		err = chromedp.Run(ctx, chromedp.OuterHTML(`body`, &html, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("Failed to get HTML: %v", err)
		}
		if !strings.Contains(html, "3 items") {
			t.Error("Items should be unchanged after cancel")
		}

		t.Log("✅ Dialog closed via command='close' polyfill, items unchanged")
	})

	t.Run("Add_Item_Via_Dialog", func(t *testing.T) {
		// Open the dialog
		err := chromedp.Run(ctx,
			chromedp.Click(`button[commandfor="add-dialog"][command="show-modal"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.getElementById('add-dialog').open === true`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to open dialog: %v", err)
		}

		// Fill in the title and submit
		err = chromedp.Run(ctx,
			chromedp.Clear(`dialog#add-dialog input[name="title"]`, chromedp.ByQuery),
			chromedp.SendKeys(`dialog#add-dialog input[name="title"]`, "New Test Item", chromedp.ByQuery),
			chromedp.Click(`dialog#add-dialog button[type="submit"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.body.innerText.includes('New Test Item')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to add item: %v", err)
		}

		// Verify all items exist
		var html string
		err = chromedp.Run(ctx, chromedp.OuterHTML(`body`, &html, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("Failed to get HTML: %v", err)
		}
		if !strings.Contains(html, "Learn LiveTemplate") {
			t.Error("Seed item 'Learn LiveTemplate' missing after add")
		}
		if !strings.Contains(html, "Build a dialog example") {
			t.Error("Seed item 'Build a dialog example' missing after add")
		}
		if !strings.Contains(html, "Write E2E tests") {
			t.Error("Seed item 'Write E2E tests' missing after add")
		}
		if !strings.Contains(html, "New Test Item") {
			t.Error("New item 'New Test Item' not found")
		}
		if !strings.Contains(html, "4 items") {
			t.Error("Item count should be '4 items'")
		}

		// Dialog should be closed after form submission
		var dialogOpen bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.getElementById('add-dialog').open`, &dialogOpen),
		)
		if err != nil {
			t.Fatalf("Failed to check dialog state: %v", err)
		}
		if dialogOpen {
			t.Error("Dialog should be closed after form submission")
		}

		// Form should be reset
		var inputValue string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('dialog#add-dialog input[name="title"]').value`, &inputValue),
		)
		if err != nil {
			t.Fatalf("Failed to get input value: %v", err)
		}
		if inputValue != "" {
			t.Errorf("Input should be reset after submission, got: %q", inputValue)
		}

		t.Log("✅ Item added via dialog, dialog closed, form reset")
	})

	t.Run("Add_Empty_Title_Error", func(t *testing.T) {
		// Navigate fresh to get clean polyfill state (command/commandfor listeners
		// are lost after WebSocket DOM updates from the previous test)
		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to reload page: %v", err)
		}

		// Open the dialog
		err = chromedp.Run(ctx,
			chromedp.Click(`button[commandfor="add-dialog"][command="show-modal"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.getElementById('add-dialog').open === true`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to open dialog: %v", err)
		}

		// Remove `required` via JS so the empty form reaches the server for validation.
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('dialog#add-dialog input[name="title"]').removeAttribute('required')`, nil),
			chromedp.Clear(`dialog#add-dialog input[name="title"]`, chromedp.ByQuery),
			chromedp.Click(`dialog#add-dialog button[type="submit"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('dialog#add-dialog small') !== null`, 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to submit empty form or validation error not shown: %v", err)
		}

		// Dialog should stay open while showing validation errors
		var dialogOpen bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.getElementById('add-dialog').open`, &dialogOpen),
		)
		if err != nil {
			t.Fatalf("Failed to check dialog state: %v", err)
		}
		if !dialogOpen {
			t.Error("Dialog should remain open when showing validation errors")
		}

		// Validation error <small> tag should be visible inside the dialog
		var errorText string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('dialog#add-dialog small').textContent`, &errorText),
		)
		if err != nil {
			t.Fatalf("Failed to read error text: %v", err)
		}
		if errorText == "" {
			t.Error("Validation error message should not be empty")
		}
		t.Logf("Validation error shown: %q", errorText)

		// Input should have aria-invalid="true"
		var ariaInvalid string
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('dialog#add-dialog input[name="title"]').getAttribute('aria-invalid')`, &ariaInvalid),
		)
		if err != nil {
			t.Fatalf("Failed to check aria-invalid: %v", err)
		}
		if ariaInvalid != "true" {
			t.Errorf("Input should have aria-invalid='true', got %q", ariaInvalid)
		}

		// Item count should remain 3 (unchanged — fresh session has seed items)
		var html string
		err = chromedp.Run(ctx, chromedp.OuterHTML(`body`, &html, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("Failed to get HTML: %v", err)
		}
		if !strings.Contains(html, "3 items") {
			t.Error("Item count should still be '3 items' after failed empty submission")
		}

		t.Log("✅ Validation errors shown inside open dialog, item count unchanged")
	})

	t.Run("Delete_Item", func(t *testing.T) {
		// Navigate fresh to get clean state
		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`table`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to reload page: %v", err)
		}

		// Delete the first seed item (Learn LiveTemplate)
		err = chromedp.Run(ctx,
			chromedp.Click(`button[name="delete"][value="1"]`, chromedp.ByQuery),
			e2etest.WaitFor(`!document.body.innerText.includes('Learn LiveTemplate')`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to delete item: %v", err)
		}

		var html string
		err = chromedp.Run(ctx, chromedp.OuterHTML(`body`, &html, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("Failed to get HTML: %v", err)
		}

		if strings.Contains(html, "Learn LiveTemplate") {
			t.Error("Deleted item 'Learn LiveTemplate' should not be present")
		}
		if !strings.Contains(html, "Build a dialog example") {
			t.Error("Remaining item 'Build a dialog example' should be present")
		}
		if !strings.Contains(html, "Write E2E tests") {
			t.Error("Remaining item 'Write E2E tests' should be present")
		}
		if !strings.Contains(html, "2 items") {
			t.Error("Item count should be '2 items' after deletion")
		}

		t.Log("✅ Item deleted, remaining items preserved")
	})

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎉 All dialog-patterns E2E tests passed!")
	fmt.Println(strings.Repeat("=", 60))
}
