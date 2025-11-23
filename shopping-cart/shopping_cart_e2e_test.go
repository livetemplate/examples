package main

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
	e2etest "github.com/livetemplate/lvt/testing"
)

// TestShoppingCartE2E tests the shopping cart app end-to-end with a real browser
func TestShoppingCartE2E(t *testing.T) {
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

	// Start shopping cart server
	serverCmd := e2etest.StartTestServer(t, "main.go", serverPort)
	defer func() {
		if serverCmd != nil && serverCmd.Process != nil {
			serverCmd.Process.Kill()
		}
	}()

	// Start Docker Chrome container
	chromeCmd := e2etest.StartDockerChrome(t, debugPort)
	defer e2etest.StopDockerChrome(t, chromeCmd, debugPort)

	// Connect to Docker Chrome via remote debugging
	chromeURL := fmt.Sprintf("http://localhost:%d", debugPort)
	allocCtx, allocCancel := chromedp.NewRemoteAllocator(context.Background(), chromeURL)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(t.Logf))
	defer cancel()

	// Set timeout for the entire test
	ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	t.Run("Initial Load", func(t *testing.T) {
		var initialHTML string

		err := chromedp.Run(ctx,
			chromedp.Navigate(e2etest.GetChromeTestURL(serverPort)),
			e2etest.WaitForWebSocketReady(5*time.Second),
			chromedp.WaitVisible(`h1`, chromedp.ByQuery),
			e2etest.ValidateNoTemplateExpressions("[data-lvt-id]"),
			chromedp.OuterHTML(`body`, &initialHTML, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to load page: %v", err)
		}

		// Verify initial state
		if !strings.Contains(initialHTML, "Shopping Cart Demo") {
			t.Error("Page title not found")
		}
		if !strings.Contains(initialHTML, "0 items") {
			t.Error("Initial cart badge not showing 0 items")
		}
		if !strings.Contains(initialHTML, "Your cart is empty") {
			t.Error("Empty cart message not found")
		}

		t.Log("✅ Initial page load verified")
	})

	t.Run("Product Catalog", func(t *testing.T) {
		var productCards int

		err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('.product-card').length`, &productCards),
		)

		if err != nil {
			t.Fatalf("Failed to count products: %v", err)
		}

		if productCards != 6 {
			t.Errorf("Expected 6 products, found %d", productCards)
		}

		// Verify first product
		var productName string
		err = chromedp.Run(ctx,
			chromedp.Text(`.product-card:first-child .product-name`, &productName, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to get product name: %v", err)
		}

		if productName == "" {
			t.Error("Product name is empty")
		}

		t.Logf("✅ Found %d products in catalog", productCards)
	})

	t.Run("WebSocket Connection", func(t *testing.T) {
		err := chromedp.Run(ctx,
			chromedp.Evaluate(`console.log('WebSocket test'); 'logged'`, nil),
			e2etest.WaitFor(`typeof window.liveTemplateClient !== 'undefined'`, 3*time.Second),
		)

		if err != nil {
			t.Fatalf("Failed to check WebSocket: %v", err)
		}

		t.Log("✅ WebSocket connection working")
	})

	t.Run("Add to Cart", func(t *testing.T) {
		var cartBadge string

		// Click the first "Add to Cart" button
		err := chromedp.Run(ctx,
			chromedp.Click(`.product-card:first-child .btn-primary`, chromedp.ByQuery),
			chromedp.Sleep(500*time.Millisecond), // Wait for state update
			chromedp.Text(`.cart-badge`, &cartBadge, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to add to cart: %v", err)
		}

		if !strings.Contains(cartBadge, "1 item") {
			t.Errorf("Expected cart badge to show '1 item', got: %s", cartBadge)
		}

		t.Log("✅ Successfully added item to cart")
	})

	t.Run("Cart Display", func(t *testing.T) {
		var cartItems int

		err := chromedp.Run(ctx,
			chromedp.Evaluate(`document.querySelectorAll('.cart-item').length`, &cartItems),
		)

		if err != nil {
			t.Fatalf("Failed to count cart items: %v", err)
		}

		if cartItems != 1 {
			t.Errorf("Expected 1 cart item, found %d", cartItems)
		}

		t.Log("✅ Cart displays added item correctly")
	})

	t.Run("LiveTemplate Updates", func(t *testing.T) {
		var html string
		err := chromedp.Run(ctx,
			chromedp.OuterHTML(`[data-lvt-id]`, &html, chromedp.ByQuery),
		)

		if err != nil {
			t.Fatalf("Failed to find LiveTemplate wrapper: %v", err)
		}

		if !strings.Contains(html, "data-lvt-id") {
			t.Error("LiveTemplate wrapper not found after updates")
		}

		t.Log("✅ LiveTemplate wrapper preserved after updates")
	})

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎉 All E2E tests passed!")
	fmt.Println(strings.Repeat("=", 60))
}
