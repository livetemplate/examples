package main

type AnimationItem struct {
	ID   string
	Name string
	Time string
	Mode string
}

type AnimationsState struct {
	Title    string
	Category string
	Items    []AnimationItem
	Mode     string
}

type LoadingStatesState struct {
	Title    string
	Category string
	LastSave string
}

type HighlightState struct {
	Title    string
	Category string
	Counter  int
}

type FlashMessagesState struct {
	Title    string
	Category string
}
