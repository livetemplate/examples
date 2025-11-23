package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/livetemplate/livetemplate"
	e2etest "github.com/livetemplate/lvt/testing"
)

// Product represents a product in the catalog
type Product struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Price       float64 `json:"price"`
	Image       string  `json:"image"`
	Stock       int     `json:"stock"`
}

// CartItem represents an item in the shopping cart
type CartItem struct {
	Product  Product `json:"product"`
	Quantity int     `json:"quantity"`
}

// Subtotal returns the subtotal for this cart item
func (c CartItem) Subtotal() float64 {
	return c.Product.Price * float64(c.Quantity)
}

// ShopState manages the shopping cart state
type ShopState struct {
	Products    []Product           `json:"products"`
	Cart        map[string]CartItem `json:"cart"`
	Total       float64             `json:"total"`
	ItemCount   int                 `json:"item_count"`
	LastUpdated string              `json:"last_updated"`
	Message     string              `json:"message"`
	MessageType string              `json:"message_type"` // "success", "error", "info"
}

// Change handles all state mutations
func (s *ShopState) Change(ctx *livetemplate.ActionContext) error {
	s.Message = "" // Clear previous messages

	switch ctx.Action {
	case "add_to_cart":
		return s.addToCart(ctx)
	case "remove_from_cart":
		return s.removeFromCart(ctx)
	case "update_quantity":
		return s.updateQuantity(ctx)
	case "clear_cart":
		return s.clearCart()
	case "checkout":
		return s.checkout(ctx)
	default:
		log.Printf("Unknown action: %s", ctx.Action)
		return nil
	}
}

func (s *ShopState) addToCart(ctx *livetemplate.ActionContext) error {
	var payload struct {
		ProductID string `json:"product_id"`
	}
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return err
	}

	// Find the product
	var product *Product
	for _, p := range s.Products {
		if p.ID == payload.ProductID {
			product = &p
			break
		}
	}

	if product == nil {
		s.setMessage("Product not found", "error")
		return nil
	}

	// Check if already in cart
	if item, exists := s.Cart[payload.ProductID]; exists {
		// Check stock
		if item.Quantity >= product.Stock {
			s.setMessage(fmt.Sprintf("Cannot add more %s - only %d in stock", product.Name, product.Stock), "error")
			return nil
		}
		item.Quantity++
		s.Cart[payload.ProductID] = item
		s.setMessage(fmt.Sprintf("Increased %s quantity to %d", product.Name, item.Quantity), "success")
	} else {
		if product.Stock < 1 {
			s.setMessage(fmt.Sprintf("%s is out of stock", product.Name), "error")
			return nil
		}
		s.Cart[payload.ProductID] = CartItem{
			Product:  *product,
			Quantity: 1,
		}
		s.setMessage(fmt.Sprintf("Added %s to cart", product.Name), "success")
	}

	s.recalculate()
	return nil
}

func (s *ShopState) removeFromCart(ctx *livetemplate.ActionContext) error {
	var payload struct {
		ProductID string `json:"product_id"`
	}
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return err
	}

	if item, exists := s.Cart[payload.ProductID]; exists {
		delete(s.Cart, payload.ProductID)
		s.setMessage(fmt.Sprintf("Removed %s from cart", item.Product.Name), "info")
		s.recalculate()
	}

	return nil
}

func (s *ShopState) updateQuantity(ctx *livetemplate.ActionContext) error {
	var payload struct {
		ProductID string `json:"product_id"`
		Quantity  int    `json:"quantity"`
	}
	if err := json.Unmarshal(ctx.Payload, &payload); err != nil {
		return err
	}

	if item, exists := s.Cart[payload.ProductID]; exists {
		if payload.Quantity < 1 {
			delete(s.Cart, payload.ProductID)
			s.setMessage(fmt.Sprintf("Removed %s from cart", item.Product.Name), "info")
		} else if payload.Quantity > item.Product.Stock {
			s.setMessage(fmt.Sprintf("Cannot set quantity to %d - only %d in stock", payload.Quantity, item.Product.Stock), "error")
			return nil
		} else {
			item.Quantity = payload.Quantity
			s.Cart[payload.ProductID] = item
		}
		s.recalculate()
	}

	return nil
}

func (s *ShopState) clearCart() error {
	s.Cart = make(map[string]CartItem)
	s.recalculate()
	s.setMessage("Cart cleared", "info")
	return nil
}

func (s *ShopState) checkout(ctx *livetemplate.ActionContext) error {
	if len(s.Cart) == 0 {
		s.setMessage("Your cart is empty", "error")
		return nil
	}

	// In a real app, this would process the payment
	// For this example, we'll just show a success message
	itemCount := s.ItemCount
	total := s.Total
	s.Cart = make(map[string]CartItem)
	s.recalculate()
	s.setMessage(fmt.Sprintf("Order placed successfully! %d items, total: $%.2f", itemCount, total), "success")
	return nil
}

func (s *ShopState) recalculate() {
	s.Total = 0
	s.ItemCount = 0
	for _, item := range s.Cart {
		s.Total += item.Product.Price * float64(item.Quantity)
		s.ItemCount += item.Quantity
	}
	s.LastUpdated = formatTime()
}

func (s *ShopState) setMessage(msg, msgType string) {
	s.Message = msg
	s.MessageType = msgType
	s.LastUpdated = formatTime()
}

func formatTime() string {
	return time.Now().Format("3:04:05 PM")
}

// GetCartItems returns cart items as a slice for template iteration
func (s *ShopState) GetCartItems() []CartItem {
	items := make([]CartItem, 0, len(s.Cart))
	for _, item := range s.Cart {
		items = append(items, item)
	}
	return items
}

func createSampleProducts() []Product {
	return []Product{
		{
			ID:          "laptop",
			Name:        "Pro Laptop",
			Description: "High-performance laptop with 16GB RAM",
			Price:       1299.99,
			Image:       "💻",
			Stock:       5,
		},
		{
			ID:          "headphones",
			Name:        "Wireless Headphones",
			Description: "Noise-cancelling with 30hr battery",
			Price:       299.99,
			Image:       "🎧",
			Stock:       10,
		},
		{
			ID:          "keyboard",
			Name:        "Mechanical Keyboard",
			Description: "RGB backlit with cherry switches",
			Price:       149.99,
			Image:       "⌨️",
			Stock:       15,
		},
		{
			ID:          "mouse",
			Name:        "Gaming Mouse",
			Description: "16000 DPI with programmable buttons",
			Price:       79.99,
			Image:       "🖱️",
			Stock:       20,
		},
		{
			ID:          "monitor",
			Name:        "4K Monitor",
			Description: "27-inch UHD display with HDR",
			Price:       599.99,
			Image:       "🖥️",
			Stock:       3,
		},
		{
			ID:          "webcam",
			Name:        "HD Webcam",
			Description: "1080p with auto-focus",
			Price:       129.99,
			Image:       "📷",
			Stock:       8,
		},
	}
}

func main() {
	log.Println("LiveTemplate Shopping Cart Server starting...")

	// Load configuration from environment variables
	envConfig, err := livetemplate.LoadEnvConfig()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := envConfig.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Create initial state
	state := &ShopState{
		Products:    createSampleProducts(),
		Cart:        make(map[string]CartItem),
		Total:       0,
		ItemCount:   0,
		LastUpdated: formatTime(),
	}

	// Create template with environment-based configuration
	tmpl := livetemplate.Must(livetemplate.New("shopping-cart", envConfig.ToOptions()...))

	// Mount handler - auto-handles initial page, WebSocket, and HTTP actions
	http.Handle("/", tmpl.Handle(state))

	// Serve client library (development only - use CDN in production)
	http.HandleFunc("/livetemplate-client.js", e2etest.ServeClientLibrary)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Printf("Server starting on http://localhost:%s", port)

	if err := http.ListenAndServe(":"+port, nil); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
