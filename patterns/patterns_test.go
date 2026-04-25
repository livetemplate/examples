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

// runStandardSubtests runs the boilerplate `UI_Standards` + `Visual_Check`
// subtest pair. `pico=true` invokes the Pico-variant UI check. Patterns
// that need additional setup before the UI check (e.g. waiting for
// entry animations to finish) should inline the subtests instead.
func runStandardSubtests(t *testing.T, ctx context.Context, pico bool, screenshotDesc string) {
	t.Helper()
	t.Run("UI_Standards", func(t *testing.T) {
		if pico {
			runUIStandardsWithPico(t, ctx)
		} else {
			runUIStandards(t, ctx)
		}
	})
	t.Run("Visual_Check", func(t *testing.T) {
		e2etest.ValidateScreenshotWithLLM(t, ctx, screenshotDesc)
	})
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

	runStandardSubtests(t, ctx, true, "Pattern index page — heading, 7 category cards with pattern links and descriptions")

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

	runStandardSubtests(t, ctx, true, "Click To Edit — view mode with name/email displayed and Edit button")

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

	runStandardSubtests(t, ctx, true, "Edit Row — table with 4 contacts, each with name/email and Edit button")

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

	runStandardSubtests(t, ctx, false, "Inline Validation — email and username inputs with submit button, no errors shown yet")

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

	runStandardSubtests(t, ctx, true, "Bulk Update — table with 4 users, checkboxes for active status, Update button")

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

	t.Run("Submit_With_No_Changes", func(t *testing.T) {
		// Clicking Update without toggling anything should report
		// "No changes" instead of a spurious "Updated N user(s)" count.
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="bulkUpdate"]`, chromedp.ByQuery),
			e2etest.WaitForText(`output[data-flash]`, "No changes", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Expected 'No changes' flash, got: %v", err)
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

	runStandardSubtests(t, ctx, true, "Reset User Input — message input with Send button, info text about auto-clear")

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

	runStandardSubtests(t, ctx, true, "File Upload — two sections: Tier 1 standard HTML upload and Tier 2 chunked upload, each with file input and Upload button")

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

	runStandardSubtests(t, ctx, false, "Preserving Form Inputs — name input, description textarea, file attachment input, submit button")

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

// --- Pattern #8: Delete Row ---

func TestDeleteRow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/lists/delete-row"

	t.Run("Initial_Load", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`table`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			e2etest.WaitForCount(`tbody tr[data-key]`, 5, 5*time.Second),
			chromedp.OuterHTML(`tbody`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		for i := 1; i <= 5; i++ {
			if !strings.Contains(html, fmt.Sprintf(`data-key="%d"`, i)) {
				t.Errorf("Row with data-key=%q not found", fmt.Sprintf("%d", i))
			}
		}
	})

	t.Run("UI_Standards", func(t *testing.T) {
		// Wait for lvt-fx:animate entry animations to finish before the
		// inline-style check — animationend clears the style attribute.
		err := chromedp.Run(ctx,
			e2etest.WaitFor(`Array.from(document.querySelectorAll('[data-key]')).every(el => !el.hasAttribute('style'))`, 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Animations did not complete: %v", err)
		}
		runUIStandards(t, ctx)
	})

	t.Run("Visual_Check", func(t *testing.T) {
		e2etest.ValidateScreenshotWithLLM(t, ctx, "Delete Row — table with 5 items showing ID, Name, Email columns and a Delete button on each row")
	})

	t.Run("Delete_First_Row", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Click(`tr[data-key="1"] button[name="delete"]`, chromedp.ByQuery),
			e2etest.WaitForCount(`tbody tr[data-key]`, 4, 5*time.Second),
			chromedp.OuterHTML(`tbody`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to delete first row: %v", err)
		}
		if strings.Contains(html, `data-key="1"`) {
			t.Error("Row 1 still present after delete")
		}
		if !strings.Contains(html, `data-key="2"`) {
			t.Error("Row 2 should still be present")
		}
	})

	t.Run("Delete_All_Remaining_Rows", func(t *testing.T) {
		// Delete rows 2, 3, 4, 5 one at a time, asserting the count after each.
		for _, row := range []struct {
			id            string
			expectedAfter int
		}{
			{"2", 3},
			{"3", 2},
			{"4", 1},
			{"5", 0},
		} {
			err := chromedp.Run(ctx,
				chromedp.Click(fmt.Sprintf(`tr[data-key="%s"] button[name="delete"]`, row.id), chromedp.ByQuery),
				e2etest.WaitForCount(`tbody tr[data-key]`, row.expectedAfter, 5*time.Second),
			)
			if err != nil {
				t.Fatalf("Failed to delete row %s: %v", row.id, err)
			}
		}
		// Assert empty state message appears and Restore button is present
		err := chromedp.Run(ctx,
			e2etest.WaitForText(`article`, "All items deleted", 5*time.Second),
			chromedp.WaitVisible(`button[name="restore"]`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Empty state or restore button not shown: %v", err)
		}
	})

	t.Run("State_Persists_Across_Reload", func(t *testing.T) {
		// Reload the page — the shared in-memory DB should still be empty
		// from the previous Delete_All_Remaining_Rows subtest, proving that
		// state persists across reloads without needing lvt:"persist" tags.
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			e2etest.WaitForText(`article`, "All items deleted", 5*time.Second),
			chromedp.WaitVisible(`button[name="restore"]`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Empty state did not persist across reload: %v", err)
		}
	})

	t.Run("Restore_Refills_Items", func(t *testing.T) {
		// Click Restore to refill the DB. All 5 items should reappear.
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="restore"]`, chromedp.ByQuery),
			e2etest.WaitForCount(`tbody tr[data-key]`, 5, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Restore did not refill items: %v", err)
		}
	})
}

// --- Pattern #9: Click To Load ---

func TestClickToLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/lists/click-to-load"

	t.Run("Initial_Load", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`table`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			e2etest.WaitForCount(`tbody tr[data-key]`, 10, 5*time.Second),
			chromedp.OuterHTML(`article`, &html, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		if !strings.Contains(html, `name="loadMore"`) {
			t.Error("Load More button not found")
		}
		if !strings.Contains(html, "Item 10") {
			t.Error("First page's last item (Item 10) not found")
		}
		if strings.Contains(html, "Item 11") {
			t.Error("Second page item (Item 11) should not be present yet")
		}
	})

	runStandardSubtests(t, ctx, false, "Click To Load — table with 10 rows (ID, Name, Email) and a Load More button below")

	t.Run("Load_Second_Page", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="loadMore"]`, chromedp.ByQuery),
			e2etest.WaitForCount(`tbody tr[data-key]`, 20, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to load second page: %v", err)
		}
		var html string
		err = chromedp.Run(ctx, chromedp.OuterHTML(`tbody`, &html, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("Failed to read tbody: %v", err)
		}
		if !strings.Contains(html, "Item 11") {
			t.Error("Second page item (Item 11) not found after load")
		}
		if !strings.Contains(html, "Item 20") {
			t.Error("Second page's last item (Item 20) not found after load")
		}
	})

	t.Run("Load_Third_Page_And_Hide_Button", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="loadMore"]`, chromedp.ByQuery),
			e2etest.WaitForCount(`tbody tr[data-key]`, 25, 5*time.Second),
			// Wait for the button to disappear (HasMore flips false when the
			// final page returns fewer than listPageSize items).
			e2etest.WaitFor(`document.querySelector('button[name="loadMore"]') === null`, 5*time.Second),
			e2etest.WaitForText(`article`, "End of list", 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to load third page: %v", err)
		}
	})
}

// --- Pattern #11: Value Select ---

// selectValueAndDispatchChange sets a <select>'s value and dispatches a
// bubbling change event so the LiveTemplate client's Change auto-wirer fires.
// chromedp.Click cannot open native <select> dropdowns in headless Chrome.
func selectValueAndDispatchChange(selector, value string) chromedp.Action {
	script := fmt.Sprintf(`(() => {
		const el = document.querySelector(%q);
		if (!el) return 'missing:' + %q;
		el.value = %q;
		el.dispatchEvent(new Event('change', { bubbles: true }));
		return 'ok';
	})()`, selector, selector, value)
	return chromedp.Evaluate(script, nil)
}

func TestValueSelect(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/lists/value-select"

	t.Run("Initial_Load", func(t *testing.T) {
		var makeOptionCount int
		var modelDisabled bool
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`select[name="make"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.Evaluate(`document.querySelectorAll('select[name="make"] option').length`, &makeOptionCount),
			chromedp.Evaluate(`document.querySelector('select[name="model"]').disabled`, &modelDisabled),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		// 1 placeholder + 3 makes (Audi, BMW, Toyota)
		if makeOptionCount != 4 {
			t.Errorf("Expected 4 make options, got %d", makeOptionCount)
		}
		if !modelDisabled {
			t.Error("Model select should be disabled when no make is selected")
		}
	})

	runStandardSubtests(t, ctx, false, "Value Select — Make dropdown with 3 car makes and Model dropdown disabled until a make is selected")

	t.Run("Select_Make_Auto_Selects_First_Model", func(t *testing.T) {
		// Selecting a Make auto-selects the first Model for immediate visual
		// feedback — the Model dropdown's value updates and the "Selected:"
		// line appears without needing a second user click.
		err := chromedp.Run(ctx,
			selectValueAndDispatchChange(`select[name="make"]`, "Audi"),
			// Wait for Model options to be populated (4 models + placeholder = 5).
			e2etest.WaitFor(`document.querySelectorAll('select[name="model"] option').length === 5`, 5*time.Second),
			// Wait for the auto-selected "Audi A3" line to appear.
			e2etest.WaitForText(`article`, "Audi A3", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to select make or auto-select model: %v", err)
		}
		var html string
		err = chromedp.Run(ctx, chromedp.OuterHTML(`select[name="model"]`, &html, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("Failed to read model select: %v", err)
		}
		for _, model := range []string{"A3", "A4", "Q5", "R8"} {
			if !strings.Contains(html, model) {
				t.Errorf("Expected Audi model %q in select, got:\n%s", model, html)
			}
		}
	})

	t.Run("Select_Model_Updates_Selection", func(t *testing.T) {
		err := chromedp.Run(ctx,
			selectValueAndDispatchChange(`select[name="model"]`, "A4"),
			e2etest.WaitForText(`article`, "Audi A4", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to select model: %v", err)
		}
	})

	t.Run("Change_Make_Auto_Selects_New_First_Model", func(t *testing.T) {
		// Switching Make auto-selects the new Make's first Model — so the
		// previous "Audi A4" line becomes "BMW 3 Series" without the user
		// needing to touch the Model dropdown.
		err := chromedp.Run(ctx,
			selectValueAndDispatchChange(`select[name="make"]`, "BMW"),
			e2etest.WaitFor(`(() => {
				const opts = document.querySelectorAll('select[name="model"] option');
				if (opts.length !== 5) return false;
				const texts = Array.from(opts).map(o => o.textContent);
				return texts.includes('3 Series') && !texts.includes('A4');
			})()`, 5*time.Second),
			e2etest.WaitForText(`article`, "BMW 3 Series", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to switch make or auto-select new model: %v", err)
		}
		// The previous "Audi A4" line should be gone.
		var html string
		err = chromedp.Run(ctx, chromedp.OuterHTML(`article`, &html, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("Failed to read article: %v", err)
		}
		if strings.Contains(html, "Audi A4") {
			t.Error("Previous selection 'Audi A4' should be cleared after make change")
		}
	})
}

// --- Pattern #12: Active Search ---

func TestActiveSearch(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/search/active-search"

	t.Run("Initial_Load", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`input[name="query"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			// Full directory is 25 contacts
			e2etest.WaitForCount(`tbody tr[data-key]`, 25, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
	})

	runStandardSubtests(t, ctx, false, "Active Search — search input labeled 'Search contacts' with a table of 25 contacts showing Name and Email columns below")

	t.Run("Filter_To_Single_Result", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Focus(`input[name="query"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="query"]`, "Chen", chromedp.ByQuery),
			// WaitForCount naturally waits out the 300ms debounce
			e2etest.WaitForCount(`tbody tr[data-key]`, 1, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to filter results: %v", err)
		}
		var html string
		err = chromedp.Run(ctx, chromedp.OuterHTML(`tbody`, &html, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("Failed to read tbody: %v", err)
		}
		if !strings.Contains(html, "Marcus Chen") {
			t.Errorf("Expected Marcus Chen in results, got:\n%s", html)
		}
	})

	t.Run("Clear_Query_Restores_All", func(t *testing.T) {
		// chromedp.Clear doesn't fire DOM events — set value and dispatch both
		// `input` (what the Change auto-wirer listens for on text inputs) and
		// `change` (defensive for event-filter implementations) in a single
		// script so the auto-wirer picks it up regardless.
		//
		// Timeout bumped to 10s: this test was flaky under CI load where
		// orphan processes from earlier tests compete for CPU. Locally
		// completes in ~0.4s; CI failure pattern was a hard 5s timeout
		// while still showing the previous query's 1-result state.
		err := chromedp.Run(ctx,
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.Focus(`input[name="query"]`, chromedp.ByQuery),
			chromedp.Evaluate(`(() => {
				const el = document.querySelector('input[name="query"]');
				el.value = '';
				el.dispatchEvent(new Event('input', { bubbles: true }));
				el.dispatchEvent(new Event('change', { bubbles: true }));
				return el.value;
			})()`, nil),
			e2etest.WaitForCount(`tbody tr[data-key]`, 25, 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to clear query: %v", err)
		}
	})

	t.Run("Empty_Results_Shows_No_Results", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Focus(`input[name="query"]`, chromedp.ByQuery),
			chromedp.SendKeys(`input[name="query"]`, "xzyzzzz-no-match", chromedp.ByQuery),
			e2etest.WaitForCount(`tbody tr[data-key]`, 0, 5*time.Second),
			e2etest.WaitForText(`article`, "No contacts match", 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to show empty results: %v", err)
		}
	})
}

// --- Pattern #13: URL-Preserved Filters ---

func TestURLFilters(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	baseURL := e2etest.GetChromeTestURL(serverPort) + "/patterns/search/url-filters"

	t.Run("Initial_Load", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`table`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			// Full dataset: 12 items
			e2etest.WaitForCount(`tbody tr[data-key]`, 12, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}
		// "All" and "By Name" should have aria-current="page"
		var html string
		err = chromedp.Run(ctx, chromedp.OuterHTML(`article nav`, &html, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("Failed to read nav: %v", err)
		}
		if !strings.Contains(html, `aria-current="page">All`) {
			t.Errorf("Expected 'All' link marked aria-current, got:\n%s", html)
		}
		if !strings.Contains(html, `aria-current="page">By Name`) {
			t.Errorf("Expected 'By Name' link marked aria-current, got:\n%s", html)
		}
	})

	runStandardSubtests(t, ctx, false, "URL-Preserved Filters — two groups of filter links (status: All/Active/Completed and sort: By Name/By Date) above a table of items with Name, Status, Date columns")

	t.Run("Filter_By_Active", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`a[href="?status=active&sort=name"]`, chromedp.ByQuery),
			// 7 active items in filterDataset (IDs 3, 4, 6, 8, 10, 11, 12).
			e2etest.WaitForCount(`tbody tr[data-key]`, 7, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to filter by active: %v", err)
		}
		var currentURL string
		err = chromedp.Run(ctx, chromedp.Location(&currentURL))
		if err != nil {
			t.Fatalf("Failed to read URL: %v", err)
		}
		if !strings.Contains(currentURL, "status=active") {
			t.Errorf("URL should contain status=active, got: %s", currentURL)
		}
	})

	t.Run("Bookmarkable_Reload", func(t *testing.T) {
		// Direct navigate to a filtered URL (simulates bookmark reload).
		err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL+"?status=completed&sort=date"),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`table`, chromedp.ByQuery),
			// Completed items: 1, 2, 5, 7, 9 = 5
			e2etest.WaitForCount(`tbody tr[data-key]`, 5, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Bookmarked URL did not restore state: %v", err)
		}
		// Verify sort order is date-desc: first row should be the newest completed item
		// (ID 9, 2024-08-19) per filterDataset in data.go.
		var firstRowHTML string
		err = chromedp.Run(ctx, chromedp.OuterHTML(`tbody tr:first-child`, &firstRowHTML, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("Failed to read first row: %v", err)
		}
		if !strings.Contains(firstRowHTML, "2024-08-19") {
			t.Errorf("Expected newest completed item (2024-08-19) first, got:\n%s", firstRowHTML)
		}
		var navHTML string
		err = chromedp.Run(ctx, chromedp.OuterHTML(`article nav`, &navHTML, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("Failed to read nav: %v", err)
		}
		if !strings.Contains(navHTML, `aria-current="page">Completed`) {
			t.Errorf("Completed link should be marked aria-current after bookmarked reload, got:\n%s", navHTML)
		}
		if !strings.Contains(navHTML, `aria-current="page">By Date`) {
			t.Errorf("By Date link should be marked aria-current after bookmarked reload, got:\n%s", navHTML)
		}
	})

	t.Run("Invalid_Status_Falls_Back", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(baseURL+"?status=nonsense&sort=date"),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`table`, chromedp.ByQuery),
			// Unknown status falls back to default "all" → 12 items
			e2etest.WaitForCount(`tbody tr[data-key]`, 12, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Invalid status did not fall back gracefully: %v", err)
		}
	})

	t.Run("Reset_To_All", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`a[href="?status=all&sort=date"]`, chromedp.ByQuery),
			e2etest.WaitForCount(`tbody tr[data-key]`, 12, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Failed to reset to all: %v", err)
		}
	})
}

// --- Pattern #10: Infinite Scroll ---

// TestInfiniteScroll verifies the [lvt-scroll-sentinel] IntersectionObserver
// wiring and the loadMorePending throttle. In headless Chrome the short
// first page keeps the sentinel intersecting, so page 2 auto-advances;
// subsequent pages require an explicit scroll because the sentinel has
// drifted past the 200px rootMargin.
func TestInfiniteScroll(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/lists/infinite-scroll"

	t.Run("Initial_Load_And_Auto_Advance", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`table`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			// First page renders, observer auto-advances while sentinel is
			// in view (safely throttled by the client's loadMorePending flag).
			e2etest.WaitFor(`document.querySelectorAll('tbody tr[data-key]').length >= 10`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}
		// Wait for the auto-advance to settle: two consecutive polls with
		// the same row count (rows have stopped arriving).
		var dataKeys string
		err = chromedp.Run(ctx,
			e2etest.WaitFor(`(() => {
				const prev = window.__lastRowCount || 0;
				const cur = document.querySelectorAll('tbody tr[data-key]').length;
				window.__lastRowCount = cur;
				return cur === prev && cur > 0;
			})()`, 3*time.Second),
			chromedp.Evaluate(`Array.from(document.querySelectorAll('tbody tr[data-key]')).map(r => r.getAttribute('data-key')).join(',')`, &dataKeys),
		)
		if err != nil {
			t.Fatalf("Auto-advance did not settle: %v", err)
		}
		// Verify no duplicate data-keys — the client's loadMorePending flag
		// plus the WS-aware connect() ensure that each load_more lands
		// exactly once on the server-side persistent state path.
		seen := make(map[string]bool)
		for _, k := range strings.Split(dataKeys, ",") {
			if seen[k] {
				t.Fatalf("Duplicate data-key %q after auto-advance: %s", k, dataKeys)
			}
			seen[k] = true
		}
	})

	runStandardSubtests(t, ctx, false, "Infinite Scroll — table with 20 rows (ID, Name, Email) followed by a 'Loading more…' sentinel at the bottom")

	t.Run("Scroll_Triggers_More_Pages", func(t *testing.T) {
		// Scroll the sentinel into view repeatedly. Each scroll fires one
		// observer callback (throttled by the client's loadMorePending flag),
		// appending one more page. With the 100-item dataset at page size 10,
		// we'd need ~8-10 scrolls to fully exhaust, so we verify the pipeline
		// works by scrolling twice and confirming two extra pages loaded.
		var baseline int
		_ = chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('tbody tr[data-key]').length`, &baseline))
		if baseline < 10 {
			t.Fatalf("Baseline row count too low: %d", baseline)
		}
		// Scroll once
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`(() => {
				const s = document.querySelector('[lvt-scroll-sentinel]');
				if (s) s.scrollIntoView({ block: 'center' });
			})()`, nil),
			e2etest.WaitFor(`document.querySelectorAll('tbody tr[data-key]').length > `+fmt.Sprintf("%d", baseline), 5*time.Second),
		)
		if err != nil {
			t.Fatalf("First scroll did not trigger a new page: %v", err)
		}
		// Scroll again
		var afterFirstScroll int
		_ = chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('tbody tr[data-key]').length`, &afterFirstScroll))
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`(() => {
				const s = document.querySelector('[lvt-scroll-sentinel]');
				if (s) s.scrollIntoView({ block: 'center' });
			})()`, nil),
			e2etest.WaitFor(`document.querySelectorAll('tbody tr[data-key]').length > `+fmt.Sprintf("%d", afterFirstScroll), 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Second scroll did not trigger a new page: %v", err)
		}
		// Duplicate check: all data-keys are unique.
		var dataKeys string
		_ = chromedp.Run(ctx,
			chromedp.Evaluate(`Array.from(document.querySelectorAll('tbody tr[data-key]')).map(r => r.getAttribute('data-key')).join(',')`, &dataKeys),
		)
		seen := make(map[string]bool)
		for _, k := range strings.Split(dataKeys, ",") {
			if seen[k] {
				t.Errorf("Duplicate data-key %q after scroll-driven pagination: %s", k, dataKeys)
			}
			seen[k] = true
		}
		// Sanity: at least 3 items from past the first page should be present.
		var html string
		_ = chromedp.Run(ctx, chromedp.OuterHTML(`tbody`, &html, chromedp.ByQuery))
		if !strings.Contains(html, "Row 1") {
			t.Error("Row 1 (first item) missing after scroll")
		}
	})
}

// --- Session 3: Loading & Progress ---

func TestLazyLoading(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/loading/lazy-loading"

	t.Run("Initial_Load_Shows_Spinner", func(t *testing.T) {
		// The page should render immediately with the spinner; the content
		// blockquote must be absent until the goroutine fires (~2s later).
		var hasBlockquote bool
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`p[aria-busy="true"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.Evaluate(`!!document.querySelector('blockquote')`, &hasBlockquote),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}
		if hasBlockquote {
			t.Error("Blockquote should not be present while still loading")
		}
	})

	t.Run("Data_Arrives_Via_Server_Push", func(t *testing.T) {
		// The goroutine sleeps 2s then pushes via TriggerAction. 5s timeout
		// is generous. After arrival, the spinner must be gone.
		var hasSpinner bool
		err := chromedp.Run(ctx,
			e2etest.WaitForText(`blockquote`, "Content loaded lazily", 5*time.Second),
			chromedp.Evaluate(`!!document.querySelector('p[aria-busy="true"]')`, &hasSpinner),
		)
		if err != nil {
			t.Fatalf("Data did not arrive: %v", err)
		}
		if hasSpinner {
			t.Error("Spinner should be gone after data arrives")
		}
	})

	t.Run("Reload_Refetches_Fresh_Content", func(t *testing.T) {
		// Click Reload; spinner reappears; new content arrives via a fresh
		// goroutine push. The two strings have different prefixes ("Content
		// loaded lazily at …" vs "Content reloaded at …"), so an inequality
		// check between them is trivially true and would not actually prove
		// that a second goroutine ran. Instead, assert directly on the
		// expected prefix transitions: firstContent must be the
		// initial-load message, secondContent must be the reload message.
		// Both prefixes are produced by separate goroutine paths, so this
		// assertion proves real second-goroutine execution.
		var firstContent, secondContent string
		err := chromedp.Run(ctx,
			chromedp.Text(`blockquote`, &firstContent, chromedp.ByQuery),
			chromedp.Click(`button[name="reload"]`, chromedp.ByQuery),
			chromedp.WaitVisible(`p[aria-busy="true"]`, chromedp.ByQuery),
			e2etest.WaitForText(`blockquote`, "Content reloaded", 5*time.Second),
			chromedp.Text(`blockquote`, &secondContent, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Reload failed: %v", err)
		}
		if !strings.Contains(firstContent, "Content loaded lazily") {
			t.Errorf("First content was not the initial load message: %q", firstContent)
		}
		if strings.Contains(firstContent, "Content reloaded") {
			t.Errorf("First content already had the reload prefix — test ordering broken: %q", firstContent)
		}
		if !strings.Contains(secondContent, "Content reloaded") {
			t.Errorf("Second content did not have the reload prefix: %q", secondContent)
		}
		if strings.Contains(secondContent, "Content loaded lazily") {
			t.Errorf("Second content still had the initial-load prefix: %q", secondContent)
		}
	})

	runStandardSubtests(t, ctx, false, "Lazy Loading — page showing a blockquote with lazily-loaded content and a secondary 'Reload' button below")
}

func TestProgressBar(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/loading/progress-bar"

	t.Run("Initial_Load", func(t *testing.T) {
		var hasProgress bool
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`button[name="start"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.Evaluate(`!!document.querySelector('progress')`, &hasProgress),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}
		if hasProgress {
			t.Error("<progress> should not be present before Start is clicked")
		}
	})

	t.Run("Start_Runs_To_Completion", func(t *testing.T) {
		// Click Start; progress element appears and ticks up. Goroutine runs
		// 10 × 500ms = 5s, so wait 10s for completion and the success flash.
		// The intermediate-tick assertion (value > 0 AND value < 100) catches
		// a regression where the goroutine skips intermediate ticks and jumps
		// straight to 100 — a `value > 0` check alone would also be satisfied
		// by an instant 100, missing the bug.
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="start"]`, chromedp.ByQuery),
			e2etest.WaitFor(`!!document.querySelector('progress')`, 3*time.Second),
			// Progress element is mid-flight: above 0 and below 100.
			// This proves the goroutine is actually ticking, not jumping.
			// 5s timeout (matching Run_Again_Restarts_Timer) gives loaded
			// CI runners a comfortable margin before the goroutine completes
			// the full 5s run and the value reaches 100.
			e2etest.WaitFor(`document.querySelector('progress') && document.querySelector('progress').value > 0 && document.querySelector('progress').value < 100`, 5*time.Second),
			// Run Again button indicates the Done state.
			e2etest.WaitForText(`button`, "Run Again", 10*time.Second),
			e2etest.WaitForText(`output[data-flash="success"]`, "Job complete", 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Progress bar did not complete: %v", err)
		}
	})

	t.Run("Run_Again_Restarts_Timer", func(t *testing.T) {
		// The Run Again button starts the timer again. Progress must begin
		// from below 100, climb back to completion, AND re-emit the success
		// flash. The flash assertion catches a regression where the second
		// run completes silently (e.g., if the controller forgot to call
		// SetFlash on the re-completion path).
		//
		// The intermediate-tick timeout is 5s (not 3s) so that on a heavily
		// loaded CI runner, where the first WS tick may be delayed, we still
		// catch a real value < 100 before the goroutine completes the full
		// 5s run. 3s was tight enough that a slow runner could miss the
		// window even though the goroutine was working correctly.
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="start"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('progress') && document.querySelector('progress').value > 0 && document.querySelector('progress').value < 100`, 5*time.Second),
			e2etest.WaitForText(`button`, "Run Again", 10*time.Second),
			e2etest.WaitForText(`output[data-flash="success"]`, "Job complete", 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Run Again failed: %v", err)
		}
	})

	runStandardSubtests(t, ctx, false, "Progress Bar — completed state showing a full progress bar, a 'Job complete' success flash below it, and a 'Run Again' button")
}

func TestAsyncOperations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/loading/async-operations"

	t.Run("Initial_Load", func(t *testing.T) {
		var hasResult bool
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			e2etest.WaitForText(`button[name="fetch"]`, "Fetch Data", 3*time.Second),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.Evaluate(`!!document.querySelector('blockquote') || !!document.querySelector('mark')`, &hasResult),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}
		if hasResult {
			t.Error("Result/error display should not be present before Fetch is clicked")
		}
	})

	t.Run("Fetch_Transitions_Through_Loading_To_Result", func(t *testing.T) {
		// Click Fetch → transient loading state → final success OR error.
		// The branch is random (~33% error rate). Tests must tolerate either.
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="fetch"]`, chromedp.ByQuery),
			// Loading state: button shows "Fetching..." and aria-busy.
			e2etest.WaitForText(`button[name="fetch"]`, "Fetching...", 3*time.Second),
			// Final state: either <blockquote> (success) or <mark> (error).
			e2etest.WaitFor(`!!document.querySelector('blockquote') || !!document.querySelector('mark')`, 5*time.Second),
			// Button must re-enable (exits "loading" status).
			e2etest.WaitForText(`button[name="fetch"]`, "Fetch Data", 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Async flow did not complete: %v", err)
		}
		// Exactly one of success or error must be present, plus the matching
		// flash. The flash text is asserted against the controller's exact
		// SetFlash message, not just the element presence — an empty
		// <output data-flash=""> placeholder would satisfy a presence-only
		// check and silently mask a regression where SetFlash wasn't called.
		//
		// `outcome` is read first, then both the wait-for-flash and the
		// flash-text read are batched into a single chromedp.Run so the
		// outcome value can't drift between the read and the wait.
		var outcome string
		err = chromedp.Run(ctx, chromedp.Evaluate(`(() => {
			if (document.querySelector('blockquote')) return 'success';
			if (document.querySelector('mark')) return 'error';
			return 'none';
		})()`, &outcome))
		if err != nil {
			t.Fatalf("Failed to read outcome: %v", err)
		}
		if outcome == "none" {
			t.Fatal("No outcome (neither success nor error) rendered")
		}
		// Map outcome → expected flash text from the controller.
		// Mirrors AsyncOpsController.FetchResult ctx.SetFlash calls.
		expectedFlashText := map[string]string{
			"success": "Fetch complete",
			"error":   "Fetch failed",
		}[outcome]
		flashSelector := fmt.Sprintf(`output[data-flash="%s"]`, outcome)
		var flashText string
		err = chromedp.Run(ctx,
			e2etest.WaitFor(fmt.Sprintf(`!!document.querySelector('%s')`, flashSelector), 3*time.Second),
			chromedp.Evaluate(
				fmt.Sprintf(`(() => { const el = document.querySelector('%s'); return el ? el.textContent.trim() : ""; })()`, flashSelector),
				&flashText,
			),
		)
		if err != nil {
			t.Fatalf("Outcome %q: failed to read %s: %v", outcome, flashSelector, err)
		}
		if !strings.Contains(flashText, expectedFlashText) {
			t.Errorf("Outcome %q: flash text = %q, want it to contain %q", outcome, flashText, expectedFlashText)
		}
	})

	// Regression test for the AsyncOpsController.Fetch Running guard.
	// Without the guard, two rapid `fetch` actions sent via direct
	// WebSocket message (bypassing the template-disabled button) would
	// each spawn a goroutine that calls TriggerAction("fetchResult"),
	// resulting in two state transitions, two SetFlash calls, and
	// potentially malformed rendered state. With the guard, the second
	// Fetch is a no-op (state.Status == "loading" → return early).
	//
	// This test asserts the user-visible invariant: concurrent Fetch
	// calls leave the UI in a single consistent state with exactly one
	// result element (blockquote OR mark, never both, never stacked).
	// It does not directly verify the guard rejected the second call —
	// detecting that from the rendered HTML is hard because the state
	// machine is idempotent in its final state — but it does prove the
	// guard's user-visible promise (concurrent Fetches don't break the
	// page) holds.
	t.Run("Concurrent_Fetch_Reaches_Single_Result", func(t *testing.T) {
		var resultCount int
		err := chromedp.Run(ctx,
			// Wait for idle state from the previous subtest.
			e2etest.WaitForText(`button[name="fetch"]`, "Fetch Data", 3*time.Second),
			// Send two Fetch actions in immediate sequence via direct WS,
			// bypassing the rendered button (which would be disabled
			// after the first click).
			chromedp.Evaluate(`(() => {
				window.liveTemplateClient.send({action: 'fetch'});
				window.liveTemplateClient.send({action: 'fetch'});
			})()`, nil),
			// Wait for the cycle to complete: button returns to "Fetch Data".
			// Total time: ~2s for the goroutine sleep + WS roundtrip.
			e2etest.WaitForText(`button[name="fetch"]`, "Fetch Data", 5*time.Second),
			// Count result elements. Exactly one of (blockquote, mark) must
			// be present. If two goroutines somehow corrupted the state
			// machine, we might see zero, two of either, or both.
			chromedp.Evaluate(`document.querySelectorAll('blockquote, mark').length`, &resultCount),
		)
		if err != nil {
			t.Fatalf("Concurrent Fetch test failed: %v", err)
		}
		if resultCount != 1 {
			t.Errorf("Expected exactly 1 result element after concurrent Fetch, got %d", resultCount)
		}
	})

	runStandardSubtests(t, ctx, false, "Async Operations — 'Fetch Data' button followed by either a success flash and blockquote with fetch result, or an error flash and mark element with an error message")
}

// --- Pattern #17: Modal Dialog ---

func TestModalDialog(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/navigation/modal-dialog"

	t.Run("Initial_Load", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`button[commandfor="edit-dialog"][command="show-modal"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}
		var dialogOpen bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`document.getElementById('edit-dialog').open`, &dialogOpen)); err != nil {
			t.Fatalf("Read dialog state failed: %v", err)
		}
		if dialogOpen {
			t.Error("Dialog should be closed on initial load")
		}
	})

	t.Run("Open_Via_Button", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`button[commandfor="edit-dialog"][command="show-modal"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.getElementById('edit-dialog').open === true`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Open via button failed: %v", err)
		}
	})

	t.Run("Submit_Invalid_Form_Stays_Open_With_Field_Errors", func(t *testing.T) {
		// noValidate=true bypasses HTML5 form validation so the empty input
		// reaches the server's validator, which is what we want to exercise.
		err := chromedp.Run(ctx,
			e2etest.WaitFor(`document.getElementById('edit-dialog').open === true`, 3*time.Second),
			chromedp.Evaluate(`document.querySelector('dialog#edit-dialog form').noValidate = true`, nil),
			chromedp.Clear(`dialog#edit-dialog input[name="name"]`, chromedp.ByQuery),
			chromedp.Click(`dialog#edit-dialog button[type="submit"]`, chromedp.ByQuery),
			e2etest.WaitFor(`(() => { const d = document.getElementById('edit-dialog'); return d.open && d.querySelector('input[name="name"][aria-invalid="true"]') !== null; })()`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Invalid form submit did not produce field error inside open dialog: %v", err)
		}
		var dialogOpen bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`document.getElementById('edit-dialog').open`, &dialogOpen)); err != nil {
			t.Fatalf("Read dialog state failed: %v", err)
		}
		if !dialogOpen {
			t.Error("Dialog should remain open after invalid submit")
		}
		var errorText string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`(() => { const s = document.querySelector('dialog#edit-dialog small'); return s ? s.textContent.trim() : ""; })()`, &errorText)); err != nil {
			t.Fatalf("Read error text failed: %v", err)
		}
		if errorText == "" {
			t.Error("Expected a field error message inside the dialog, found none")
		}
	})

	t.Run("Submit_Valid_Form_Closes_Dialog_And_Updates_State", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Clear(`dialog#edit-dialog input[name="name"]`, chromedp.ByQuery),
			chromedp.SendKeys(`dialog#edit-dialog input[name="name"]`, "Grace Hopper", chromedp.ByQuery),
			chromedp.Click(`dialog#edit-dialog button[type="submit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`output[data-flash="success"]`, "Profile saved", 5*time.Second),
			e2etest.WaitFor(`document.getElementById('edit-dialog').open === false`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Valid form submit did not produce success flash + dialog close: %v", err)
		}
		var bodyText string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`document.body.textContent`, &bodyText)); err != nil {
			t.Fatalf("Read body text failed: %v", err)
		}
		if !strings.Contains(bodyText, "Grace Hopper") {
			t.Error("Saved Name 'Grace Hopper' not visible in page text")
		}
		// Re-open the dialog and verify the form input now reflects the saved
		// state (the value="{{.Name}}" template expression should have rerendered).
		var nameValue string
		err = chromedp.Run(ctx,
			chromedp.Click(`button[commandfor="edit-dialog"][command="show-modal"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.getElementById('edit-dialog').open === true`, 5*time.Second),
			chromedp.Value(`dialog#edit-dialog input[name="name"]`, &nameValue, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Re-open dialog to verify form state failed: %v", err)
		}
		if nameValue != "Grace Hopper" {
			t.Errorf("Form input not repopulated from saved state; got %q, want %q", nameValue, "Grace Hopper")
		}
	})

	t.Run("Open_Via_Hash_Link", func(t *testing.T) {
		// Reset to a clean URL first (no #hash), then click the hash anchor.
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`a[href="#edit-dialog"]`, chromedp.ByQuery),
			chromedp.Click(`a[href="#edit-dialog"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.getElementById('edit-dialog').open === true`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Open via hash link failed: %v", err)
		}
		var hash string
		if err := chromedp.Run(ctx, chromedp.Evaluate(`location.hash`, &hash)); err != nil {
			t.Fatalf("Read hash failed: %v", err)
		}
		if hash != "#edit-dialog" {
			t.Errorf("Expected #edit-dialog after hash-link click, got %q", hash)
		}
	})

	t.Run("Browser_Back_Closes_Dialog", func(t *testing.T) {
		err := chromedp.Run(ctx,
			e2etest.WaitFor(`document.getElementById('edit-dialog').open === true`, 3*time.Second),
			chromedp.Evaluate(`history.back()`, nil),
			e2etest.WaitFor(`document.getElementById('edit-dialog').open === false`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Browser Back did not close the dialog: %v", err)
		}
	})

	runStandardSubtests(t, ctx, false, "Modal Dialog — page heading, profile summary, an 'Edit profile' button, and an 'Open via URL hash' secondary link. The dialog itself is closed at this point.")
}

// --- Pattern #18: Confirm Dialog ---

func TestConfirmDialog(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/navigation/confirm-dialog"

	t.Run("Initial_Load", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`button[commandfor="confirm-1"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}
		var rowCount int
		if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('tbody tr[data-key]').length`, &rowCount)); err != nil {
			t.Fatalf("Row count read failed: %v", err)
		}
		if rowCount != confirmDialogItemCount {
			t.Errorf("Expected %d rows, got %d", confirmDialogItemCount, rowCount)
		}
	})

	t.Run("Open_Specific_Item_Confirm", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`button[commandfor="confirm-2"][command="show-modal"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.getElementById('confirm-2').open === true`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Open confirm-2 failed: %v", err)
		}
		var otherOpen bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`document.getElementById('confirm-1').open || document.getElementById('confirm-3').open`, &otherOpen)); err != nil {
			t.Fatalf("Sibling dialog state read failed: %v", err)
		}
		if otherOpen {
			t.Error("Sibling confirm dialogs should remain closed")
		}
	})

	t.Run("Cancel_Closes_Without_Deleting", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`dialog#confirm-2 button[command="close"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.getElementById('confirm-2').open === false`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Cancel close failed: %v", err)
		}
		var rowCount int
		if err := chromedp.Run(ctx, chromedp.Evaluate(`document.querySelectorAll('tbody tr[data-key]').length`, &rowCount)); err != nil {
			t.Fatalf("Row count read failed: %v", err)
		}
		if rowCount != confirmDialogItemCount {
			t.Errorf("Expected %d rows after cancel, got %d", confirmDialogItemCount, rowCount)
		}
	})

	t.Run("Confirm_Deletes_Item", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`button[commandfor="confirm-3"][command="show-modal"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.getElementById('confirm-3').open === true`, 5*time.Second),
			chromedp.Click(`dialog#confirm-3 button[name="delete"]`, chromedp.ByQuery),
			e2etest.WaitForCount(`tbody tr[data-key]`, confirmDialogItemCount-1, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Delete via confirm failed: %v", err)
		}
		var rowExists bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`!!document.querySelector('tr[data-key="3"]')`, &rowExists)); err != nil {
			t.Fatalf("Row existence check failed: %v", err)
		}
		if rowExists {
			t.Error("Row with data-key=3 should be removed after delete")
		}
	})

	t.Run("Per_Item_Hash_Link_Opens_Specific_Dialog", func(t *testing.T) {
		// confirm-3 was just deleted, so use confirm-1.
		err := chromedp.Run(ctx,
			chromedp.Navigate(url+"#confirm-1"),
			e2etest.WaitForWebSocketReady(5*time.Second),
			e2etest.WaitFor(`document.getElementById('confirm-1') && document.getElementById('confirm-1').open === true`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Direct hash-link did not open confirm-1: %v", err)
		}
	})

	runStandardSubtests(t, ctx, false, "Confirm Dialog — page heading, table of items each with a Delete button, and one open dialog showing 'Delete \"<name>\"?' confirmation prompt with Cancel and Delete buttons.")
}

// --- Pattern #19: Tabs (HATEOAS) ---

func TestTabs(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/navigation/tabs"

	t.Run("Default_Tab_Is_Overview", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`a[href="?tab=overview"][aria-current="page"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
		)
		if err != nil {
			t.Fatalf("Default tab not Overview: %v", err)
		}
	})

	t.Run("Click_Settings_Tab_Activates_It", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`a[href="?tab=settings"]`, chromedp.ByQuery),
			e2etest.WaitFor(`!!document.querySelector('a[href="?tab=settings"][aria-current="page"]')`, 5*time.Second),
			e2etest.WaitForText(`section h4`, "Settings", 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Settings tab click failed: %v", err)
		}
		var overviewActive bool
		if err := chromedp.Run(ctx, chromedp.Evaluate(`!!document.querySelector('a[href="?tab=overview"][aria-current="page"]')`, &overviewActive)); err != nil {
			t.Fatalf("Overview state read failed: %v", err)
		}
		if overviewActive {
			t.Error("Overview tab should no longer be active after Settings click")
		}
	})

	t.Run("Tab_Switch_Uses_WebSocket_Not_HTTP", func(t *testing.T) {
		// Override window.fetch to count HTTP requests to the tabs URL.
		// The __navigate__ in-band path must not trigger any. t.Cleanup
		// guarantees the restore even if a chromedp step fails mid-flow,
		// so a failure here cannot pollute later subtests.
		t.Cleanup(func() {
			_ = chromedp.Run(ctx, chromedp.Evaluate(`(() => { if (window.__origFetch) { window.fetch = window.__origFetch; delete window.__origFetch; } })()`, nil))
		})
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`(() => {
				window.__navHttpHits = 0;
				window.__origFetch = window.fetch;
				window.fetch = function(input, init) {
					try {
						const u = typeof input === 'string' ? input : input.url;
						if (u && u.includes('/patterns/navigation/tabs')) window.__navHttpHits++;
					} catch (e) {}
					return window.__origFetch.apply(window, arguments);
				};
			})()`, nil),
			chromedp.Click(`a[href="?tab=activity"]`, chromedp.ByQuery),
			e2etest.WaitFor(`!!document.querySelector('a[href="?tab=activity"][aria-current="page"]')`, 5*time.Second),
			e2etest.WaitForText(`section h4`, "Activity", 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Activity tab click failed: %v", err)
		}
		var hits int
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.__navHttpHits`, &hits)); err != nil {
			t.Fatalf("HTTP hit count read failed: %v", err)
		}
		if hits != 0 {
			t.Errorf("Same-pathname tab switch should use WebSocket __navigate__, not HTTP fetch (got %d HTTP hits)", hits)
		}
	})

	t.Run("Direct_URL_Load_Activates_Tab", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url+"?tab=settings"),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`a[href="?tab=settings"][aria-current="page"]`, chromedp.ByQuery),
			e2etest.WaitForText(`section h4`, "Settings", 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Direct URL load with ?tab=settings failed: %v", err)
		}
	})

	t.Run("Invalid_Tab_Falls_Back_To_Default", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url+"?tab=garbage"),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`a[href="?tab=overview"][aria-current="page"]`, chromedp.ByQuery),
		)
		if err != nil {
			t.Fatalf("Invalid tab fallback failed: %v", err)
		}
	})

	runStandardSubtests(t, ctx, false, "Tabs (HATEOAS) — page heading, three-tab nav with the Overview tab marked active, and an Overview content section beneath.")
}

// --- Pattern #20: SPA Navigation ---

func TestSPANavigation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/navigation/spa-navigation"

	t.Run("Initial_Load", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`a[href="?step=1"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			e2etest.WaitForText(`section p strong`, "Step 1 of 3", 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}
	})

	t.Run("Same_Pathname_Step_Update_No_HTTP", func(t *testing.T) {
		t.Cleanup(func() {
			_ = chromedp.Run(ctx, chromedp.Evaluate(`(() => { if (window.__origFetchSPA) { window.fetch = window.__origFetchSPA; delete window.__origFetchSPA; } })()`, nil))
		})
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`(() => {
				window.__spaHttpHits = 0;
				window.__origFetchSPA = window.fetch;
				window.fetch = function(input, init) {
					try {
						const u = typeof input === 'string' ? input : input.url;
						if (u && u.includes('/patterns/navigation/spa-navigation')) window.__spaHttpHits++;
					} catch (e) {}
					return window.__origFetchSPA.apply(window, arguments);
				};
			})()`, nil),
			chromedp.Click(`a[href="?step=2"]`, chromedp.ByQuery),
			e2etest.WaitForText(`section p strong`, "Step 2 of 3", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Step-2 click failed: %v", err)
		}
		var hits int
		if err := chromedp.Run(ctx, chromedp.Evaluate(`window.__spaHttpHits`, &hits)); err != nil {
			t.Fatalf("HTTP hit count read failed: %v", err)
		}
		if hits != 0 {
			t.Errorf("Same-pathname step update should use WebSocket __navigate__, got %d HTTP hits", hits)
		}
	})

	t.Run("External_Link_Has_No_Intercept_Attribute", func(t *testing.T) {
		var hasAttr bool
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`!!document.querySelector('a[href="https://example.com"][lvt-nav\\:no-intercept]')`, &hasAttr),
		)
		if err != nil {
			t.Fatalf("External link attribute check failed: %v", err)
		}
		if !hasAttr {
			t.Error("External example.com link must carry lvt-nav:no-intercept opt-out attribute")
		}
	})

	t.Run("Step_3_Direct_URL_Activates", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url+"?step=3"),
			e2etest.WaitForWebSocketReady(5*time.Second),
			e2etest.WaitForText(`section p strong`, "Step 3 of 3", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Direct ?step=3 load failed: %v", err)
		}
	})

	t.Run("Out_Of_Range_Step_Falls_Back_To_Default", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url+"?step=99"),
			e2etest.WaitForWebSocketReady(5*time.Second),
			e2etest.WaitForText(`section p strong`, "Step 1 of 3", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Out-of-range ?step= did not fall back to Step 1: %v", err)
		}
	})

	runStandardSubtests(t, ctx, false, "SPA Navigation — page heading and three sections: same-pathname step nav with Step indicator, cross-pathname links to other patterns, and an external link section.")
}

// --- Pattern #21: Keyboard Shortcuts ---

func TestKeyboardShortcuts(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/navigation/keyboard-shortcuts"

	t.Run("Initial_Load", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`button[name="open"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}
	})

	t.Run("Open_Button_Click_Opens_Panel", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="open"]`, chromedp.ByQuery),
			e2etest.WaitForText(`h4`, "Command Panel", 5*time.Second),
			chromedp.Click(`button[name="close"]`, chromedp.ByQuery),
			e2etest.WaitForText(`button`, "Open panel", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Open-button Tier-1 fallback failed: %v", err)
		}
	})

	t.Run("Slash_Key_Opens_Panel", func(t *testing.T) {
		// chromedp.KeyEvent delivers to the focused element; lvt-on:window:keydown
		// listens at the window, so we dispatch a synthetic event there.
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.dispatchEvent(new KeyboardEvent('keydown', {key: '/', bubbles: true}))`, nil),
			e2etest.WaitForText(`h4`, "Command Panel", 5*time.Second),
			e2etest.WaitFor(`(() => {
				const items = document.querySelectorAll('ul li small');
				return Array.from(items).some(el => (el.textContent || "").includes("Opened panel"));
			})()`, 3*time.Second),
		)
		if err != nil {
			var html string
			_ = chromedp.Run(ctx, chromedp.OuterHTML(`body`, &html, chromedp.ByQuery))
			t.Fatalf("Slash key did not open panel: %v\nrendered body:\n%s", err, html)
		}
	})

	t.Run("Escape_Closes_Panel", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', bubbles: true}))`, nil),
			e2etest.WaitForText(`button`, "Open panel", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Escape did not close panel: %v", err)
		}
		var logHasClose bool
		// `ul li small` matches the layout's category breadcrumb too, so
		// scan all matches for the "Closed panel" entry rather than relying
		// on the first match.
		if err := chromedp.Run(ctx, chromedp.Evaluate(`Array.from(document.querySelectorAll('ul li small')).some(el => (el.textContent || "").includes('Closed panel'))`, &logHasClose)); err != nil {
			t.Fatalf("Log read failed: %v", err)
		}
		if !logHasClose {
			t.Error("Activity log should contain a 'Closed panel' entry")
		}
	})

	t.Run("Tier1_Form_Fallback_Works", func(t *testing.T) {
		// Re-open via /, then close via the in-panel form button (which
		// works without keyboard or JS as a Tier-1 fallback).
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.dispatchEvent(new KeyboardEvent('keydown', {key: '/', bubbles: true}))`, nil),
			e2etest.WaitForText(`h4`, "Command Panel", 5*time.Second),
			chromedp.Click(`button[name="close"]`, chromedp.ByQuery),
			e2etest.WaitForText(`button`, "Open panel", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Tier-1 form fallback close failed: %v", err)
		}
	})

	runStandardSubtests(t, ctx, false, "Keyboard Shortcuts — page heading with shortcut hints (kbd elements for / and Escape), an 'Open panel' button when closed, and an Activity log with recent open/close entries.")
}
