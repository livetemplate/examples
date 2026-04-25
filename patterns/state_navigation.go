package main

// ModalDialogState holds the state for the Modal Dialog pattern (#17).
type ModalDialogState struct {
	Title    string
	Category string
	Name     string
	Email    string
	SavedAt  string
}

// ConfirmDialogState holds the state for the Confirm Dialog pattern (#18).
type ConfirmDialogState struct {
	Title    string
	Category string
	Items    []Item
}

// TabsState holds the state for the Tabs pattern (#19).
// ActiveTab is one of: "overview", "settings", "activity".
type TabsState struct {
	Title     string
	Category  string
	ActiveTab string
}

// SPANavState holds the state for the SPA Navigation pattern (#20).
// Step is clamped to [1, 3] from the ?step= query parameter.
type SPANavState struct {
	Title    string
	Category string
	Step     int
}

// ShortcutsState holds the state for the Keyboard Shortcuts pattern (#21).
// Log retains the last shortcutsLogMax entries (FIFO).
type ShortcutsState struct {
	Title     string
	Category  string
	PanelOpen bool
	Log       []string
}
