# Shopping Cart Example

A complete shopping cart application demonstrating LiveTemplate's state management, real-time updates, and interactive features. This example showcases common e-commerce patterns with a beautiful, responsive UI.

## Features

- 🛍️ **Product Catalog**: Browse 6 sample tech products with images and descriptions
- 🛒 **Cart Management**: Add, remove, and update quantities in real-time
- 💰 **Live Calculations**: Automatic total and item count updates
- 📊 **Stock Tracking**: Prevents adding more items than available stock
- 💬 **User Feedback**: Real-time success/error messages
- 🎨 **Beautiful UI**: Gradient design with smooth animations
- 📱 **Responsive**: Works on desktop and mobile devices
- ⚡ **Instant Updates**: All changes happen without page reloads

## What This Example Demonstrates

### 1. Complex State Management

```go
type ShopState struct {
    Products    []Product           `json:"products"`
    Cart        map[string]CartItem `json:"cart"`
    Total       float64             `json:"total"`
    ItemCount   int                 `json:"item_count"`
    Message     string              `json:"message"`
}
```

**Shows how to:**
- Manage nested data structures (products, cart items)
- Calculate derived state (totals, counts)
- Handle temporary UI state (messages)

### 2. Action Handling with Payloads

```go
func (s *ShopState) Change(ctx *livetemplate.ActionContext) error {
    switch ctx.Action {
    case "add_to_cart":
        return s.addToCart(ctx)
    case "remove_from_cart":
        return s.removeFromCart(ctx)
    case "update_quantity":
        return s.updateQuantity(ctx)
    }
}
```

**Shows how to:**
- Handle multiple action types
- Parse JSON payloads from user interactions
- Implement business logic in action handlers

### 3. Data Validation & Business Rules

```go
// Check stock before adding
if item.Quantity >= product.Stock {
    s.setMessage(fmt.Sprintf("Only %d in stock", product.Stock), "error")
    return nil
}
```

**Shows how to:**
- Validate user actions
- Enforce business rules (stock limits)
- Provide helpful error messages

### 4. Template Iteration & Conditionals

```html
{{range .Products}}
<div class="product-card">
    <button
        lvt-click="add_to_cart"
        lvt-click-payload='{"product_id": "{{.ID}}"}'
        {{if lt .Stock 1}}disabled{{end}}>
        {{if lt .Stock 1}}Out of Stock{{else}}Add to Cart{{end}}
    </button>
</div>
{{end}}
```

**Shows how to:**
- Iterate over collections
- Use conditional rendering
- Pass data in action payloads
- Disable UI based on state

### 5. Helper Methods for Templates

```go
func (s *ShopState) GetCartItems() []CartItem {
    items := make([]CartItem, 0, len(s.Cart))
    for _, item := range s.Cart {
        items = append(items, item)
    }
    return items
}
```

**Shows how to:**
- Create helper methods for template access
- Convert maps to slices for iteration
- Prepare data for presentation

## Running the Example

### 1. Install Dependencies

```bash
cd shopping-cart
go mod download
```

### 2. Run the Server

```bash
go run main.go
```

The server will start at http://localhost:8080

### 3. Try It Out

1. **Browse Products**: View the 6 tech products in the catalog
2. **Add to Cart**: Click "Add to Cart" on any product
3. **Watch Updates**: See the cart badge and total update instantly
4. **Manage Quantities**: Use the number input to change quantities
5. **Remove Items**: Click "Remove" to take items out of the cart
6. **Test Stock Limits**: Try adding more items than available stock
7. **Checkout**: Click "Checkout" to simulate completing the order
8. **Clear Cart**: Use "Clear Cart" to empty everything at once

## Architecture Patterns

### State Management

This example demonstrates a clean state management pattern:

```
User Action → Change() → Action Handler → Update State → Recalculate() → UI Update
```

**Benefits:**
- Single source of truth (ShopState)
- Predictable state updates
- Easy to test and debug
- Automatic UI synchronization

### Separation of Concerns

```go
// Business logic
func (s *ShopState) addToCart(ctx *livetemplate.ActionContext) error {
    // Validation, state updates, calculations
}

// UI feedback
func (s *ShopState) setMessage(msg, msgType string) {
    s.Message = msg
    s.MessageType = msgType
}

// Calculations
func (s *ShopState) recalculate() {
    s.Total = 0
    s.ItemCount = 0
    // ... calculate totals
}
```

**Benefits:**
- Clear responsibilities
- Reusable functions
- Easy to maintain

## File Structure

```
shopping-cart/
├── main.go                      # Server code with state management
├── shopping-cart.tmpl           # HTML template with cart UI
├── shopping_cart_e2e_test.go   # E2E tests with Chromedp
├── test_main_test.go            # Test setup and cleanup
├── go.mod                       # Dependencies
└── README.md                    # This file
```

## Testing

### Run E2E Tests

```bash
go test -v
```

The tests verify:
- ✅ Initial page load and empty cart state
- ✅ Product catalog display (6 products)
- ✅ WebSocket connection
- ✅ Adding items to cart
- ✅ Cart display and updates
- ✅ LiveTemplate wrapper preservation

### Run Quick Tests Only

```bash
go test -v -short
```

## Key Concepts Demonstrated

### 1. **Real-time State Synchronization**
   - Every action immediately updates the UI
   - No manual DOM manipulation needed
   - LiveTemplate handles the synchronization

### 2. **Complex Data Structures**
   - Nested objects (Product, CartItem)
   - Collections (arrays, maps)
   - Derived state (totals, counts)

### 3. **User Feedback**
   - Success messages (item added)
   - Error messages (out of stock)
   - Info messages (cart cleared)
   - Auto-dismissing message system

### 4. **Form Inputs**
   - Number inputs for quantities
   - Change event handling
   - Input validation
   - Dynamic min/max values

### 5. **Conditional UI**
   - Empty cart state
   - Populated cart state
   - Disabled buttons (out of stock)
   - Message type styling

## Common E-commerce Patterns

This example implements patterns you'll find in real shopping carts:

1. **Add to Cart**: One-click adding with stock validation
2. **Quantity Management**: Direct input or increment/decrement
3. **Remove Items**: Simple removal from cart
4. **Cart Summary**: Running totals and item counts
5. **Stock Limits**: Preventing over-ordering
6. **User Feedback**: Clear messaging for all actions
7. **Checkout Flow**: Simulated order completion

## Extending This Example

Want to add more features? Here are some ideas:

### Easy Extensions

1. **Product Search**: Add a search box to filter products
   ```go
   SearchQuery string `json:"search_query"`
   ```

2. **Category Filter**: Group products by category
   ```go
   Category string // Add to Product struct
   SelectedCategory string // Add to ShopState
   ```

3. **Coupon Codes**: Apply discounts
   ```go
   CouponCode string
   Discount float64
   ```

### Medium Extensions

4. **Product Variants**: Size, color, etc.
   ```go
   type Variant struct {
       Size  string
       Color string
       SKU   string
   }
   ```

5. **Wishlist**: Save items for later
   ```go
   Wishlist map[string]Product
   ```

6. **Order History**: Track completed orders
   ```go
   type Order struct {
       ID        string
       Items     []CartItem
       Total     float64
       Timestamp time.Time
   }
   ```

### Advanced Extensions

7. **Database Integration**: Persist cart across sessions
8. **User Accounts**: Multiple carts per user
9. **Payment Processing**: Integrate Stripe/PayPal
10. **Inventory Management**: Real-time stock updates

## Best Practices Shown

✅ **Clear Action Names**: `add_to_cart`, not `act1`
✅ **Validation First**: Check stock before updating state
✅ **User Feedback**: Always show what happened
✅ **Defensive Coding**: Handle edge cases (empty cart, out of stock)
✅ **Separation of Concerns**: Business logic separate from presentation
✅ **Helper Methods**: Make templates cleaner with helper functions
✅ **Type Safety**: Strong typing with Go structs
✅ **E2E Testing**: Verify the complete user experience

## Performance Considerations

This example is optimized for:

- **Fast Updates**: Only changed parts of the DOM are updated
- **Minimal State**: Only essential data is stored
- **Efficient Calculations**: Recalculate only when needed
- **Small Payloads**: JSON payloads are minimal

## Learn More

- [LiveTemplate Documentation](https://github.com/livetemplate/livetemplate)
- [Other Examples](../)
- [Counter Example](../counter/) - Simpler state management
- [Todos Example](../todos/) - CRUD operations with database
- [Chat Example](../chat/) - Multi-user interactions

## Troubleshooting

**Cart not updating?**
- Check browser console for errors
- Verify WebSocket connection (green indicator)
- Look for JavaScript errors in DevTools

**Actions not working?**
- Ensure buttons have correct `lvt-click` attributes
- Check payload JSON is valid
- Verify action names match Change() switch cases

**Stock validation not working?**
- Check stock values in `createSampleProducts()`
- Verify validation logic in `addToCart()`
- Look for error messages in the UI

## Credits

Built with ❤️ using [LiveTemplate v0.3.0](https://github.com/livetemplate/livetemplate)

## License

MIT License - see [LICENSE](../LICENSE) for details
