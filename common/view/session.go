package view

func NewSessionMeta(id, cwd string, createdAt, updatedAt int64, msgCount int) SessionMeta {
	return SessionMeta{
		ID:        id,
		CWD:       cwd,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		MsgCount:  msgCount,
	}
}
