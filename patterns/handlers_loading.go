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
	// path wires WithSession). The defensive check stays so a future
	// framework regression surfaces as "no push happens" rather than a
	// panic — but it should NOT be confused with the JS-disabled fallback.
	// JS-disabled clients never reach OnConnect at all (no WebSocket = no
	// OnConnect call); the JS-disabled spinner-forever case is created by
	// Mount() returning Loading=true on the initial HTTP GET. The nil
	// branch here is purely a defensive guard against framework bugs.
	session := ctx.Session()
	if session == nil {
		return state, nil
	}
	// Reconnect-during-loading note: if the client disconnects and
	// reconnects within the 2s window, OnConnect fires again and spawns
	// a second goroutine while the first is still asleep. Both goroutines
	// dispatch via groupID lookup (registry.GetByGroup), and groupID is
	// stable across reconnects (cookie-bound), so when each goroutine
	// wakes one of two things happens:
	//   (a) The reconnect hasn't completed yet → GetByGroup returns no
	//       connections → TriggerAction returns "no connected sessions"
	//       → goroutine exits via the cancellation pattern below.
	//   (b) The reconnect has completed → both goroutines successfully
	//       dispatch to the new connection. DataLoaded runs twice with
	//       slightly different timestamps; the second call overwrites
	//       Data. This is harmless — the user just sees the timestamp
	//       update once. Loading=false is idempotent.
	// No explicit dedup guard is needed for this demo. Production code
	// that absolutely requires single-flight semantics should track the
	// in-flight request ID in state and check it inside DataLoaded.
	go func() {
		time.Sleep(lazyLoadDelay)
		if err := session.TriggerAction("dataLoaded", map[string]any{
			"data": "Content loaded lazily at " + time.Now().Format("15:04:05"),
		}); err != nil {
			return // Session disconnected — stop cleanly.
		}
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
	// No explicit Running-style guard here (unlike ProgressBarController.Start)
	// because the template hides the Reload button while Loading=true, so a
	// click cannot re-trigger Reload during the 2s window. A direct WebSocket
	// message could bypass the rendered UI and call Reload again; if that
	// happens, both goroutines run to completion and both call TriggerAction
	// successfully (TriggerAction errors only on session disconnect, not on
	// state changes). The second goroutine's payload simply overwrites
	// state.Data with a newer timestamp, which is harmless — the user sees
	// the most recent reload's content. If stricter single-flight semantics
	// are wanted later, copy ProgressBarController.Start's
	// `if state.Loading { return state, nil }` guard to the top of this method.
	state.Loading = true
	state.Data = ""
	go func() {
		time.Sleep(lazyLoadDelay)
		if err := session.TriggerAction("dataLoaded", map[string]any{
			"data": "Content reloaded at " + time.Now().Format("15:04:05"),
		}); err != nil {
			return // Session disconnected — stop cleanly.
		}
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
//
// Reconnect semantics — why no OnConnect:
// ProgressBarState has no `lvt:"persist"` struct tags, so `h.persistable == nil`
// in the framework and `restorePersistedState` returns (nil, false). On every
// WebSocket connect (including reconnects after a network blip), mount.go falls
// through to `cloneStateTyped()` and produces fresh zero-value state
// (Running=false, Done=false, Progress=0). The "stuck Running=true with no
// goroutine" scenario therefore cannot occur — reconnecting always shows the
// Start button again. LazyLoadController needs OnConnect because the spinner
// vs. data swap is the *whole point* of that pattern; here the pattern is the
// goroutine ticking the bar, and a mid-run disconnect simply ends that demo
// (the user clicks Start again on the next page load).
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
			if err := session.TriggerAction("updateProgress", map[string]any{
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
//
// Reconnect semantics — why no OnConnect (same reasoning as ProgressBarController):
// AsyncOpsState has no `lvt:"persist"` tags, so a reconnect mid-fetch produces
// fresh zero-value state (Status="") via cloneStateTyped, not a stuck
// Status="loading". The user always sees the Fetch Data button after a
// reconnect. The in-flight goroutine's eventual TriggerAction either lands on
// the new connection (showing a result the user didn't initiate — harmless,
// since this is a demo) or errors out cleanly when the goroutine's session
// is gone. Adding OnConnect to "recover" loading state would actively make
// this worse by trying to restore Status="loading" against a goroutine that
// the framework has already torn down.
type AsyncOpsController struct{}

const asyncFetchDelay = 2 * time.Second

func (c *AsyncOpsController) Fetch(state AsyncOpsState, ctx *livetemplate.Context) (AsyncOpsState, error) {
	// Re-entrancy guard: block concurrent Fetch while one is already in
	// flight. The button is template-disabled during loading, but a direct
	// WebSocket message bypassing the rendered UI could otherwise spawn
	// two parallel goroutines that both call TriggerAction("fetchResult"),
	// producing two state transitions and two SetFlash calls on the same
	// session. Mirrors the Running guard in ProgressBarController.Start.
	if state.Status == "loading" {
		return state, nil
	}
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
		//
		// Both branches use the same `if err := …; err != nil { return }`
		// pattern as the other controllers for consistency, even though
		// this is a single-shot goroutine where there's nothing else to
		// cancel — readers learning the pattern from this example should
		// see the idiomatic form everywhere.
		if rand.Intn(3) == 0 {
			if err := session.TriggerAction("fetchResult", map[string]any{
				"success": false,
				"error":   "Connection timed out",
			}); err != nil {
				return // Session disconnected — stop cleanly.
			}
		} else {
			if err := session.TriggerAction("fetchResult", map[string]any{
				"success": true,
				"result":  "Data fetched successfully at " + time.Now().Format("15:04:05"),
			}); err != nil {
				return // Session disconnected — stop cleanly.
			}
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
