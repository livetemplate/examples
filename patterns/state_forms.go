package main

// ClickToEditState holds the state for the Click To Edit pattern (#1).
type ClickToEditState struct {
	Title     string
	Category  string
	FirstName string
	LastName  string
	Email     string
	Editing   bool
}

// EditRowState holds the state for the Edit Row pattern (#2).
type EditRowState struct {
	Title     string
	Category  string
	Contacts  []Contact
	EditingID string
}

// InlineValidationState holds the state for the Inline Validation pattern (#3).
type InlineValidationState struct {
	Title    string
	Category string
	Email    string
	Username string
	Saved    bool
}

// BulkUpdateState holds the state for the Bulk Update pattern (#4).
type BulkUpdateState struct {
	Title    string
	Category string
	Users    []UserRow
}

// ResetInputState holds the state for the Reset User Input pattern (#5).
type ResetInputState struct {
	Title    string
	Category string
	Messages []string
}

// FileUploadState holds the state for the File Upload pattern (#6).
type FileUploadState struct {
	Title    string
	Category string
}

// PreserveInputsState holds the state for the Preserving File Inputs pattern (#7).
type PreserveInputsState struct {
	Title       string
	Category    string
	Name        string
	Description string
}
