package main

import (
	"net/http"
	"slices"

	"github.com/livetemplate/livetemplate"
)

// --- Pattern #12: Active Search ---

type ActiveSearchController struct{}

func (c *ActiveSearchController) Mount(state ActiveSearchState, ctx *livetemplate.Context) (ActiveSearchState, error) {
	// Full directory visible on initial render so the "filter down" story is obvious.
	state.Results = searchContacts(state.Query)
	return state, nil
}

func (c *ActiveSearchController) Change(state ActiveSearchState, ctx *livetemplate.Context) (ActiveSearchState, error) {
	if ctx.Has("query") {
		state.Query = ctx.GetString("query")
		state.Results = searchContacts(state.Query)
	}
	return state, nil
}

func activeSearchHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/search/active-search.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&ActiveSearchController{}, livetemplate.AsState(&ActiveSearchState{
		Title:    "Active Search",
		Category: "Search & Filtering",
	}))
}

// --- Pattern #13: URL-Preserved Filters ---

type URLFiltersController struct{}

// validStatuses / validSorts allow Mount to reject unknown query param values
// without crashing: unknown values fall back to the previous/default state
// rather than producing a 404 or an error. Bookmarks with stale params still
// render usefully.
var (
	validStatuses = map[string]bool{"all": true, "active": true, "completed": true}
	validSorts    = map[string]bool{"name": true, "date": true}
)

func (c *URLFiltersController) Mount(state URLFiltersState, ctx *livetemplate.Context) (URLFiltersState, error) {
	// ctx.Action() == "" means this is a GET navigation (initial load or SPA
	// link click), not a POST action. Only read URL query params on GET to
	// avoid clobbering state from an in-flight action.
	if ctx.Action() == "" {
		if s := ctx.GetString("status"); s != "" && validStatuses[s] {
			state.Status = s
		}
		if s := ctx.GetString("sort"); s != "" && validSorts[s] {
			state.Sort = s
		}
	}
	// Always recompute the item list so the initial render (with defaults)
	// and any subsequent action both see fresh data.
	state.Items = filterItems(state.Status, state.Sort)
	return state, nil
}

func urlFiltersHandler(baseOpts []livetemplate.Option) http.Handler {
	opts := append(slices.Clone(baseOpts),
		livetemplate.WithParseFiles("templates/layout.tmpl", "templates/search/url-filters.tmpl"),
	)
	tmpl := livetemplate.Must(livetemplate.New("layout", opts...))
	return tmpl.Handle(&URLFiltersController{}, livetemplate.AsState(&URLFiltersState{
		Title:    "URL-Preserved Filters",
		Category: "Search & Filtering",
		Status:   "all",
		Sort:     "name",
	}))
}
