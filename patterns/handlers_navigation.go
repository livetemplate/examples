package main

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/go-playground/validator/v10"
	"github.com/livetemplate/livetemplate"
)

// validateNav is the per-process validator shared by navigation-category
// controllers. The validator package caches struct/field metadata internally,
// so a single instance is faster than constructing one per request.
var validateNav = validator.New()

// --- Pattern #17: Modal Dialog ---

// ModalDialogController demonstrates a native <dialog> opened via the Invoker
// Commands API (command/commandfor). On invalid submit the form's field errors
// must render inside the still-open dialog — that's the load-bearing behavior
// this pattern shows; on valid submit the dialog closes and a flash appears
// outside it.
type ModalDialogController struct{}

type modalDialogInput struct {
	Name  string `json:"name"  validate:"required,min=3"`
	Email string `json:"email" validate:"required,email"`
}

func (c *ModalDialogController) Save(state ModalDialogState, ctx *livetemplate.Context) (ModalDialogState, error) {
	var in modalDialogInput
	if err := ctx.BindAndValidate(&in, validateNav); err != nil {
		return state, err
	}
	state.Name = in.Name
	state.Email = in.Email
	state.SavedAt = time.Now().Format("15:04:05")
	ctx.SetFlash("success", "Profile saved", livetemplate.FlashExpiry(5*time.Second))
	return state, nil
}

func modalDialogHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/navigation/modal-dialog.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&ModalDialogController{}, livetemplate.AsState(&ModalDialogState{
		Title:    "Modal Dialog",
		Category: "Dialogs, Tabs & Navigation",
		Name:     "Ada Lovelace",
		Email:    "ada@analytical.engine",
	}))
}

// --- Pattern #18: Confirm Dialog ---

// ConfirmDialogController gates a destructive action behind a per-row
// <dialog id="confirm-{{.ID}}">. The Delete action reads the item id from the
// submit button's value attribute (the canonical Tier-1 row-action shape),
// not a hidden input.
type ConfirmDialogController struct{}

const confirmDialogItemCount = 5

func (c *ConfirmDialogController) Mount(state ConfirmDialogState, ctx *livetemplate.Context) (ConfirmDialogState, error) {
	if len(state.Items) == 0 && ctx.Action() == "" {
		state.Items = getItemPage(1, confirmDialogItemCount)
	}
	return state, nil
}

func (c *ConfirmDialogController) Delete(state ConfirmDialogState, ctx *livetemplate.Context) (ConfirmDialogState, error) {
	id := ctx.GetString("value")
	state.Items = slices.DeleteFunc(state.Items, func(it Item) bool { return it.ID == id })
	return state, nil
}

func confirmDialogHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/navigation/confirm-dialog.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&ConfirmDialogController{}, livetemplate.AsState(&ConfirmDialogState{
		Title:    "Confirm Dialog",
		Category: "Dialogs, Tabs & Navigation",
	}))
}

// --- Pattern #19: Tabs (HATEOAS) ---

// TabsController is Mount-only. Tab clicks are <a href="?tab=…"> which the
// framework routes through the WebSocket as the reserved __navigate__ action;
// the server re-runs Mount() with the new query params and ctx.Action() is ""
// inside that re-run, so the same `if ctx.Action() == ""` guard used on initial
// GET also covers tab switches.
type TabsController struct{}

var validTabs = map[string]bool{"overview": true, "settings": true, "activity": true}

func (c *TabsController) Mount(state TabsState, ctx *livetemplate.Context) (TabsState, error) {
	if ctx.Action() == "" {
		if t := ctx.GetString("tab"); validTabs[t] {
			state.ActiveTab = t
		} else if state.ActiveTab == "" {
			state.ActiveTab = "overview"
		}
	}
	return state, nil
}

func tabsHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/navigation/tabs.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&TabsController{}, livetemplate.AsState(&TabsState{
		Title:    "Tabs (HATEOAS)",
		Category: "Dialogs, Tabs & Navigation",
	}))
}

// --- Pattern #20: SPA Navigation ---

// SPANavController demonstrates the framework's automatic link interception:
// same-pathname query-string changes go through the in-band __navigate__ action
// over WebSocket, cross-pathname links trigger a full reconnect, and external
// targets opt out with lvt-nav:no-intercept.
type SPANavController struct{}

const spaNavMaxStep = 3

func (c *SPANavController) Mount(state SPANavState, ctx *livetemplate.Context) (SPANavState, error) {
	if ctx.Action() == "" {
		if s := ctx.GetString("step"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n >= 1 && n <= spaNavMaxStep {
				state.Step = n
			}
		}
		if state.Step == 0 {
			state.Step = 1
		}
	}
	return state, nil
}

func spaNavigationHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/navigation/spa-navigation.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&SPANavController{}, livetemplate.AsState(&SPANavState{
		Title:    "SPA Navigation",
		Category: "Dialogs, Tabs & Navigation",
	}))
}

// --- Pattern #21: Keyboard Shortcuts ---

// ShortcutsController is Tier-2: it uses lvt-on:window:keydown bindings to
// drive a command-palette-style overlay. "/" opens the panel; "Escape" closes
// it (bound only while the panel is rendered). A bounded log of recent open/
// close events surfaces the state in the page itself.
type ShortcutsController struct{}

const shortcutsLogMax = 5

func (c *ShortcutsController) Open(state ShortcutsState, ctx *livetemplate.Context) (ShortcutsState, error) {
	if state.PanelOpen {
		return state, nil
	}
	state.PanelOpen = true
	state.Log = appendLog(state.Log, fmt.Sprintf("[%s] Opened panel", time.Now().Format("15:04:05")))
	return state, nil
}

func (c *ShortcutsController) Close(state ShortcutsState, ctx *livetemplate.Context) (ShortcutsState, error) {
	if !state.PanelOpen {
		return state, nil
	}
	state.PanelOpen = false
	state.Log = appendLog(state.Log, fmt.Sprintf("[%s] Closed panel", time.Now().Format("15:04:05")))
	return state, nil
}

func appendLog(log []string, entry string) []string {
	log = append(log, entry)
	if len(log) > shortcutsLogMax {
		log = log[len(log)-shortcutsLogMax:]
	}
	return log
}

func keyboardShortcutsHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/navigation/keyboard-shortcuts.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&ShortcutsController{}, livetemplate.AsState(&ShortcutsState{
		Title:    "Keyboard Shortcuts",
		Category: "Dialogs, Tabs & Navigation",
	}))
}
