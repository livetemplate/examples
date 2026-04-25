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

// validateNav is a single per-process validator instance shared by all
// navigation-category controllers that use BindAndValidate. The validator
// package recommends a singleton because internal struct/field caches are
// computed lazily on first access.
var validateNav = validator.New()

// --- Pattern #17: Modal Dialog ---

// ModalDialogController demonstrates a native <dialog> element triggered via
// the Invoker Commands API (command/commandfor). The form inside the dialog
// uses BindAndValidate so invalid submissions surface field-scoped errors via
// {{.lvt.ErrorTag}} that render INSIDE the still-open dialog — exercising the
// client v0.8.33 morphdom fix that allows child updates inside open dialogs.
//
// On a successful save the framework's diff updates the dialog wrapper and
// the browser closes the dialog as part of normal form-submit handling; the
// success flash then becomes visible on the page beneath the trigger button.
type ModalDialogController struct{}

// modalDialogInput mirrors the form fields. Validator tags drive the
// per-field error messages that {{.lvt.ErrorTag "name"}} renders inside the
// dialog when validation fails.
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

// ConfirmDialogController demonstrates a CSP-compliant confirmation flow: each
// row owns its own <dialog id="confirm-{{.ID}}"> opened via command="show-modal"
// commandfor="confirm-{{.ID}}". The Delete action reads the item id from the
// submit button's value attribute, NOT a hidden input — see CLAUDE.md and the
// patterns proposal for the rationale ("button name=delete value={{.ID}}").
//
// Items live on the controller (singleton, never cloned) behind a sync.Mutex,
// so deletes persist across page reloads within a process. We don't use one
// here because demo cardinality is small (5 items) and the user-visible
// "Restore" button is intentionally absent — refreshing reseeds.
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
// client routes through the WebSocket as the reserved __navigate__ action
// (library v0.8.19+, client v0.8.26+). The server re-runs Mount() with the
// new query params; ctx.Action() returns "" inside that re-run (the action
// loop sets it via WithAction("")), so the same `if ctx.Action() == ""`
// guard used on initial GET also runs on tab switches. Compare to Pattern
// #13 (URL-Preserved Filters) which uses the same shape.
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
// same-pathname query-string changes use the in-band __navigate__ action over
// WebSocket, while cross-pathname links trigger a full reconnect. The Step
// counter ([1, 3]) showcases the same-pathname path; cross-pathname is shown
// via links to other pattern handlers; lvt-nav:no-intercept is shown via an
// external link.
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
// drive a command-palette-style overlay. Open binds "/", Close binds "Escape"
// inside the open panel. A small ring buffer of activity log entries doubles
// as visual feedback in the rendered HTML — handy for the e2e tests since
// the panel state can be confirmed without screen-scraping the overlay.
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
