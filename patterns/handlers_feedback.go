package main

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/livetemplate/livetemplate"
)

// --- Pattern #22: Animations ---

type AnimationsController struct{}

var validAnimateModes = map[string]bool{"fade": true, "slide": true, "scale": true}

func (c *AnimationsController) Add(state AnimationsState, ctx *livetemplate.Context) (AnimationsState, error) {
	if m := ctx.GetString("mode"); validAnimateModes[m] {
		state.Mode = m
	}
	state.Items = append(state.Items, AnimationItem{
		ID:   fmt.Sprintf("item-%d", len(state.Items)+1),
		Name: fmt.Sprintf("Item %d (%s)", len(state.Items)+1, state.Mode),
		Time: time.Now().Format("15:04:05"),
		Mode: state.Mode,
	})
	return state, nil
}

func animationsHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/feedback/animations.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&AnimationsController{}, livetemplate.AsState(&AnimationsState{
		Title:    "Animations",
		Category: "Visual Feedback",
		Mode:     "fade",
	}))
}

// --- Pattern #23: Loading States ---

type LoadingStatesController struct{}

const slowSaveDelay = 2 * time.Second

func (c *LoadingStatesController) SlowSave(state LoadingStatesState, ctx *livetemplate.Context) (LoadingStatesState, error) {
	// Real handlers should honor ctx.Context().Done(); plain Sleep is fine for a 2s demo.
	time.Sleep(slowSaveDelay)
	state.LastSave = time.Now().Format("15:04:05")
	return state, nil
}

func loadingStatesHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/feedback/loading-states.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&LoadingStatesController{}, livetemplate.AsState(&LoadingStatesState{
		Title:    "Loading States",
		Category: "Visual Feedback",
	}))
}

// --- Pattern #24: Highlight on Change ---

type HighlightController struct{}

func (c *HighlightController) Increment(state HighlightState, ctx *livetemplate.Context) (HighlightState, error) {
	state.Counter++
	return state, nil
}

func highlightHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/feedback/highlight.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&HighlightController{}, livetemplate.AsState(&HighlightState{
		Title:    "Highlight on Change",
		Category: "Visual Feedback",
	}))
}

// --- Pattern #25: Flash Messages ---

type FlashMessagesController struct{}

const flashSuccessExpiry = 5 * time.Second

// nudgeFlashExpiry triggers a re-render at FlashExpiry's deadline.
// FlashExpiry is render-driven (no background timer), so without a nudge
// the expired flash sits in the DOM until the user's next interaction.
// The pruner runs on the next render and removes the expired entry.
//
// The "refresh" action it dispatches is registered as a no-op handler on
// each controller that calls this helper.
func nudgeFlashExpiry(ctx *livetemplate.Context, expiry time.Duration) {
	session := ctx.Session()
	if session == nil {
		return
	}
	go func() {
		time.Sleep(expiry + 100*time.Millisecond)
		_ = session.TriggerAction("refresh", nil)
	}()
}

func (c *FlashMessagesController) Save(state FlashMessagesState, ctx *livetemplate.Context) (FlashMessagesState, error) {
	name := strings.TrimSpace(ctx.GetString("name"))
	if name == "" {
		ctx.ClearFlash("success")
		ctx.SetFlash("error", "Name is required")
		return state, nil
	}
	ctx.ClearFlash("error")
	ctx.SetFlash("success", "Saved: "+name, livetemplate.FlashExpiry(flashSuccessExpiry))
	nudgeFlashExpiry(ctx, flashSuccessExpiry)
	return state, nil
}

// Refresh is a no-op action whose only purpose is to trigger a re-render
// (and therefore a getMessages snapshot, which prunes expired flash).
// Invoked by the goroutine in Save after FlashExpiry elapses.
func (c *FlashMessagesController) Refresh(state FlashMessagesState, ctx *livetemplate.Context) (FlashMessagesState, error) {
	return state, nil
}

func (c *FlashMessagesController) Notify(state FlashMessagesState, ctx *livetemplate.Context) (FlashMessagesState, error) {
	ctx.SetFlash("info", "Heads up — this stays until you dismiss it")
	return state, nil
}

func (c *FlashMessagesController) DismissNotify(state FlashMessagesState, ctx *livetemplate.Context) (FlashMessagesState, error) {
	ctx.ClearFlash("info")
	return state, nil
}

func flashMessagesHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/feedback/flash-messages.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&FlashMessagesController{}, livetemplate.AsState(&FlashMessagesState{
		Title:    "Flash Messages",
		Category: "Visual Feedback",
	}))
}
