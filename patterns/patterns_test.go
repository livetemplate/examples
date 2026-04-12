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

// setupTest starts the server and Docker Chrome, returning the chromedp context
// and the server port. Cleanup is handled via t.Cleanup.
func setupTest(t *testing.T) (context.Context, context.CancelFunc, int) {
	t.Helper()

	serverPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for server: %v", err)
	}

	debugPort, err := e2etest.GetFreePort()
	if err != nil {
		t.Fatalf("Failed to get free port for Chrome: %v", err)
	}

	serverCmd := e2etest.StartTestServer(t, ".", serverPort)
	t.Cleanup(func() {
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Kill()
		}
	})

	chromeCmd := e2etest.StartDockerChrome(t, debugPort)
	_ = chromeCmd
	t.Cleanup(func() {
		e2etest.StopDockerChrome(t, debugPort)
	})

	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	t.Cleanup(allocCancel)

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	t.Cleanup(cancel)

	ctx, cancel = context.WithTimeout(ctx, 120*time.Second)

	return ctx, cancel, serverPort
}

// uiStandardsJS is the JavaScript snippet for UI standards validation.
const uiStandardsJS = `(() => {
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
})()`

// runUIStandards validates CSP compliance and meta tags.
func runUIStandards(t *testing.T, ctx context.Context) {
	t.Helper()
	var violations string
	err := chromedp.Run(ctx,
		chromedp.Evaluate(uiStandardsJS, &violations),
	)
	if err != nil {
		t.Fatalf("UI standards check failed: %v", err)
	}
	if violations != "" {
		t.Errorf("UI standard violations: %s", violations)
	}
}

// runUIStandardsWithPico validates CSP compliance, meta tags, AND Pico CSS
// conventions (input+button must be inside fieldset[role="group"]).
// Use this for pages with inline forms. For pages with vertical labeled forms,
// use runUIStandards (the fieldset[role="group"] rule doesn't apply to vertical forms).
func runUIStandardsWithPico(t *testing.T, ctx context.Context) {
	t.Helper()
	runUIStandards(t, ctx)
	if err := chromedp.Run(ctx, e2etest.ValidatePicoCSS()); err != nil {
		t.Errorf("Pico CSS check failed: %v", err)
	}
}

// attachFileViaDataTransfer sets a File on the given file input using the
// DataTransfer API. chromedp.SetUploadFiles cannot be used with Docker
// Chrome because the container has no access to host filesystem paths.
func attachFileViaDataTransfer(inputSelector, filename, content, mimeType string) chromedp.Action {
	script := fmt.Sprintf(`
		(() => {
			const file = new File([%q], %q, {type: %q});
			const input = document.querySelector(%q);
			const dt = new DataTransfer();
			dt.items.add(file);
			input.files = dt.files;
		})()
	`, content, filename, mimeType, inputSelector)
	return chromedp.Evaluate(script, nil)
}

// --- Index Page ---

func TestIndexPage(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	t.Run("Initial_Load", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h2`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`body`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		if !strings.Contains(html, "LiveTemplate Patterns") {
			t.Error("Page title not found")
		}
		if !strings.Contains(html, "Forms &amp; Editing") {
			t.Error("Forms & Editing category not found")
		}
	})

	t.Run("UI_Standards", func(t *testing.T) {
		runUIStandardsWithPico(t, ctx)
	})

	t.Run("Visual_Check", func(t *testing.T) {
		e2etest.ValidateScreenshotWithLLM(t, ctx, "Pattern index page — heading, 7 category cards with pattern links and descriptions")
	})

	t.Run("Pattern_Links", func(t *testing.T) {
		var count int
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('a[href^="/patterns/forms/"]').length`, &count),
		)
		if err != nil {
			t.Fatalf("Failed to count pattern links: %v", err)
		}
		if count != 7 {
			t.Errorf("Expected 7 Forms pattern links, got %d", count)
		}
	})
}

// --- Pattern #1: Click To Edit ---

func TestClickToEdit(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/forms/click-to-edit"

	t.Run("Initial_Load", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h3`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		if !strings.Contains(html, "John") {
			t.Error("Default first name 'John' not found")
		}
		if !strings.Contains(html, "john@example.com") {
			t.Error("Default email not found")
		}
		// Should be in view mode — table should be present, form should not
		if !strings.Contains(html, "<table>") {
			t.Error("View mode table not found")
		}
	})

	t.Run("UI_Standards", func(t *testing.T) {
		runUIStandardsWithPico(t, ctx)
	})

	t.Run("Visual_Check", func(t *testing.T) {
		e2etest.ValidateScreenshotWithLLM(t, ctx, "Click To Edit — view mode with name/email displayed and Edit button")
	})

	t.Run("Edit_Mode", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="edit"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('input[name="firstName"]') !== null`, 5*time.Second),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to enter edit mode: %v", err)
		}
		if !strings.Contains(html, `name="firstName"`) {
			t.Error("Edit form firstName input not found")
		}
		if !strings.Contains(html, `name="save"`) {
			t.Error("Save button not found")
		}
		if !strings.Contains(html, `name="cancel"`) {
			t.Error("Cancel button not found")
		}
	})

	t.Run("Save", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			// Clear and fill fields
			chromedp.Clear(`input[name="firstName"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="firstName"]`, "Jane", chromedp.ByQuery),
			chromedp.Clear(`input[name="lastName"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="lastName"]`, "Smith", chromedp.ByQuery),
			chromedp.Clear(`input[name="email"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="email"]`, "jane@smith.org", chromedp.ByQuery),
			chromedp.Click(`button[name="save"]`, chromedp.ByQuery),
			// Wait for view mode to return
			e2etest.WaitFor(`document.querySelector('article table') !== null`, 5*time.Second),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to save: %v", err)
		}
		if !strings.Contains(html, "Jane") {
			t.Error("Updated first name 'Jane' not found")
		}
		if !strings.Contains(html, "Smith") {
			t.Error("Updated last name 'Smith' not found")
		}
		if !strings.Contains(html, "jane@smith.org") {
			t.Error("Updated email not found")
		}
	})

	t.Run("Cancel", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			// Enter edit mode
			chromedp.Click(`button[name="edit"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('input[name="firstName"]') !== null`, 5*time.Second),
			// Cancel without saving
			chromedp.Click(`button[name="cancel"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('article table') !== null`, 5*time.Second),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to cancel: %v", err)
		}
		// Should still have the saved values from previous test
		if !strings.Contains(html, "Jane") {
			t.Error("First name should still be 'Jane' after cancel")
		}
	})
}

// --- Pattern #2: Edit Row ---

func TestEditRow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/forms/edit-row"

	t.Run("Initial_Load", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`table`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`tbody`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		if !strings.Contains(html, "Joe Smith") {
			t.Error("Contact 'Joe Smith' not found")
		}
		if !strings.Contains(html, "Kim Yee") {
			t.Error("Contact 'Kim Yee' not found")
		}
	})

	t.Run("UI_Standards", func(t *testing.T) {
		runUIStandardsWithPico(t, ctx)
	})

	t.Run("Visual_Check", func(t *testing.T) {
		e2etest.ValidateScreenshotWithLLM(t, ctx, "Edit Row — table with 4 contacts, each with name/email and Edit button")
	})

	t.Run("Edit_Row", func(t *testing.T) {
		// Click Edit on the first row
		err := chromedp.Run(ctx,
			chromedp.Click(`tr[data-key="1"] button[name="edit"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('tr[data-key="1"] input[name="name"]') !== null`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to enter edit mode for row 1: %v", err)
		}

		// Verify the edit form has the correct values
		var nameVal, emailVal string
		err = chromedp.Run(ctx,
			chromedp.Value(`tr[data-key="1"] input[name="name"]`, &nameVal, chromedp.ByQuery),
			chromedp.Value(`tr[data-key="1"] input[name="email"]`, &emailVal, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to read input values: %v", err)
		}
		if nameVal != "Joe Smith" {
			t.Errorf("Expected name 'Joe Smith', got %q", nameVal)
		}
		if emailVal != "joe@smith.org" {
			t.Errorf("Expected email 'joe@smith.org', got %q", emailVal)
		}
	})

	t.Run("Save_Row", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Clear(`tr[data-key="1"] input[name="name"]`, chromedp.ByQuery),
			chromedp.SendKeys(`tr[data-key="1"] input[name="name"]`, "Joseph Smith", chromedp.ByQuery),
			chromedp.Click(`tr[data-key="1"] button[name="save"]`, chromedp.ByQuery),
			e2etest.WaitForText(`tr[data-key="1"]`, "Joseph Smith", 5*time.Second),
			chromedp.OuterHTML(`tbody`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to save row: %v", err)
		}
		if !strings.Contains(html, "Joseph Smith") {
			t.Error("Updated name 'Joseph Smith' not found")
		}
		// Verify other rows are unaffected
		if !strings.Contains(html, "Angie MacDowell") {
			t.Error("Other contact 'Angie MacDowell' should still be present")
		}
		if !strings.Contains(html, "Kim Yee") {
			t.Error("Other contact 'Kim Yee' should still be present")
		}
	})

	t.Run("Cancel_Edit", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			// Edit row 2
			chromedp.Click(`tr[data-key="2"] button[name="edit"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('tr[data-key="2"] input[name="name"]') !== null`, 5*time.Second),
			// Cancel
			chromedp.Click(`tr[data-key="2"] button[name="cancel"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('tr[data-key="2"] input[name="name"]') === null`, 5*time.Second),
			chromedp.OuterHTML(`tbody`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to cancel edit: %v", err)
		}
		if !strings.Contains(html, "Angie MacDowell") {
			t.Error("Contact 'Angie MacDowell' should remain unchanged after cancel")
		}
	})
}

// --- Pattern #3: Inline Validation ---

func TestInlineValidation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/forms/inline-validation"

	t.Run("Initial_Load", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h3`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		if !strings.Contains(html, "Inline Validation") {
			t.Error("Page heading not found")
		}
	})

	t.Run("UI_Standards", func(t *testing.T) {
		runUIStandards(t, ctx)
	})

	t.Run("Visual_Check", func(t *testing.T) {
		e2etest.ValidateScreenshotWithLLM(t, ctx, "Inline Validation — email and username inputs with submit button, no errors shown yet")
	})

	t.Run("Valid_Submit", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="email"]`, "test@example.com", chromedp.ByQuery),
			chromedp.SendKeys(`input[name="username"]`, "testuser", chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Saved successfully", 5*time.Second),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to submit valid form: %v", err)
		}
		if !strings.Contains(html, "Saved successfully") {
			t.Error("Success message not found")
		}
	})
}

// --- Pattern #4: Bulk Update ---

func TestBulkUpdate(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/forms/bulk-update"

	t.Run("Initial_Load", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`table`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`tbody`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		if !strings.Contains(html, "Joe Smith") {
			t.Error("User 'Joe Smith' not found")
		}
		// Verify initial checkbox states (users 1,2 active; 3,4 inactive)
		var checked1, checked3 bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('input[name="active-1"]').checked`, &checked1),
			chromedp.Evaluate(`document.querySelector('input[name="active-3"]').checked`, &checked3),
		)
		if err != nil {
			t.Fatalf("Failed to check checkbox states: %v", err)
		}
		if !checked1 {
			t.Error("User 1 should be active initially")
		}
		if checked3 {
			t.Error("User 3 should be inactive initially")
		}
	})

	t.Run("UI_Standards", func(t *testing.T) {
		runUIStandardsWithPico(t, ctx)
	})

	t.Run("Visual_Check", func(t *testing.T) {
		e2etest.ValidateScreenshotWithLLM(t, ctx, "Bulk Update — table with 4 users, checkboxes for active status, Update button")
	})

	t.Run("Toggle_And_Update", func(t *testing.T) {
		err := chromedp.Run(ctx,
			// Uncheck user 1, check user 3
			chromedp.Click(`input[name="active-1"]`, chromedp.ByQuery),
			chromedp.Click(`input[name="active-3"]`, chromedp.ByQuery),
			// Click Update
			chromedp.Click(`button[name="bulkUpdate"]`, chromedp.ByQuery),
			// Wait for flash message (FlashTag renders as <output data-flash>)
			e2etest.WaitForText(`output[data-flash]`, "Updated", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to toggle and update: %v", err)
		}

		// Verify new checkbox states
		var checked1, checked2, checked3, checked4 bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('input[name="active-1"]').checked`, &checked1),
			chromedp.Evaluate(`document.querySelector('input[name="active-2"]').checked`, &checked2),
			chromedp.Evaluate(`document.querySelector('input[name="active-3"]').checked`, &checked3),
			chromedp.Evaluate(`document.querySelector('input[name="active-4"]').checked`, &checked4),
		)
		if err != nil {
			t.Fatalf("Failed to verify checkbox states: %v", err)
		}
		if checked1 {
			t.Error("User 1 should now be inactive")
		}
		if !checked2 {
			t.Error("User 2 should still be active")
		}
		if !checked3 {
			t.Error("User 3 should now be active")
		}
		if checked4 {
			t.Error("User 4 should still be inactive")
		}
	})
}

// --- Pattern #5: Reset User Input ---

func TestResetInput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/forms/reset-input"

	t.Run("Initial_Load", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h3`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		if !strings.Contains(html, "Reset User Input") {
			t.Error("Page heading not found")
		}
	})

	t.Run("UI_Standards", func(t *testing.T) {
		runUIStandardsWithPico(t, ctx)
	})

	t.Run("Visual_Check", func(t *testing.T) {
		e2etest.ValidateScreenshotWithLLM(t, ctx, "Reset User Input — message input with Send button, info text about auto-clear")
	})

	t.Run("Submit_Message", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="message"]`, "Hello World", chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Hello World", 5*time.Second),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to submit message: %v", err)
		}
		if !strings.Contains(html, "Hello World") {
			t.Error("Submitted message not found")
		}
	})

	t.Run("Form_Auto_Reset", func(t *testing.T) {
		// After submission, the input should be cleared
		var inputVal string
		err := chromedp.Run(ctx,
			chromedp.Value(`input[name="message"]`, &inputVal, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to read input value: %v", err)
		}
		if inputVal != "" {
			t.Errorf("Input should be empty after submit, got %q", inputVal)
		}
	})

	t.Run("Multiple_Messages", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="message"]`, "Second message", chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Second message", 5*time.Second),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to submit second message: %v", err)
		}
		// Both messages should be present
		if !strings.Contains(html, "Hello World") {
			t.Error("First message 'Hello World' should still be present")
		}
		if !strings.Contains(html, "Second message") {
			t.Error("Second message not found")
		}
	})
}

// --- Pattern #6: File Upload ---

func TestFileUpload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/forms/file-upload"

	t.Run("Initial_Load", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h3`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		if !strings.Contains(html, "File Upload") {
			t.Error("Page heading not found")
		}
		if !strings.Contains(html, "Tier 1") {
			t.Error("Tier 1 section not found")
		}
		if !strings.Contains(html, "Tier 2") {
			t.Error("Tier 2 section not found")
		}
	})

	t.Run("UI_Standards", func(t *testing.T) {
		runUIStandardsWithPico(t, ctx)
	})

	t.Run("Visual_Check", func(t *testing.T) {
		e2etest.ValidateScreenshotWithLLM(t, ctx, "File Upload — two sections: Tier 1 standard HTML upload and Tier 2 chunked upload, each with file input and Upload button")
	})

	t.Run("Submit_Without_File", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.Click(`button[name="upload"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "No file selected", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("No-file error flash not shown: %v", err)
		}
	})

	t.Run("Tier1_Upload_With_File", func(t *testing.T) {
		// Upload a file via Tier 1 (standard multipart form).
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`input[name="document"]`, chromedp.ByQuery),
			attachFileViaDataTransfer(`input[name="document"]`, "hello.txt", "hello world", "text/plain"),
			chromedp.Click(`button[name="upload"]`, chromedp.ByQuery),
			e2etest.WaitForText(`output[data-flash]`, "Uploaded: hello.txt", 10*time.Second),
		)
		if err != nil {
			var debugHTML string
			_ = chromedp.Run(ctx, chromedp.OuterHTML(`body`, &debugHTML, chromedp.ByQuery))
			t.Logf("Page HTML at failure:\n%s", debugHTML)
			t.Fatalf("Tier 1 upload failed: %v", err)
		}
	})

	t.Run("Form_Structure", func(t *testing.T) {
		// Verify both Tier 1 and Tier 2 upload forms are present
		var enctype string
		var hasFileInput, hasLvtUpload bool
		err := chromedp.Run(ctx,
			chromedp.AttributeValue(`form[enctype]`, "enctype", &enctype, nil, chromedp.ByQuery),
			chromedp.Evaluate(`document.querySelector('input[name="document"][type="file"]') !== null`, &hasFileInput),
			chromedp.Evaluate(`document.querySelector('input[lvt-upload="chunked-doc"]') !== null`, &hasLvtUpload),
		)
		if err != nil {
			t.Fatalf("Failed to verify form structure: %v", err)
		}
		if enctype != "multipart/form-data" {
			t.Errorf("Expected enctype='multipart/form-data', got %q", enctype)
		}
		if !hasFileInput {
			t.Error("Tier 1 file input not found")
		}
		if !hasLvtUpload {
			t.Error("Tier 2 lvt-upload input not found")
		}
	})
}

// --- Pattern #7: Preserving File Inputs ---

func TestPreserveInputs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/forms/preserve-inputs"

	t.Run("Initial_Load", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h3`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		if !strings.Contains(html, "Preserving Form Inputs") {
			t.Error("Page heading not found")
		}
		if !strings.Contains(html, `lvt-form:preserve`) {
			t.Error("lvt-form:preserve attribute not found")
		}
	})

	t.Run("UI_Standards", func(t *testing.T) {
		runUIStandards(t, ctx)
	})

	t.Run("Visual_Check", func(t *testing.T) {
		e2etest.ValidateScreenshotWithLLM(t, ctx, "Preserving Form Inputs — name input, description textarea, file attachment input, submit button")
	})

	t.Run("Submit_Shows_Flash", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="name"]`, "Test Name", chromedp.ByQuery),
			chromedp.SendKeys(`textarea[name="description"]`, "Test Description", chromedp.ByQuery),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Saved: Test Name", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to submit or flash not shown: %v", err)
		}
	})

	t.Run("Form_Values_Preserved_After_Submit", func(t *testing.T) {
		// After successful submit with lvt-form:preserve, form values
		// should NOT be cleared (unlike normal forms which auto-reset).
		var nameVal, descVal string
		err := chromedp.Run(ctx,
			chromedp.Value(`input[name="name"]`, &nameVal, chromedp.ByQuery),
			chromedp.Value(`textarea[name="description"]`, &descVal, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to read form values: %v", err)
		}
		if nameVal != "Test Name" {
			t.Errorf("Name should be preserved after submit, got %q", nameVal)
		}
		if descVal != "Test Description" {
			t.Errorf("Description should be preserved after submit, got %q", descVal)
		}
	})

	t.Run("Values_Survive_Rerender", func(t *testing.T) {
		// Submit again — triggers a re-render. Form values should survive
		// because lvt-form:preserve prevents the client from overwriting
		// input values during DOM patching.
		err := chromedp.Run(ctx,
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Saved: Test Name", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Second submit failed: %v", err)
		}

		var nameVal string
		err = chromedp.Run(ctx,
			chromedp.Value(`input[name="name"]`, &nameVal, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to read name after re-render: %v", err)
		}
		if nameVal != "Test Name" {
			t.Errorf("Name should survive re-render with lvt-form:preserve, got %q", nameVal)
		}
	})

	t.Run("Submit_With_File_Attached", func(t *testing.T) {
		// Regression: text fields must reach the server even when the
		// HTTP multipart path is taken (file attached).
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`input[name="name"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="name"]`, "WithFile Name", chromedp.ByQuery),
			chromedp.SendKeys(`textarea[name="description"]`, "With File Description", chromedp.ByQuery),
			attachFileViaDataTransfer(`input[name="attachment"]`, "test.txt", "test content", "text/plain"),
			chromedp.Click(`button[type="submit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`output[data-flash]`, "Saved: WithFile Name", 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Submit with file attached failed: %v", err)
		}
	})
}
