package view

import "nekocode/bot/session"

func SessionMetas(list []session.Meta) []SessionMeta {
	out := make([]SessionMeta, 0, len(list))
	for _, m := range list {
		out = append(out, NewSessionMeta(m.ID, m.CWD, m.CreatedAt, m.UpdatedAt, m.MsgCount))
	}
	return out
}

func NewSessionMeta(id, cwd string, createdAt, updatedAt int64, msgCount int) SessionMeta {
	return SessionMeta{
		ID:        id,
		CWD:       cwd,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
		MsgCount:  msgCount,
	}
}
