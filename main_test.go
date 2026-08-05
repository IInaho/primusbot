package main

import controlruntime "nekocode/runtime"

// Keep transport methods on the Wails-bound wrapper, not only on the inner
// GUI implementation. Wails generates frontend bindings from main.App.
var _ interface {
	CommandMenu(string) *controlruntime.CommandMenu
} = (*App)(nil)
