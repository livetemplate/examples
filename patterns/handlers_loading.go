package main

import (
	"math/rand"
	"net/http"
	"slices"
	"time"

	"github.com/livetemplate/livetemplate"
)

// --- Pattern #14: Lazy Loading ---

// LazyLoadController spawns a goroutine on OnConnect that pushes the lazily-
// loaded payload via session.TriggerAction after a simulated delay. If the
// client reconnects after the payload has already arrived, OnConnect is a
// no-op so the goroutine does not fire a second time.
type LazyLoadController struct{}

// lazyLoadDelay is how long the simulated "slow API" takes before data arrives.
const lazyLoadDelay = 2 * time.Second

func (c *LazyLoadController) Mount(state LazyLoadState, ctx *livetemplate.Context) (LazyLoadState, error) {
	// Guard: Mount also fires on POST actions (e.g., Reload). Without this,
	// the POST would reset Data/Loading and stomp on the action's own return.
	if ctx.Action() == "" {
		state.Loading = true
		state.Data = ""
	}
	return state, nil
}

func (c *LazyLoadController) OnConnect(state LazyLoadState, ctx *livetemplate.Context) (LazyLoadState, error) {
	// Skip if the data has already arrived (e.g., reconnect after a network
	// hiccup) — re-spawning the goroutine would emit a duplicate update.
	if !state.Loading {
		return state, nil
	}
	// Session is guaranteed non-nil by livetemplate v0.8.18+ (every connect
	// path wires WithSession), but the defensive check stays so a future
	// regression that re-introduces nil sessions surfaces as "no push happens"
	// rather than a panic. Note: if session is nil here, state.Loading
	// remains true and the spinner is shown indefinitely. This is the
	// documented JS-disabled fallback behaviour for the Lazy Loading
	// pattern (see proposal §14) — JS-enabled clients always reach the
	// WebSocket connect path that wires the session.
	session := ctx.Session()
	if session == nil {
		return state, nil
	}
	go func() {
		time.Sleep(lazyLoadDelay)
		_ = session.TriggerAction("dataLoaded", map[string]interface{}{
			"data": "Content loaded lazily at " + time.Now().Format("15:04:05"),
		})
	}()
	return state, nil
}

func (c *LazyLoadController) DataLoaded(state LazyLoadState, ctx *livetemplate.Context) (LazyLoadState, error) {
	state.Data = ctx.GetString("data")
	state.Loading = false
	return state, nil
}

func (c *LazyLoadController) Reload(state LazyLoadState, ctx *livetemplate.Context) (LazyLoadState, error) {
	// Check session BEFORE mutating state. With livetemplate v0.8.18+ this
	// is always non-nil, but the early return ensures the UI does not
	// transition into Loading=true with no goroutine to ever clear it
	// — which would happen if the framework's session wiring regressed.
	session := ctx.Session()
	if session == nil {
		return state, nil
	}
	state.Loading = true
	state.Data = ""
	go func() {
		time.Sleep(lazyLoadDelay)
		_ = session.TriggerAction("dataLoaded", map[string]interface{}{
			"data": "Content reloaded at " + time.Now().Format("15:04:05"),
		})
	}()
	return state, nil
}

func lazyLoadingHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/loading/lazy-loading.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&LazyLoadController{}, livetemplate.AsState(&LazyLoadState{
		Title:    "Lazy Loading",
		Category: "Loading & Progress",
	}))
}

// --- Pattern #15: Progress Bar ---

// ProgressBarController drives a bounded goroutine that ticks progress from
// 10% to 100% in 10% increments every 500ms. The goroutine exits cleanly if
// session.TriggerAction returns an error (session disconnected) — this is the
// canonical cancellation pattern documented in the Server Push pattern (#31).
type ProgressBarController struct{}

const (
	progressStep     = 10
	progressTickRate = 500 * time.Millisecond
)

func (c *ProgressBarController) Start(state ProgressBarState, ctx *livetemplate.Context) (ProgressBarState, error) {
	// Per-session guard against double-click stacking two goroutines.
	if state.Running {
		return state, nil
	}
	// Check session BEFORE setting Running=true. With livetemplate v0.8.18+
	// this is always non-nil, but if it ever became nil the previous
	// ordering (mutate first, check second) would leave Running=true
	// permanently and the Running guard above would block all subsequent
	// Start clicks. Checking first ensures the UI stays interactive.
	session := ctx.Session()
	if session == nil {
		return state, nil
	}
	state.Running = true
	state.Done = false
	state.Progress = 0
	go func() {
		for i := progressStep; i <= 100; i += progressStep {
			time.Sleep(progressTickRate)
			if err := session.TriggerAction("updateProgress", map[string]interface{}{
				"progress": i,
			}); err != nil {
				return // Session disconnected — stop cleanly.
			}
		}
	}()
	return state, nil
}

func (c *ProgressBarController) UpdateProgress(state ProgressBarState, ctx *livetemplate.Context) (ProgressBarState, error) {
	state.Progress = ctx.GetInt("progress")
	if state.Progress >= 100 {
		state.Running = false
		state.Done = true
		ctx.SetFlash("success", "Job complete")
	}
	return state, nil
}

func progressBarHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/loading/progress-bar.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&ProgressBarController{}, livetemplate.AsState(&ProgressBarState{
		Title:    "Progress Bar",
		Category: "Loading & Progress",
	}))
}

// --- Pattern #16: Async Operations ---

// AsyncOpsController implements a loading/success/error state machine. The
// Fetch action transitions to "loading" synchronously, then a goroutine waits
// and pushes a "fetchResult" action with either a success payload or an error
// payload. Demonstrates the minimal state-machine shape you'd use for any
// async RPC (database query, HTTP API, job queue, etc.).
type AsyncOpsController struct{}

const asyncFetchDelay = 2 * time.Second

func (c *AsyncOpsController) Fetch(state AsyncOpsState, ctx *livetemplate.Context) (AsyncOpsState, error) {
	// Check session BEFORE setting Status="loading". With livetemplate
	// v0.8.18+ this is always non-nil, but if it ever became nil the
	// previous ordering (mutate first, check second) would leave the
	// button stuck showing "Fetching..." with no goroutine to clear it.
	session := ctx.Session()
	if session == nil {
		return state, nil
	}
	state.Status = "loading"
	state.Result = ""
	state.Error = ""
	go func() {
		time.Sleep(asyncFetchDelay)
		// Simulated ~33% failure rate. Non-deterministic between runs because
		// Go 1.20+ auto-seeds top-level math/rand from a system source at
		// program startup — no rand.Seed call is needed. Tests must assert
		// {success OR error}, not a specific branch, since either may fire
		// on any given run.
		if rand.Intn(3) == 0 {
			_ = session.TriggerAction("fetchResult", map[string]interface{}{
				"success": false,
				"error":   "Connection timed out",
			})
		} else {
			_ = session.TriggerAction("fetchResult", map[string]interface{}{
				"success": true,
				"result":  "Data fetched successfully at " + time.Now().Format("15:04:05"),
			})
		}
	}()
	return state, nil
}

func (c *AsyncOpsController) FetchResult(state AsyncOpsState, ctx *livetemplate.Context) (AsyncOpsState, error) {
	if ctx.GetBool("success") {
		state.Status = "success"
		state.Result = ctx.GetString("result")
		state.Error = ""
		ctx.SetFlash("success", "Fetch complete")
	} else {
		state.Status = "error"
		state.Error = ctx.GetString("error")
		state.Result = ""
		ctx.SetFlash("error", "Fetch failed")
	}
	return state, nil
}

func asyncOperationsHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/loading/async-operations.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&AsyncOpsController{}, livetemplate.AsState(&AsyncOpsState{
		Title:    "Async Operations",
		Category: "Loading & Progress",
	}))
}
