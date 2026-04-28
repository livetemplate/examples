package main

// DeleteRowState holds the state for the Delete Row pattern (#8).
type DeleteRowState struct {
	Title    string
	Category string
	Items    []Item
}

// ClickToLoadState holds the state for the Click To Load pattern (#9).
type ClickToLoadState struct {
	Title       string
	Category    string
	Items       []Item
	CurrentPage int
	HasMore     bool
}

// InfiniteScrollState holds the state for the Infinite Scroll pattern (#10).
type InfiniteScrollState struct {
	Title       string
	Category    string
	Items       []Item
	CurrentPage int
	HasMore     bool
}

// ValueSelectState holds the state for the Value Select pattern (#11).
type ValueSelectState struct {
	Title    string
	Category string
	Makes    []string
	Models   []string
	Make     string
	Model    string
}

// SortableState holds the state for the Sortable List pattern (#12).
type SortableState struct {
	Title    string
	Category string
	Items    []SortableItem
}

// SortableItem is a keyed task in the Sortable List pattern. Key drives
// data-key in the template, which is what the diff engine and the
// drag-and-drop client use to identify which item moved.
type SortableItem struct {
	Key  string
	Name string
}
