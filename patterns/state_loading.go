package main

// LazyLoadState holds the state for the Lazy Loading pattern (#14).
type LazyLoadState struct {
	Title    string
	Category string
	Loading  bool
	Data     string
}

// ProgressBarState holds the state for the Progress Bar pattern (#15).
type ProgressBarState struct {
	Title    string
	Category string
	Progress int
	Running  bool
	Done     bool
}

// AsyncOpsState holds the state for the Async Operations pattern (#16).
// Status is a simple state machine: "" (idle), "loading", "success", "error".
type AsyncOpsState struct {
	Title    string
	Category string
	Status   string
	Result   string
	Error    string
}
