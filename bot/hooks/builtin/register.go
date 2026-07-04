package builtin

import "nekocode/bot/hooks"

func Register(r *hooks.Registry) {
	for _, h := range All() {
		r.Register(h)
	}
}
