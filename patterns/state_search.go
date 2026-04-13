package main

// ActiveSearchState holds the state for the Active Search pattern (#12).
type ActiveSearchState struct {
	Title    string
	Category string
	Query    string
	Results  []Contact
}

// URLFiltersState holds the state for the URL-Preserved Filters pattern (#13).
type URLFiltersState struct {
	Title    string
	Category string
	Status   string
	Sort     string
	Items    []FilterItem
}
