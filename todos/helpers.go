package main

import (
	"math"
	"sort"
	"strings"
	"time"
)

// applySorting sorts the filtered todos based on the current sort setting.
//
// Supported sort modes:
//   - "" (empty/default): newest first (reverse chronological by created_at)
//   - "alphabetical": A-Z by todo text (case-insensitive)
//   - "reverse_alphabetical": Z-A by todo text (case-insensitive)
//   - "oldest_first": chronological by created_at
func applySorting(state TodoState) TodoState {
	switch state.SortBy {
	case "alphabetical":
		sort.Slice(state.FilteredTodos, func(i, j int) bool {
			return strings.ToLower(state.FilteredTodos[i].Text) < strings.ToLower(state.FilteredTodos[j].Text)
		})
	case "reverse_alphabetical":
		sort.Slice(state.FilteredTodos, func(i, j int) bool {
			return strings.ToLower(state.FilteredTodos[i].Text) > strings.ToLower(state.FilteredTodos[j].Text)
		})
	case "oldest_first":
		sort.Slice(state.FilteredTodos, func(i, j int) bool {
			return state.FilteredTodos[i].CreatedAt.Before(state.FilteredTodos[j].CreatedAt)
		})
	default:
		// Default: newest first (reverse chronological)
		sort.Slice(state.FilteredTodos, func(i, j int) bool {
			return state.FilteredTodos[i].CreatedAt.After(state.FilteredTodos[j].CreatedAt)
		})
	}
	return state
}

// applyPagination calculates pagination metadata and extracts the current page's todos.
//
// It updates the following state fields:
//   - TotalPages: total number of pages based on filtered todos and page size
//   - CurrentPage: clamped to valid range [1, TotalPages]
//   - PaginatedTodos: slice of todos for the current page
//   - ShowPagination: whether to show pagination controls (more than 1 page)
//   - PrevDisabled: whether the previous button should be disabled
//   - NextDisabled: whether the next button should be disabled
func applyPagination(state TodoState) TodoState {
	// Handle empty state
	if len(state.FilteredTodos) == 0 {
		state.TotalPages = 1
		state.CurrentPage = 1
		state.PaginatedTodos = []TodoItem{}
		state.ShowPagination = false
		state.PrevDisabled = true
		state.NextDisabled = true
		return state
	}

	// Calculate total pages
	state.TotalPages = int(math.Ceil(float64(len(state.FilteredTodos)) / float64(state.PageSize)))

	// Clamp current page to valid range
	if state.CurrentPage < 1 {
		state.CurrentPage = 1
	}
	if state.CurrentPage > state.TotalPages {
		state.CurrentPage = state.TotalPages
	}

	// Calculate slice indices for current page
	start := (state.CurrentPage - 1) * state.PageSize
	end := start + state.PageSize
	if end > len(state.FilteredTodos) {
		end = len(state.FilteredTodos)
	}

	// Extract current page items
	state.PaginatedTodos = state.FilteredTodos[start:end]

	// Update pagination UI flags
	hasPaginated := len(state.PaginatedTodos) > 0
	state.ShowPagination = hasPaginated && state.TotalPages > 1
	state.PrevDisabled = !hasPaginated || state.CurrentPage <= 1
	state.NextDisabled = !hasPaginated || state.CurrentPage >= state.TotalPages

	return state
}

// formatTime returns the current time formatted as "YYYY-MM-DD HH:MM:SS".
// This format is used to display when the todo list was last modified.
func formatTime() string {
	return time.Now().Format("2006-01-02 15:04:05")
}
