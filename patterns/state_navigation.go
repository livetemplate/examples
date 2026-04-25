package main

type ModalDialogState struct {
	Title    string
	Category string
	Name     string
	Email    string
	SavedAt  string
}

type ConfirmDialogState struct {
	Title    string
	Category string
	Items    []Item
}

type TabsState struct {
	Title     string
	Category  string
	ActiveTab string
}

type SPANavState struct {
	Title    string
	Category string
	Step     int
}

type ShortcutsState struct {
	Title     string
	Category  string
	PanelOpen bool
	Log       []string
}
