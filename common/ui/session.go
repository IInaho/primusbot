package ui

// SessionMeta is a lightweight descriptor for a persisted session,
// used by the UI to render the session history list.
type SessionMeta struct {
	ID        string `json:"id"`
	CWD       string `json:"cwd"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
	MsgCount  int    `json:"msgCount"`
}
