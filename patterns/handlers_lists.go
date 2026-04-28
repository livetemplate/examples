package main

import (
	"net/http"
	"slices"
	"sync"

	"github.com/livetemplate/livetemplate"
)

// listPageSize is the page size used by Click To Load (#9) and Infinite Scroll (#10).
const listPageSize = 10

// --- Pattern #8: Delete Row ---

// DeleteRowController holds a shared in-memory "database" protected by a
// mutex. Mount copies the DB snapshot into per-session state on every
// connect, so deletions persist across reloads and cross-handler navigation
// without needing `lvt:"persist"` struct tags. The DB lives for the life
// of the process; restarting the server resets it.
type DeleteRowController struct {
	mu    sync.Mutex
	items []Item
}

const deleteRowInitialCount = 5

func newDeleteRowController() *DeleteRowController {
	return &DeleteRowController{items: getItemPage(1, deleteRowInitialCount)}
}

// snapshot returns an independent copy of the current DB. Caller must not
// hold c.mu when invoking (this method acquires it internally).
func (c *DeleteRowController) snapshot() []Item {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.items)
}

func (c *DeleteRowController) Mount(state DeleteRowState, ctx *livetemplate.Context) (DeleteRowState, error) {
	state.Items = c.snapshot()
	return state, nil
}

func (c *DeleteRowController) Delete(state DeleteRowState, ctx *livetemplate.Context) (DeleteRowState, error) {
	// Button sends its `value` attribute as data.value — see
	// docs/references/progressive-complexity-reference.md.
	id := ctx.GetString("value")
	c.mu.Lock()
	c.items = slices.DeleteFunc(c.items, func(item Item) bool {
		return item.ID == id
	})
	c.mu.Unlock()
	state.Items = c.snapshot()
	return state, nil
}

// Restore refills the DB to its initial state. Wired to a button that
// appears after the last item is deleted, so visitors can reset the demo
// without restarting the server.
func (c *DeleteRowController) Restore(state DeleteRowState, ctx *livetemplate.Context) (DeleteRowState, error) {
	c.mu.Lock()
	c.items = getItemPage(1, deleteRowInitialCount)
	c.mu.Unlock()
	state.Items = c.snapshot()
	return state, nil
}

func deleteRowHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/lists/delete-row.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(newDeleteRowController(), livetemplate.AsState(&DeleteRowState{
		Title:    "Delete Row",
		Category: "Lists & Data",
	}))
}

// --- Pattern #9: Click To Load ---

type ClickToLoadController struct{}

func (c *ClickToLoadController) LoadMore(state ClickToLoadState, ctx *livetemplate.Context) (ClickToLoadState, error) {
	state.CurrentPage++
	newItems := getItemPage(state.CurrentPage, listPageSize)
	state.Items = append(state.Items, newItems...)
	state.HasMore = len(newItems) == listPageSize
	return state, nil
}

func clickToLoadHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/lists/click-to-load.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&ClickToLoadController{}, livetemplate.AsState(&ClickToLoadState{
		Title:       "Click To Load",
		Category:    "Lists & Data",
		Items:       getItemPage(1, listPageSize),
		CurrentPage: 1,
		HasMore:     true,
	}))
}

// --- Pattern #11: Value Select (Cascading Selects) ---

type ValueSelectController struct{}

func (c *ValueSelectController) Mount(state ValueSelectState, ctx *livetemplate.Context) (ValueSelectState, error) {
	state.Makes = getCarMakes()
	if state.Make != "" {
		state.Models = getCarModels(state.Make)
	}
	return state, nil
}

func (c *ValueSelectController) Change(state ValueSelectState, ctx *livetemplate.Context) (ValueSelectState, error) {
	if ctx.Has("make") {
		state.Make = ctx.GetString("make")
		state.Models = getCarModels(state.Make)
		// Auto-select first model so the user sees the cascade propagate.
		state.Model = ""
		if len(state.Models) > 0 {
			state.Model = state.Models[0]
		}
	}
	if ctx.Has("model") {
		state.Model = ctx.GetString("model")
	}
	return state, nil
}

func valueSelectHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/lists/value-select.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&ValueSelectController{}, livetemplate.AsState(&ValueSelectState{
		Title:    "Value Select",
		Category: "Lists & Data",
	}))
}

// --- Pattern #10: Infinite Scroll ---

type InfiniteScrollController struct{}

// LoadMore is dispatched by the client-side IntersectionObserver when
// <div lvt-scroll-sentinel> becomes visible. Uses the larger
// infiniteScrollDataset (100 items) so the auto-pagination cascade is
// actually visible during the demo; ClickToLoad uses the 25-item
// listDataset which only needs a couple of clicks.
func (c *InfiniteScrollController) LoadMore(state InfiniteScrollState, ctx *livetemplate.Context) (InfiniteScrollState, error) {
	state.CurrentPage++
	newItems := getInfiniteScrollPage(state.CurrentPage, listPageSize)
	state.Items = append(state.Items, newItems...)
	state.HasMore = len(newItems) == listPageSize
	return state, nil
}

func infiniteScrollHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/lists/infinite-scroll.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&InfiniteScrollController{}, livetemplate.AsState(&InfiniteScrollState{
		Title:       "Infinite Scroll",
		Category:    "Lists & Data",
		Items:       getInfiniteScrollPage(1, listPageSize),
		CurrentPage: 1,
		HasMore:     true,
	}))
}

// --- Sortable List ---

// SortableController holds the list ordering process-wide so it persists across reloads (live multi-tab sync would need Sync()).
type SortableController struct {
	mu    sync.Mutex
	items []SortableItem
}

func newSortableController() *SortableController {
	return &SortableController{items: initialSortableItems()}
}

func (c *SortableController) snapshot() []SortableItem {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.items)
}

func (c *SortableController) Mount(state SortableState, ctx *livetemplate.Context) (SortableState, error) {
	state.Items = c.snapshot()
	return state, nil
}

// Reorder reads dragSourceKey / dragTargetKey (injected by livetemplate/client from the source/target data-key) and always repopulates state.Items from the locked snapshot — the framework-provided value is per-session and may lag the shared ordering.
func (c *SortableController) Reorder(state SortableState, ctx *livetemplate.Context) (SortableState, error) {
	src := ctx.GetString("dragSourceKey")
	tgt := ctx.GetString("dragTargetKey")

	c.mu.Lock()
	defer c.mu.Unlock()

	if src == "" || tgt == "" || src == tgt {
		state.Items = slices.Clone(c.items)
		return state, nil
	}

	srcIdx, tgtIdx := -1, -1
	for i, it := range c.items {
		if it.Key == src {
			srcIdx = i
		}
		if it.Key == tgt {
			tgtIdx = i
		}
		if srcIdx >= 0 && tgtIdx >= 0 {
			break
		}
	}
	if srcIdx < 0 || tgtIdx < 0 {
		state.Items = slices.Clone(c.items)
		return state, nil
	}

	moved := c.items[srcIdx]
	c.items = slices.Delete(c.items, srcIdx, srcIdx+1)
	if srcIdx < tgtIdx {
		tgtIdx--
	}
	c.items = slices.Insert(c.items, tgtIdx, moved)

	state.Items = slices.Clone(c.items)
	return state, nil
}

func (c *SortableController) Reset(state SortableState, ctx *livetemplate.Context) (SortableState, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = initialSortableItems()
	state.Items = slices.Clone(c.items)
	return state, nil
}

func sortableHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/lists/sortable.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(newSortableController(), livetemplate.AsState(&SortableState{
		Title:    "Sortable List",
		Category: "Lists & Data",
	}))
}
