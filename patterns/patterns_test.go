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

// --- Sortable List ---

func TestSortable(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/lists/sortable"

	// CDP Input.dispatchMouseEvent is unreliable for HTML5 DnD in headless
	// Docker Chrome, so we dispatch real DragEvent objects with a shared
	// DataTransfer instead. This still exercises the full client
	// delegation pipeline — not a liveTemplateClient.send() shortcut.
	simulateDrag := func(srcKey, tgtKey string) chromedp.Action {
		js := fmt.Sprintf(`
			(() => {
				const src = document.querySelector('#sortable-list li[data-key=%q]');
				const tgt = document.querySelector('#sortable-list li[data-key=%q]');
				if (!src || !tgt) throw new Error('source or target not found');
				const dt = new DataTransfer();
				src.dispatchEvent(new DragEvent('dragstart', {bubbles:true, cancelable:true, dataTransfer:dt}));
				tgt.dispatchEvent(new DragEvent('dragover',  {bubbles:true, cancelable:true, dataTransfer:dt}));
				tgt.dispatchEvent(new DragEvent('drop',      {bubbles:true, cancelable:true, dataTransfer:dt}));
			})()
		`, srcKey, tgtKey)
		return chromedp.Evaluate(js, nil)
	}

	// Reset the demo's shared in-memory order at the start. The controller's
	// state is process-wide so other tests (or a previous run of this test
	// in dev) could leave the list reordered.
	t.Run("Initial_Reset", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`#sortable-list`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			e2etest.WaitForCount(`#sortable-list li[data-key]`, 6, 5*time.Second),
			chromedp.Click(`button[name="reset"]`, chromedp.ByQuery),
			e2etest.WaitFor(
				`document.querySelectorAll('#sortable-list li')[0].dataset.key === 'task-1'`,
				5*time.Second,
			),
		)
		if err != nil {
			t.Fatalf("Failed to load + reset: %v", err)
		}
	})

	runStandardSubtests(t, ctx, false, "Sortable List — six task items each with a hamburger drag handle, in default order, plus a Reset Order button")

	// resetToInitial restores the canonical task-1..task-6 order so each
	// reorder subtest starts from a known state and doesn't depend on the
	// previous one's outcome.
	resetToInitial := chromedp.Tasks{
		chromedp.Click(`button[name="reset"]`, chromedp.ByQuery),
		e2etest.WaitFor(
			`document.querySelectorAll('#sortable-list li')[0].dataset.key === 'task-1' && document.querySelectorAll('#sortable-list li')[5].dataset.key === 'task-6'`,
			5*time.Second,
		),
	}

	t.Run("Reorder_DragForward", func(t *testing.T) {
		// Initial: [task-1, task-2, task-3, task-4, task-5, task-6]
		// Drag task-1 onto task-3 with insert-before-target semantics:
		// task-1 is removed from index 0, the post-removal target index of
		// task-3 is 1, and task-1 is inserted at index 1.
		// Expected:  [task-2, task-1, task-3, task-4, task-5, task-6]
		var order string
		err := chromedp.Run(ctx,
			resetToInitial,
			simulateDrag("task-1", "task-3"),
			e2etest.WaitFor(
				`document.querySelectorAll('#sortable-list li')[1].dataset.key === 'task-1'`,
				5*time.Second,
			),
			chromedp.Evaluate(
				`Array.from(document.querySelectorAll('#sortable-list li')).map(el => el.dataset.key).join(',')`,
				&order,
			),
		)
		if err != nil {
			t.Fatalf("Forward drag failed: %v", err)
		}
		want := "task-2,task-1,task-3,task-4,task-5,task-6"
		if order != want {
			t.Errorf("Order after forward drag: got %q, want %q", order, want)
		}
	})

	t.Run("Reorder_DragBackward", func(t *testing.T) {
		// Initial: [task-1, task-2, task-3, task-4, task-5, task-6]
		// Drag task-6 onto task-2: task-6 is removed from index 5, no
		// post-removal index adjustment (srcIdx > tgtIdx), task-6 inserted
		// at task-2's index 1.
		// Expected: [task-1, task-6, task-2, task-3, task-4, task-5]
		var order string
		err := chromedp.Run(ctx,
			resetToInitial,
			simulateDrag("task-6", "task-2"),
			e2etest.WaitFor(
				`document.querySelectorAll('#sortable-list li')[1].dataset.key === 'task-6'`,
				5*time.Second,
			),
			chromedp.Evaluate(
				`Array.from(document.querySelectorAll('#sortable-list li')).map(el => el.dataset.key).join(',')`,
				&order,
			),
		)
		if err != nil {
			t.Fatalf("Backward drag failed: %v", err)
		}
		want := "task-1,task-6,task-2,task-3,task-4,task-5"
		if order != want {
			t.Errorf("Order after backward drag: got %q, want %q", order, want)
		}
	})

	t.Run("SelfDrop_NoOp", func(t *testing.T) {
		// The controller short-circuits when source == target, so no diff
		// is emitted. We can't condition-wait on a state that should NOT
		// change, so we wait long enough for any spurious server-side
		// reorder to round-trip (~500ms) and assert order is unchanged.
		var orderBefore string
		if err := chromedp.Run(ctx,
			resetToInitial,
			chromedp.Evaluate(
				`Array.from(document.querySelectorAll('#sortable-list li')).map(el => el.dataset.key).join(',')`,
				&orderBefore,
			),
		); err != nil {
			t.Fatalf("Failed to read order before self-drop: %v", err)
		}

		firstKey := strings.Split(orderBefore, ",")[0]
		if err := chromedp.Run(ctx, simulateDrag(firstKey, firstKey)); err != nil {
			t.Fatalf("Self-drop dispatch failed: %v", err)
		}

		// time.Sleep (Go-side) is fine for negative assertions — the
		// CLAUDE.md "no chromedp.Sleep" rule is about browser-side waits
		// that hide timing bugs in positive assertions.
		time.Sleep(500 * time.Millisecond)

		var orderAfter string
		if err := chromedp.Run(ctx, chromedp.Evaluate(
			`Array.from(document.querySelectorAll('#sortable-list li')).map(el => el.dataset.key).join(',')`,
			&orderAfter,
		)); err != nil {
			t.Fatalf("Failed to read order after self-drop: %v", err)
		}
		if orderAfter != orderBefore {
			t.Errorf("Self-drop changed order: was %q, now %q", orderBefore, orderAfter)
		}
	})

	t.Run("Reset_RestoresInitialOrder", func(t *testing.T) {
		// After all the prior reorders, hit Reset and assert order is
		// back to task-1..task-6.
		var order string
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="reset"]`, chromedp.ByQuery),
			e2etest.WaitFor(
				`document.querySelectorAll('#sortable-list li')[0].dataset.key === 'task-1'`,
				5*time.Second,
			),
			chromedp.Evaluate(
				`Array.from(document.querySelectorAll('#sortable-list li')).map(el => el.dataset.key).join(',')`,
				&order,
			),
		)
		if err != nil {
			t.Fatalf("Reset failed: %v", err)
		}
		want := "task-1,task-2,task-3,task-4,task-5,task-6"
		if order != want {
			t.Errorf("Order after reset: got %q, want %q", order, want)
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
		// 10 × 500ms = 5s. The intermediate-tick assertion (value > 0 AND
		// value < 100) catches a regression where the goroutine skips
		// intermediate ticks and jumps straight to 100 — a value > 0 check
		// alone would also be satisfied by an instant 100, missing the bug.
		// 5s timeout matches the goroutine's full duration so on loaded CI
		// runners we still catch a real value < 100 before completion.
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="start"]`, chromedp.ByQuery),
			e2etest.WaitFor(`!!document.querySelector('progress')`, 3*time.Second),
			e2etest.WaitFor(`document.querySelector('progress') && document.querySelector('progress').value > 0 && document.querySelector('progress').value < 100`, 5*time.Second),
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

	t.Run("Brief_Disconnect_Within_Retry_Window_Completes", func(t *testing.T) {
		// Reload to clean state. Click Start. Once the timer is mid-flight,
		// force-disconnect the WebSocket via the client's public API. The
		// server-side ticker will retry session.TriggerAction for ~5s; if we
		// reconnect within that window, the timer must continue and
		// complete to 100%.
		err := chromedp.Run(ctx,
			chromedp.Reload(),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`button[name="start"]`, chromedp.ByQuery),
			chromedp.Click(`button[name="start"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('progress') && document.querySelector('progress').value >= 20`, 3*time.Second),
			// Force-disconnect; this is the same disconnect() the
			// visibility-reconnect path uses, so the server treats it
			// identically to an iOS-killed connection.
			chromedp.Evaluate(`window.liveTemplateClient.disconnect()`, nil),
			// Wall-clock sleep: we're deliberately leaving the connection
			// down inside the goroutine's 5s retry window to verify it
			// survives the gap. There's no client-observable signal
			// during the retry loop (the goroutine's TriggerAction errors
			// silently), so a condition-based wait wouldn't fit here.
			chromedp.Sleep(1*time.Second),
			chromedp.Evaluate(`window.liveTemplateClient.connect()`, nil),
			e2etest.WaitForWebSocketReady(5*time.Second),
			// Run continues. Wait for completion.
			e2etest.WaitForText(`button`, "Run Again", 10*time.Second),
		)
		if err != nil {
			t.Fatalf("Brief-disconnect run did not complete: %v", err)
		}
	})

	t.Run("Long_Disconnect_Beyond_Retry_Window_Settles_Without_Impossible_State", func(t *testing.T) {
		// Reload to clean state. Click Start. Mid-flight, disconnect for
		// 7s — past the goroutine's 5s retry budget. The goroutine must
		// give up; on reconnect the page must NOT display a corrupted
		// state (Done=true with Progress<100, or Running=true with no
		// goroutine to advance it). Acceptable settled states: clean
		// "Start Job" button (Running=false, Done=false) OR completed
		// "Run Again" with Progress=100.
		err := chromedp.Run(ctx,
			chromedp.Reload(),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`button[name="start"]`, chromedp.ByQuery),
			chromedp.Click(`button[name="start"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('progress') && document.querySelector('progress').value >= 20`, 3*time.Second),
			chromedp.Evaluate(`window.liveTemplateClient.disconnect()`, nil),
			// chromedp.Sleep is intentional — we're waiting wall-clock
			// for the goroutine's 5s retry budget to expire.
			chromedp.Sleep(7*time.Second),
			chromedp.Evaluate(`window.liveTemplateClient.connect()`, nil),
			e2etest.WaitForWebSocketReady(5*time.Second),
			// Wait for a stable settled state.
			e2etest.WaitFor(`(() => {
				const btn = document.querySelector('button[name="start"]');
				if (!btn) return false;
				return btn.textContent.includes('Start Job') || btn.textContent.includes('Run Again');
			})()`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Long-disconnect run did not settle: %v", err)
		}

		// Invariant: if "Run Again" is shown (Done=true), progress must be 100.
		// If "Start Job" is shown (Running=false, Done=false), no progress
		// element should be present.
		var settled struct {
			HasRunAgain bool `json:"hasRunAgain"`
			HasStartJob bool `json:"hasStartJob"`
			HasProgress bool `json:"hasProgress"`
			ProgressVal int  `json:"progressVal"`
		}
		err = chromedp.Run(ctx, chromedp.Evaluate(`(() => {
			const btn = document.querySelector('button[name="start"]');
			const p = document.querySelector('progress');
			return {
				hasRunAgain: btn && btn.textContent.includes('Run Again'),
				hasStartJob: btn && btn.textContent.includes('Start Job'),
				hasProgress: !!p,
				progressVal: p ? Number(p.value) : 0,
			};
		})()`, &settled))
		if err != nil {
			t.Fatalf("Could not read settled state: %v", err)
		}
		if settled.HasRunAgain && settled.ProgressVal != 100 {
			t.Errorf("Impossible state: Run Again button shown but progress=%d (must be 100)", settled.ProgressVal)
		}
		if settled.HasStartJob && settled.HasProgress {
			t.Errorf("Impossible state: Start Job button shown but progress element still present")
		}
		if !settled.HasRunAgain && !settled.HasStartJob {
			t.Error("Impossible state: neither Run Again nor Start Job button shown after long disconnect")
		}
	})

	t.Run("Multiple_Disconnect_Cycles_Never_Produce_Impossible_State", func(t *testing.T) {
		// Run Again, then disconnect/reconnect rapidly several times during
		// the run. After settled, verify the same invariant: Done=true →
		// Progress=100. This is the regression test for the bug where Mount
		// revival spawned a competing goroutine that overwrote Progress with
		// a stale value AFTER another goroutine had set Done=true.
		err := chromedp.Run(ctx,
			chromedp.Reload(),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`button[name="start"]`, chromedp.ByQuery),
			chromedp.Click(`button[name="start"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('progress') && document.querySelector('progress').value >= 10`, 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Could not start the timer: %v", err)
		}

		// Three disconnect/reconnect cycles. Each cycle: capture progress
		// before, fire disconnect+connect back-to-back, wait for the
		// reconnect to land, then wait for the goroutine to make further
		// progress (or for the run to complete) before the next cycle —
		// avoids fixed-duration sleeps that flake on slow CI.
		for i := 0; i < 3; i++ {
			var beforeProgress int
			err := chromedp.Run(ctx, chromedp.Evaluate(
				`(()=>{const p=document.querySelector('progress');return p?Number(p.value):0;})()`,
				&beforeProgress,
			))
			if err != nil {
				t.Fatalf("Cycle %d: could not read pre-cycle progress: %v", i, err)
			}
			err = chromedp.Run(ctx,
				chromedp.Evaluate(`window.liveTemplateClient.disconnect(); window.liveTemplateClient.connect()`, nil),
				e2etest.WaitForWebSocketReady(3*time.Second),
				e2etest.WaitFor(fmt.Sprintf(`(() => {
					const btn = document.querySelector('button[name="start"]');
					if (btn && btn.textContent.includes('Run Again')) return true;
					const p = document.querySelector('progress');
					return p && Number(p.value) > %d;
				})()`, beforeProgress), 5*time.Second),
			)
			if err != nil {
				t.Fatalf("Disconnect cycle %d failed: %v", i, err)
			}
		}

		// Wait for a stable final state.
		err = chromedp.Run(ctx,
			e2etest.WaitFor(`(() => {
				const btn = document.querySelector('button[name="start"]');
				if (!btn) return false;
				return btn.textContent.includes('Start Job') || btn.textContent.includes('Run Again');
			})()`, 15*time.Second),
		)
		if err != nil {
			t.Fatalf("Did not settle after disconnect cycles: %v", err)
		}

		var settled struct {
			HasRunAgain bool `json:"hasRunAgain"`
			HasStartJob bool `json:"hasStartJob"`
			ProgressVal int  `json:"progressVal"`
		}
		err = chromedp.Run(ctx, chromedp.Evaluate(`(() => {
			const btn = document.querySelector('button[name="start"]');
			const p = document.querySelector('progress');
			return {
				hasRunAgain: btn && btn.textContent.includes('Run Again'),
				hasStartJob: btn && btn.textContent.includes('Start Job'),
				progressVal: p ? Number(p.value) : 0,
			};
		})()`, &settled))
		if err != nil {
			t.Fatalf("Could not read settled state: %v", err)
		}
		// The core invariant: if the UI advertises completion (Run Again
		// button), progress must be a true 100. The screenshot reproducer
		// for this bug showed Run Again next to a 70% bar.
		if settled.HasRunAgain && settled.ProgressVal != 100 {
			t.Errorf("Impossible state after disconnect cycles: Run Again button shown but progress=%d", settled.ProgressVal)
		}
	})

	t.Run("Done_State_Survives_Reconnect_Via_Persist", func(t *testing.T) {
		// Progress and Done are lvt:"persist" — a completed run must stay
		// completed across a disconnect/reconnect cycle. The user keeps the
		// "Run Again" button rather than snapping back to Start Job.
		err := chromedp.Run(ctx,
			chromedp.Reload(),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`button[name="start"]`, chromedp.ByQuery),
			chromedp.Click(`button[name="start"]`, chromedp.ByQuery),
			e2etest.WaitForText(`button`, "Run Again", 10*time.Second),
			// Disconnect+reconnect back-to-back; WaitForWebSocketReady
			// synchronises on the new connection rather than a fixed sleep.
			chromedp.Evaluate(`window.liveTemplateClient.disconnect(); window.liveTemplateClient.connect()`, nil),
			e2etest.WaitForWebSocketReady(5*time.Second),
			// Run Again must still be there.
			e2etest.WaitForText(`button`, "Run Again", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Done state did not persist across reconnect: %v", err)
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

func TestAnimations(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/feedback/animations"

	t.Run("Initial_Load", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`select[name="mode"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}
	})

	t.Run("Add_Plays_Fade_Animation", func(t *testing.T) {
		// Click Add with default mode "fade". The directive sets
		// element.style.animation = "lvt-fade-in 500ms ease-out" on the new
		// row. The keystone assertion is "directive applied the keyframe"
		// — cleanup is verified by the Existing_Rows subtest below, which
		// asserts items 1 and 2 have no inline style after a third add.
		var anim string
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="add"]`, chromedp.ByQuery),
			e2etest.WaitFor(`!!document.querySelector('li[data-key="item-1"]')`, 3*time.Second),
			chromedp.Evaluate(`document.querySelector('li[data-key="item-1"]').style.animation`, &anim),
		)
		if err != nil {
			t.Fatalf("Add did not produce item-1: %v", err)
		}
		if !strings.Contains(anim, "lvt-fade-in") {
			t.Errorf("expected style.animation to contain 'lvt-fade-in', got %q", anim)
		}
	})

	t.Run("Mode_Switch_Affects_New_Rows", func(t *testing.T) {
		// Switch select to "slide" via DOM (the value is form-submitted with
		// Add). The next row's `lvt-fx:animate="{{$.Mode}}"` resolves to slide,
		// so style.animation should contain lvt-slide-in. After the render,
		// the select must still show "slide" (server state echoed via the
		// `selected` attribute).
		var anim, selectVal string
		err := chromedp.Run(ctx,
			chromedp.SetValue(`select[name="mode"]`, "slide", chromedp.ByQuery),
			chromedp.Click(`button[name="add"]`, chromedp.ByQuery),
			e2etest.WaitFor(`!!document.querySelector('li[data-key="item-2"]')`, 3*time.Second),
			chromedp.Evaluate(`document.querySelector('li[data-key="item-2"]').style.animation`, &anim),
			chromedp.Evaluate(`document.querySelector('select[name="mode"]').value`, &selectVal),
		)
		if err != nil {
			t.Fatalf("Slide-mode add failed: %v", err)
		}
		if !strings.Contains(anim, "lvt-slide-in") {
			t.Errorf("expected style.animation to contain 'lvt-slide-in', got %q", anim)
		}
		if selectVal != "slide" {
			t.Errorf("mode select did not retain 'slide' across re-render, got %q", selectVal)
		}
	})

	t.Run("Existing_Rows_Do_Not_Re_animate", func(t *testing.T) {
		// Wait for item-2 animation to complete, then add item-3. The WeakSet
		// guard in directives.ts must prevent items 1 and 2 from re-animating.
		// Note: the 3s timeout assumes the default 500ms `--lvt-animate-duration`.
		// If a future page overrides that variable to >3s, this test will flake.
		var item1Style, item2Style string
		err := chromedp.Run(ctx,
			e2etest.WaitFor(`document.querySelector('li[data-key="item-2"]').style.animation === ""`, 5*time.Second),
			chromedp.Click(`button[name="add"]`, chromedp.ByQuery),
			e2etest.WaitFor(`!!document.querySelector('li[data-key="item-3"]')`, 3*time.Second),
			chromedp.Evaluate(`document.querySelector('li[data-key="item-1"]').style.animation || ""`, &item1Style),
			chromedp.Evaluate(`document.querySelector('li[data-key="item-2"]').style.animation || ""`, &item2Style),
		)
		if err != nil {
			t.Fatalf("Re-animate guard test setup failed: %v", err)
		}
		if item1Style != "" {
			t.Errorf("item-1 style.animation should be empty after second add (WeakSet guard), got %q", item1Style)
		}
		if item2Style != "" {
			t.Errorf("item-2 style.animation should be empty after third add (WeakSet guard), got %q", item2Style)
		}
		// Wait for item-3 to finish before standard subtests so they don't
		// see a transient inline-style on a [data-key] element.
		_ = chromedp.Run(ctx, e2etest.WaitFor(`document.querySelector('li[data-key="item-3"]').style.animation === ""`, 5*time.Second))
	})

	runStandardSubtests(t, ctx, true, "Animations pattern — heading, a row with a mode select (fade/slide/scale) and Add Item button, and a list of three added items each labeled with its mode.")
}

func TestLoadingStates(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/feedback/loading-states"

	t.Run("Initial_Load", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`section:nth-of-type(1) button[name="slowSave"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}
	})

	t.Run("Tier1_Fieldset_Disabled_During_Pending", func(t *testing.T) {
		// Submit Tier 1 form with typed input. While the action is in flight
		// (2s sleep), the fieldset should carry `disabled` and the form
		// should be aria-busy. After completion, both reset AND the input
		// auto-clears (default form-reset behavior; would need `lvt-form:preserve`
		// to retain).
		var inputValue string
		err := chromedp.Run(ctx,
			chromedp.SendKeys(`section:nth-of-type(1) input[name="data"]`, "hello", chromedp.ByQuery),
			chromedp.Click(`section:nth-of-type(1) button[name="slowSave"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('section:nth-of-type(1) fieldset').disabled === true`, 1*time.Second),
			e2etest.WaitFor(`document.querySelector('section:nth-of-type(1) form').getAttribute('aria-busy') === 'true'`, 1*time.Second),
			e2etest.WaitFor(`document.querySelector('section:nth-of-type(1) fieldset').disabled === false`, 8*time.Second),
			chromedp.Evaluate(`document.querySelector('section:nth-of-type(1) input[name="data"]').value`, &inputValue),
		)
		if err != nil {
			t.Fatalf("Tier 1 fieldset auto-disable failed: %v", err)
		}
		if inputValue != "" {
			t.Errorf("Tier 1 input did not auto-reset after submit, got %q", inputValue)
		}
	})

	t.Run("DisableWith_Replaces_Button_Text", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`section:nth-of-type(2) button[name="slowSave"]`, chromedp.ByQuery),
			e2etest.WaitForText(`section:nth-of-type(2) button[name="slowSave"]`, "Saving", 1*time.Second),
			e2etest.WaitForText(`section:nth-of-type(2) button[name="slowSave"]`, "Save", 5*time.Second),
		)
		if err != nil {
			t.Fatalf("disable-with text replacement failed: %v", err)
		}
	})

	t.Run("SetAttr_Pending_Sets_AriaBusy", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`section:nth-of-type(3) button[name="slowSave"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('section:nth-of-type(3) button[name="slowSave"]').getAttribute('aria-busy') === 'true'`, 1*time.Second),
			e2etest.WaitFor(`document.querySelector('section:nth-of-type(3) button[name="slowSave"]').getAttribute('aria-busy') === 'false'`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("setAttr:on:pending toggle failed: %v", err)
		}
	})

	t.Run("Submit_Updates_LastSave", func(t *testing.T) {
		// Multiple <small> elements on the page (breadcrumb + section descriptions),
		// so check the document body's text.
		err := chromedp.Run(ctx,
			e2etest.WaitForText(`body`, "Last save:", 8*time.Second),
		)
		if err != nil {
			t.Fatalf("Last save indicator never appeared: %v", err)
		}
	})

	runStandardSubtests(t, ctx, true, "Loading States pattern — three sections (Tier 1 automatic, Tier 2 disable-with, Tier 2 setAttr) each with an input and Save button, plus a 'Last save:' timestamp below.")
}

func TestHighlightOnChange(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/feedback/highlight"

	t.Run("Initial_Load", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}
	})

	// UI_Standards is intentionally NOT run for this pattern. The highlight
	// directive's whole job is to add an inline background-color and
	// transition for the visual flash, which fires on every render walk that
	// touches the subtree — including the first paint. Even with the
	// post-cycle attribute cleanup landed in client v0.8.37
	// (livetemplate/client#100), the in-flight state during the ~550ms cycle
	// still has a non-empty `style` attribute that the [style] CSP rule would
	// flag if UI_Standards happened to sample mid-cycle. The "no inline
	// styles" rule isn't a meaningful guarantee for a pattern whose entire
	// premise is inline styling — Visual_Check plus the interaction subtests
	// below cover the right behavior.

	t.Run("Increment_Flashes_Both_Highlight_Targets", func(t *testing.T) {
		// directives.ts sets style.transition for ~550ms per render-touch.
		// The transition assertion has a wider polling window than the
		// bg-color one (which clears at 50ms) and is just as load-bearing
		// — the transition IS the visual flash. Use Array.from to count
		// only the inner highlight cards (page wrappers don't carry the
		// directive). The 5s WaitFor budget is generous; with the lvt
		// chrome-throttling fix (livetemplate/lvt#314, v0.1.4) the polled
		// approach reliably lands inside the directive's window.
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitFor(`(() => {
				const els = Array.from(document.querySelectorAll('[lvt-fx\\:highlight]'));
				if (els.length < 2) return false;
				return els.every(el => (el.style.transition || "").includes('background-color'));
			})()`, 5*time.Second),
		)
		if err != nil {
			t.Fatalf("Highlight transition not applied to both targets: %v", err)
		}
	})

	t.Run("Highlight_Cleans_Up_After_Duration", func(t *testing.T) {
		// directives.ts highlight cycle: 50ms delay + 500ms transition. The
		// WaitFor polls until both bg + transition are clear, with a 2s
		// budget that comfortably covers the 550ms cycle.
		err := chromedp.Run(ctx,
			e2etest.WaitFor(`(() => {
				const els = Array.from(document.querySelectorAll('[lvt-fx\\:highlight]'));
				return els.every(el => el.style.backgroundColor === '' && el.style.transition === '');
			})()`, 2*time.Second),
		)
		if err != nil {
			t.Fatalf("Highlight did not clean up: %v", err)
		}
	})

	t.Run("Counter_Increments_On_Both_Mirrors", func(t *testing.T) {
		// Counter is at 1 from the prior `Increment_Flashes_Both_Highlight_Targets`
		// subtest; this click brings it to 2. Both mirrors must reflect the
		// shared `.Counter` value.
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitForText(`body`, "Counter A: 2", 3*time.Second),
			e2etest.WaitForText(`body`, "Counter B (mirror): 2", 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Counter increment not reflected on mirrors: %v", err)
		}
	})

	t.Run("Visual_Check", func(t *testing.T) {
		e2etest.ValidateScreenshotWithLLM(t, ctx, "Highlight on Change pattern — heading, an Increment button, and two cards 'Counter A' and 'Counter B (mirror)' both showing the same number 2.")
	})
}

func TestFlashMessages(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/feedback/flash-messages"

	t.Run("Initial_Load", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`button[name="save"]`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
		)
		if err != nil {
			t.Fatalf("Initial load failed: %v", err)
		}
	})

	t.Run("Empty_Save_Shows_Error_Flash", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="save"]`, chromedp.ByQuery),
			e2etest.WaitForText(`output[data-flash="error"]`, "Name is required", 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Empty save did not surface error flash: %v", err)
		}
	})

	t.Run("Valid_Save_Shows_Success_Clears_Error", func(t *testing.T) {
		var nameValue string
		err := chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="name"]`, "Ada", chromedp.ByQuery),
			chromedp.Click(`button[name="save"]`, chromedp.ByQuery),
			e2etest.WaitForText(`output[data-flash="success"]`, "Saved: Ada", 3*time.Second),
			// Error flash should be gone (ClearFlash("error") in the controller).
			e2etest.WaitFor(`!document.querySelector('output[data-flash="error"]')`, 3*time.Second),
			// Form auto-resets on success — the name input must be cleared.
			chromedp.Evaluate(`document.querySelector('input[name="name"]').value`, &nameValue),
		)
		if err != nil {
			t.Fatalf("Valid save did not clear error / set success: %v", err)
		}
		if nameValue != "" {
			t.Errorf("name input did not auto-reset after successful save, got %q", nameValue)
		}
	})

	t.Run("Notify_Persists_Until_Dismiss", func(t *testing.T) {
		// 2s idle is enough to prove persistence — info has no FlashExpiry,
		// so any prune timer would have fired by now if one existed. Wall-
		// clock waits like this can't be condition-based (proving absence-
		// of-change), but 2s is comfortably below the success-flash 5s
		// expiry from the previous subtest.
		err := chromedp.Run(ctx,
			chromedp.Click(`button[name="notify"]`, chromedp.ByQuery),
			e2etest.WaitForText(`output[data-flash="info"]`, "Heads up", 3*time.Second),
			chromedp.Sleep(2*time.Second),
			e2etest.WaitFor(`!!document.querySelector('output[data-flash="info"]')`, 1*time.Second),
			chromedp.Click(`button[name="dismissNotify"]`, chromedp.ByQuery),
			e2etest.WaitFor(`!document.querySelector('output[data-flash="info"]')`, 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Notify→Dismiss lifecycle failed: %v", err)
		}
	})

	t.Run("Success_AutoExpires_After_FlashExpiry", func(t *testing.T) {
		// FlashExpiry is render-driven: pruneExpiredFlash runs inside
		// getMessages before the snapshot, so the next render after the
		// deadline ships clean HTML. Wait past the expiry, click Notify
		// to trigger a render, and assert success is gone in the same
		// response that introduces info.
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('input[name="name"]').value = ""`, nil),
			chromedp.SendKeys(`input[name="name"]`, "Bob", chromedp.ByQuery),
			chromedp.Click(`button[name="save"]`, chromedp.ByQuery),
			e2etest.WaitForText(`output[data-flash="success"]`, "Saved: Bob", 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Could not seed a success flash: %v", err)
		}
		// chromedp.Sleep is unavoidable here — we're literally waiting on
		// wall-clock for the FlashExpiry deadline to elapse. Sleep past 5s
		// (the expiry duration), then click Notify to trigger a render.
		err = chromedp.Run(ctx,
			chromedp.Sleep(5500*time.Millisecond),
			chromedp.Click(`button[name="notify"]`, chromedp.ByQuery),
			e2etest.WaitForText(`output[data-flash="info"]`, "Heads up", 3*time.Second),
			e2etest.WaitFor(`!document.querySelector('output[data-flash="success"]')`, 3*time.Second),
		)
		if err != nil {
			t.Fatalf("Success flash did not auto-expire after FlashExpiry: %v", err)
		}
	})

	// pico=false: page uses {{.lvt.FlashTag}}, which renders <output data-flash>;
	// the Pico validator (chrome.go:967) prefers <ins>/<del>, but this pattern
	// IS the FlashTag demo, so the standard subtests use the non-Pico variant.
	runStandardSubtests(t, ctx, false, "Flash Messages pattern — heading, two forms (one with a Name input + Save, one with Notify and Dismiss buttons), and an info flash 'Heads up — this stays until you dismiss it' visible.")
}

// --- Pattern #26: Multi-User Sync ---

func TestMultiUserSync(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/realtime/multi-user-sync"

	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		e2etest.WaitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`button[name="increment"]`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("Tab 1 initial load failed: %v", err)
	}

	// chromedp.NewContext(parent) where parent is a chromedp context creates
	// a NEW TAB in the same browser. Cookies and storage are shared, so both
	// tabs land in the same session group — the prerequisite for Sync()
	// auto-dispatch (mount.go:1466-1468) to fire across them.
	peerCtx, peerCancel := chromedp.NewContext(ctx)
	defer peerCancel()
	if err := chromedp.Run(peerCtx,
		chromedp.Navigate(url),
		e2etest.WaitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`button[name="increment"]`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("Peer tab initial load failed: %v", err)
	}

	t.Run("Increment_Tab1_Updates_Both", func(t *testing.T) {
		if err := chromedp.Run(ctx,
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Counter: 1", 3*time.Second),
		); err != nil {
			t.Fatalf("Tab 1 did not reflect Counter: 1: %v", err)
		}
		// Peer must see the same value via Sync auto-dispatch — Increment
		// did NOT call BroadcastAction; Sync fires unconditionally because
		// HasSync && !syncExplicitlyBroadcast at mount.go:1466.
		if err := chromedp.Run(peerCtx,
			e2etest.WaitForText(`article`, "Counter: 1", 3*time.Second),
		); err != nil {
			t.Fatalf("Peer did not pick up Counter: 1 from Sync auto-dispatch: %v", err)
		}
	})

	t.Run("Increment_Tab2_Updates_Both", func(t *testing.T) {
		if err := chromedp.Run(peerCtx,
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Counter: 2", 3*time.Second),
		); err != nil {
			t.Fatalf("Peer did not reflect Counter: 2 after its own click: %v", err)
		}
		if err := chromedp.Run(ctx,
			e2etest.WaitForText(`article`, "Counter: 2", 3*time.Second),
		); err != nil {
			t.Fatalf("Tab 1 did not pick up Counter: 2 from peer Sync: %v", err)
		}
	})

	t.Run("Late_Joiner_Sees_Current_Counter_On_Mount", func(t *testing.T) {
		// Counter is at 2 from the prior subtests. A new tab opening
		// AFTER the increments must see 2 immediately on its initial
		// render — not 0 with a wait for the next peer action's Sync.
		// This guards the MultiUserSyncController.Mount() call (without
		// it, the late joiner would render Counter:0 from zero-value
		// state until a peer action fired Sync).
		lateCtx, lateCancel := chromedp.NewContext(ctx)
		defer lateCancel()
		if err := chromedp.Run(lateCtx,
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Counter: 2", 3*time.Second),
		); err != nil {
			t.Fatalf("Late-joining tab did not see Counter: 2 on mount: %v", err)
		}
	})

	runStandardSubtests(t, ctx, true, "Multi-User Sync pattern — heading, a paragraph 'Counter: 2', and an Increment button. Layout is centered with Pico styling.")
}

// --- Pattern #27: Broadcasting ---

func TestBroadcasting(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/realtime/broadcasting"

	// Tab 1 Joins as Alice.
	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		e2etest.WaitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.SendKeys(`input[name="username"]`, "Alice", chromedp.ByQuery),
		chromedp.Click(`button[name="join"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`button[name="send"]`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("Tab 1 join failed: %v", err)
	}

	// Peer tab Joins as Bob. Username is intentionally NOT lvt:"persist"
	// (state_realtime.go) so the second tab gets its own join form even
	// though it shares the session-group cookie with tab 1.
	peerCtx, peerCancel := chromedp.NewContext(ctx)
	defer peerCancel()
	if err := chromedp.Run(peerCtx,
		chromedp.Navigate(url),
		e2etest.WaitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.SendKeys(`input[name="username"]`, "Bob", chromedp.ByQuery),
		chromedp.Click(`button[name="join"]`, chromedp.ByQuery),
		chromedp.WaitVisible(`button[name="send"]`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("Peer tab join failed: %v", err)
	}

	t.Run("Send_From_Tab1_Appears_In_Peer", func(t *testing.T) {
		var textVal string
		if err := chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="text"]`, "hi from Alice", chromedp.ByQuery),
			chromedp.Click(`button[name="send"]`, chromedp.ByQuery),
			e2etest.WaitForText(`div.messages`, "hi from Alice", 3*time.Second),
			// CLAUDE.md E2E #4: assert form fields cleared after submit.
			// The compose form has no lvt-form:preserve, so the text input
			// resets to empty after a successful Send re-renders the form.
			chromedp.Evaluate(`document.querySelector('input[name="text"]').value`, &textVal),
		); err != nil {
			t.Fatalf("Tab 1 did not see its own message: %v", err)
		}
		if textVal != "" {
			t.Errorf("text input did not reset after Send, got %q", textVal)
		}
		if err := chromedp.Run(peerCtx,
			e2etest.WaitForText(`div.messages`, "hi from Alice", 3*time.Second),
		); err != nil {
			t.Fatalf("Peer did not receive broadcast from tab 1: %v", err)
		}
	})

	t.Run("Send_From_Peer_Appears_In_Tab1", func(t *testing.T) {
		var textVal string
		if err := chromedp.Run(peerCtx,
			chromedp.SendKeys(`input[name="text"]`, "hi from Bob", chromedp.ByQuery),
			chromedp.Click(`button[name="send"]`, chromedp.ByQuery),
			e2etest.WaitForText(`div.messages`, "hi from Bob", 3*time.Second),
			chromedp.Evaluate(`document.querySelector('input[name="text"]').value`, &textVal),
		); err != nil {
			t.Fatalf("Peer did not see its own message: %v", err)
		}
		if textVal != "" {
			t.Errorf("peer text input did not reset after Send, got %q", textVal)
		}
		if err := chromedp.Run(ctx,
			e2etest.WaitForText(`div.messages`, "hi from Bob", 3*time.Second),
		); err != nil {
			t.Fatalf("Tab 1 did not receive broadcast from peer: %v", err)
		}
	})

	t.Run("Empty_Send_Appends_Nothing", func(t *testing.T) {
		// Testing "no change after time T" without a wall-clock Sleep:
		// fire the empty Send first, then a known-good "guard" Send, and
		// wait for the guard message to appear. By the time the guard's
		// render lands, the empty Send's no-op response (queued before
		// the guard) has been processed too, so any spurious append from
		// the empty would already be in the DOM. The final count must be
		// baseline + 1 (the guard) — anything else means the empty Send
		// appended.
		var countBefore int
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('div.messages p[data-key]').length`, &countBefore),
		); err != nil {
			t.Fatalf("Could not count messages: %v", err)
		}
		const guardText = "guard message"
		var countAfter int
		// Both sends go via liveTemplateClient.send rather than the form UI
		// because the empty submit's transient pending state races with the
		// next click's SendKeys ("Element is not focusable"). This is the
		// same idiom TestAsyncOperations.Concurrent_Fetch_Reaches_Single_Result
		// uses (patterns_test.go around line 1750) for the analogous
		// "two-sends-in-a-row, observe one render" pattern. The behavior
		// being tested is the empty-input branch of Send returning no-op
		// state — that path is exercised identically whether the client
		// fires it via form submit or via send(), and the protocol-level
		// helper is the only way to avoid the focus race here without a
		// wall-clock Sleep.
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(fmt.Sprintf(`(() => {
				window.liveTemplateClient.send({action: 'send', data: {text: ''}});
				window.liveTemplateClient.send({action: 'send', data: {text: %q}});
			})()`, guardText), nil),
			e2etest.WaitForText(`div.messages`, guardText, 3*time.Second),
			chromedp.Evaluate(`document.querySelectorAll('div.messages p[data-key]').length`, &countAfter),
		); err != nil {
			t.Fatalf("Empty/guard send sequence failed: %v", err)
		}
		if countAfter != countBefore+1 {
			t.Errorf("Empty send appended a message: before=%d after=%d (expected before+1=%d for guard only)", countBefore, countAfter, countBefore+1)
		}
	})

	t.Run("Empty_Username_Join_Is_NoOp", func(t *testing.T) {
		// Targeted protocol test for the Join handler's empty-username
		// guard. The HTML `required` attribute on the username input
		// stops empty submission via the UI; the server-side guard is
		// defense-in-depth for protocol-level clients. Same guard-message
		// idiom as Empty_Send_Appends_Nothing: fire empty + guard, wait
		// for the guard's effect, conclude empty had no effect.
		if err := chromedp.Run(peerCtx,
			// Re-navigate to get a fresh join form. Peer was previously
			// joined as Bob and sent messages; navigating resets its
			// per-connection state so we can test the empty-username path
			// against an unjoined client.
			chromedp.Navigate(url),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
			chromedp.Evaluate(`(() => {
				window.liveTemplateClient.send({action: 'join', data: {username: ''}});
				window.liveTemplateClient.send({action: 'join', data: {username: 'GuardBob'}});
			})()`, nil),
			// Guard's Join sets state.Username, swapping the join form
			// out for the compose form. If the empty-username send had
			// gone through first and set state.Username = "", the swap
			// would still happen on the guard — but if the empty had
			// somehow set Username to "" AFTER the guard, we'd never
			// see "Posting as GuardBob".
			e2etest.WaitForText(`article`, "Posting as GuardBob", 3*time.Second),
			chromedp.WaitVisible(`button[name="send"]`, chromedp.ByQuery),
		); err != nil {
			t.Fatalf("Empty/guard join sequence failed: %v", err)
		}
	})

	runStandardSubtests(t, ctx, true, "Broadcasting pattern — heading, 'Posting as Alice' label, message list with three entries (one from Alice, one from Bob, and a 'guard message' from Alice), and a compose form with a text input + Send button.")
}

// --- Pattern #28: Presence Tracking ---

func TestPresence(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/realtime/presence"

	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		e2etest.WaitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.SendKeys(`input[name="username"]`, "Alice", chromedp.ByQuery),
		chromedp.Click(`button[name="join"]`, chromedp.ByQuery),
		e2etest.WaitForText(`mark`, "1 user(s) online", 3*time.Second),
	); err != nil {
		t.Fatalf("Tab 1 Alice join failed: %v", err)
	}

	peerCtx, peerCancel := chromedp.NewContext(ctx)
	defer peerCancel()
	if err := chromedp.Run(peerCtx,
		chromedp.Navigate(url),
		e2etest.WaitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`input[name="username"]`, chromedp.ByQuery),
		chromedp.SendKeys(`input[name="username"]`, "Bob", chromedp.ByQuery),
		chromedp.Click(`button[name="join"]`, chromedp.ByQuery),
		e2etest.WaitForText(`mark`, "2 user(s) online", 3*time.Second),
	); err != nil {
		t.Fatalf("Peer Bob join failed: %v", err)
	}

	t.Run("Tab1_Sees_Two_After_Peer_Joins", func(t *testing.T) {
		if err := chromedp.Run(ctx,
			e2etest.WaitForText(`mark`, "2 user(s) online", 3*time.Second),
		); err != nil {
			t.Fatalf("Tab 1 did not see updated count after peer joined: %v", err)
		}
	})

	t.Run("Tab1_Leave_Decrements_Both", func(t *testing.T) {
		if err := chromedp.Run(ctx,
			chromedp.Click(`button[name="leave"]`, chromedp.ByQuery),
			e2etest.WaitForText(`mark`, "1 user(s) online", 3*time.Second),
		); err != nil {
			t.Fatalf("Tab 1 leave did not decrement local count: %v", err)
		}
		if err := chromedp.Run(peerCtx,
			e2etest.WaitForText(`mark`, "1 user(s) online", 3*time.Second),
		); err != nil {
			t.Fatalf("Peer did not see decrement after tab 1 leave: %v", err)
		}
	})

	t.Run("Peer_Leave_Goes_To_Zero", func(t *testing.T) {
		if err := chromedp.Run(peerCtx,
			chromedp.Click(`button[name="leave"]`, chromedp.ByQuery),
			e2etest.WaitForText(`mark`, "0 user(s) online", 3*time.Second),
		); err != nil {
			t.Fatalf("Peer leave did not decrement local count: %v", err)
		}
		if err := chromedp.Run(ctx,
			e2etest.WaitForText(`mark`, "0 user(s) online", 3*time.Second),
		); err != nil {
			t.Fatalf("Tab 1 did not see final decrement after peer leave: %v", err)
		}
	})

	t.Run("Empty_Username_Join_Is_NoOp", func(t *testing.T) {
		// Targeted protocol test for Join's empty-username guard. The
		// HTML `required` attribute is the UI guard; this test covers
		// the server-side defense-in-depth path. Same guard-message
		// idiom as Broadcasting: fire empty + guard, wait for the
		// guard's effect — if empty had been honored, OnlineCount
		// would have ticked to 1 before the guard's Join fired and we'd
		// never see exactly 1 user.
		//
		// Run on peerCtx (which is at the join form post-Peer_Leave) and
		// have the guard user Leave at the end so ctx ends in the
		// canonical "0 user(s) online + join form" state for
		// runStandardSubtests' Visual_Check.
		if err := chromedp.Run(peerCtx,
			chromedp.Evaluate(`(() => {
				window.liveTemplateClient.send({action: 'join', data: {username: ''}});
				window.liveTemplateClient.send({action: 'join', data: {username: 'GuardX'}});
			})()`, nil),
			e2etest.WaitForText(`mark`, "1 user(s) online", 3*time.Second),
			// Cleanup: peer leaves so ctx ends at "0 user(s) online".
			chromedp.Click(`button[name="leave"]`, chromedp.ByQuery),
			e2etest.WaitForText(`mark`, "0 user(s) online", 3*time.Second),
		); err != nil {
			t.Fatalf("Empty/guard join sequence failed: %v", err)
		}
		if err := chromedp.Run(ctx,
			e2etest.WaitForText(`mark`, "0 user(s) online", 3*time.Second),
		); err != nil {
			t.Fatalf("ctx did not see GuardX leave broadcast: %v", err)
		}
	})

	runStandardSubtests(t, ctx, true, "Presence Tracking pattern — heading, a highlighted '0 user(s) online' indicator, and a join form with a username input + Join button.")
}

// --- Pattern #29: Reconnection Recovery ---

func TestReconnection(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/realtime/reconnection"

	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		e2etest.WaitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`button[name="increment"]`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("Initial load failed: %v", err)
	}

	t.Run("Counter_And_Notes_Survive_Reload", func(t *testing.T) {
		if err := chromedp.Run(ctx,
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Counter: 1", 3*time.Second),
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Counter: 2", 3*time.Second),
			chromedp.Click(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Counter: 3", 3*time.Second),
			chromedp.SendKeys(`textarea[name="notes"]`, "persisted hello", chromedp.ByQuery),
			chromedp.Click(`button[name="saveNotes"]`, chromedp.ByQuery),
			e2etest.WaitFor(`document.querySelector('textarea[name="notes"]').value === "persisted hello"`, 3*time.Second),
		); err != nil {
			t.Fatalf("Pre-reload setup failed: %v", err)
		}

		// Reload — fresh HTTP GET re-mounts via the session-group cookie;
		// the framework restores Counter and Notes from the session store
		// before the first render.
		var notesValue string
		if err := chromedp.Run(ctx,
			chromedp.Reload(),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`button[name="increment"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Counter: 3", 3*time.Second),
			chromedp.Evaluate(`document.querySelector('textarea[name="notes"]').value`, &notesValue),
		); err != nil {
			t.Fatalf("Reload + restore failed: %v", err)
		}
		if notesValue != "persisted hello" {
			t.Errorf("Notes not restored after reload, got %q", notesValue)
		}
	})

	// pico=false: the page uses a vertical labeled <textarea> form, which
	// doesn't fit Pico's input+button-in-fieldset[role=group] convention.
	// runUIStandardsWithPico would flag the form as a Pico violation.
	runStandardSubtests(t, ctx, false, "Reconnection Recovery pattern — heading, 'Counter: 3' display with persistence note, an Increment button, and a notes textarea pre-filled with 'persisted hello' plus a Save Notes button.")
}

// --- Pattern #30: Live Preview ---

func TestLivePreview(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/realtime/live-preview"

	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		e2etest.WaitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`input[name="input"]`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("Initial load failed: %v", err)
	}

	t.Run("Type_Updates_Preview_After_Debounce", func(t *testing.T) {
		// 300ms debounce per client/constants.ts:DEFAULT_CHANGE_DEBOUNCE_MS;
		// 3s slack covers debounce + WS round-trip + render comfortably.
		if err := chromedp.Run(ctx,
			chromedp.SendKeys(`input[name="input"]`, "World", chromedp.ByQuery),
			e2etest.WaitForText(`#preview`, "Hello, World!", 3*time.Second),
		); err != nil {
			t.Fatalf("Preview did not update after typing: %v", err)
		}
	})

	t.Run("Submit_Commits_Input_To_State", func(t *testing.T) {
		if err := chromedp.Run(ctx,
			chromedp.Click(`button[name="submit"]`, chromedp.ByQuery),
			e2etest.WaitForText(`#preview`, "Saved: World", 3*time.Second),
		); err != nil {
			t.Fatalf("Submit did not commit input: %v", err)
		}
		var val string
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelector('input[name="input"]').value`, &val),
		); err != nil {
			t.Fatalf("Could not read input value: %v", err)
		}
		if val != "World" {
			t.Errorf("input value after submit: want %q, got %q", "World", val)
		}
	})

	runStandardSubtests(t, ctx, true, "Live Preview pattern — heading, a Name input pre-filled with 'World' + Save button, and an output element showing 'Saved: World'.")
}

// --- Pattern #31: Server Push ---

func TestServerPush(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping E2E test in short mode")
	}

	ctx, cancel, serverPort := setupTest(t)
	defer cancel()

	url := e2etest.GetChromeTestURL(serverPort) + "/patterns/realtime/server-push"

	if err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		e2etest.WaitForWebSocketReady(5*time.Second),
		chromedp.WaitVisible(`button[name="startTimer"]`, chromedp.ByQuery),
	); err != nil {
		t.Fatalf("Initial load failed: %v", err)
	}

	t.Run("Start_Switches_To_Running_View", func(t *testing.T) {
		// Click Start. The handler sets state.Running=true synchronously and
		// spawns the timer goroutine; the response render swaps the page to
		// the "Timer running" view immediately. Asserting only that view
		// switch (not intermediate Elapsed values) is deliberate — see the
		// next subtest's comment for why per-tick assertions are unreliable
		// from this same chromedp ctx.
		if err := chromedp.Run(ctx,
			chromedp.Click(`button[name="startTimer"]`, chromedp.ByQuery),
			e2etest.WaitForText(`article`, "Timer running", 3*time.Second),
		); err != nil {
			t.Fatalf("Start did not switch to running view: %v", err)
		}
	})

	t.Run("Second_Start_Is_Idempotent", func(t *testing.T) {
		// StartTimer's `if state.Running { return state, nil }` guard
		// prevents spawning a second goroutine on a duplicate start. The
		// rendered Start button is hidden while running, so the only way
		// to attempt a duplicate start is via the protocol-level helper —
		// this is a targeted regression test for the guard, the kind of
		// case CLAUDE.md endorses for liveTemplateClient.send usage.
		// After the second send, the page must still show the timer
		// running view (single goroutine, single state.Running flip).
		var hasStartButton bool
		if err := chromedp.Run(ctx,
			chromedp.Evaluate(`window.liveTemplateClient.send({action: 'startTimer'})`, nil),
			e2etest.WaitForText(`article`, "Timer running", 3*time.Second),
			chromedp.Evaluate(`!!document.querySelector('button[name="startTimer"]')`, &hasStartButton),
		); err != nil {
			t.Fatalf("Second StartTimer dispatch failed: %v", err)
		}
		if hasStartButton {
			t.Error("Start button is visible while timer is running — guard didn't hold")
		}
	})

	t.Run("Completes_With_Done_Message", func(t *testing.T) {
		// "Last completed: 10s" requires state.Elapsed=10 (set by the 10th
		// Tick action with elapsed=10) AND state.Running=false (set by
		// TimerDone). The full goroutine cycle (10×1s ticks + TimerDone)
		// takes ~10s of wall-clock; 14s gives comfortable slack.
		//
		// Why no per-tick assertion: chromedp's tight Go-side polling for
		// intermediate values competes with the browser main thread for
		// morphdom-application time. Server-side log probing has shown the
		// goroutine fires all 10 ticks correctly at 1Hz, but pushed renders
		// during a tight WaitFor poll often don't surface in the DOM until
		// polling stops. Asserting the FINAL state proves the full cycle
		// (every Tick was processed AND TimerDone fired) without depending
		// on intermediate-value visibility.
		if err := chromedp.Run(ctx,
			e2etest.WaitForText(`article`, "Last completed: 10s", 14*time.Second),
			chromedp.WaitVisible(`button[name="startTimer"]`, chromedp.ByQuery),
		); err != nil {
			t.Fatalf("Timer did not complete: %v", err)
		}
	})

	// pico=false: the Start Timer button is a single-button form (no
	// adjacent input), which doesn't trigger Pico's fieldset[role=group]
	// rule but the validator's heuristic flags any non-grouped form as
	// a Pico violation. runUIStandards (without Pico) covers the rest.
	runStandardSubtests(t, ctx, false, "Server Push pattern — heading, a Start Timer button, and a 'Last completed: 10s' note shown below it.")
}
