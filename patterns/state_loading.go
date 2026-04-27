package main

// LazyLoadState holds the state for the Lazy Loading pattern (#14).
type LazyLoadState struct {
	Title    string
	Category string
	Loading  bool
	Data     string
}

// ProgressBarState holds the state for the Progress Bar pattern (#15).
//
// Progress and Done are persisted so a completed run's outcome
// survives a brief reconnect (e.g. mobile app-switching). Running is
// NOT persisted: a stale Running=true with no goroutine to advance it
// would leave the UI on aria-busy forever, the failure mode Pattern
// #31 explicitly avoids. The ticker goroutine retries TriggerAction
// for a bounded window before exiting, so brief backgrounds (<~5s)
// are recovered by the goroutine itself; longer disconnects exit and
// the user sees a clean "Start Job" button via the persisted but
// non-Running state.
type ProgressBarState struct {
	Title    string
	Category string
	Progress int `lvt:"persist"`
	Running  bool
	Done     bool `lvt:"persist"`
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
