package main

type BroadcastMessage struct {
	ID   int
	User string
	Text string
}

type MultiUserSyncState struct {
	Title    string
	Category string
	Counter  int
}

type BroadcastingState struct {
	Title    string
	Category string
	// Username is intentionally NOT lvt:"persist" — persist storage is keyed
	// by session group (state.go:1421 SessionStore.Set(ctx, groupID, ...)),
	// so persisting it would force every tab in the same browser to share a
	// single Username. The whole point of the demo is letting two tabs join
	// as different users; per-connection state is what makes that work.
	// Reconnect Recovery (#29) covers the persist scenario instead.
	Username string
	Messages []BroadcastMessage
}

type PresenceState struct {
	Title    string
	Category string
	// Username + Joined are intentionally NOT lvt:"persist" — see comment on
	// BroadcastingState.Username. Tabs need independent presence identity.
	Username    string
	Joined      bool
	OnlineCount int
}

type ReconnectionState struct {
	Title    string
	Category string
	Counter  int    `lvt:"persist"`
	Notes    string `lvt:"persist"`
}

type LivePreviewState struct {
	Title    string
	Category string
	// Input is persisted so a reconnect lands on the user's last-saved value;
	// Preview is derived from Input by Change/Submit so it doesn't need to
	// persist — leaving it unpersisted means a stale derived value can't
	// briefly appear before the next render rebuilds it.
	Input   string `lvt:"persist"`
	Preview string
}

type ServerPushState struct {
	Title    string
	Category string
	Running  bool
	Elapsed  int
}
