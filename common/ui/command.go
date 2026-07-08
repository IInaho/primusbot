package ui

// CmdResult tells the TUI what to do after a command is executed.
type CmdResult int

const (
	CmdNone           CmdResult = iota // no command matched, start agent
	CmdHandled                         // command handled, no further action
	CmdConfirming                      // command handled, wait for confirmation
	CmdSessionResumed                  // session resumed, TUI should reload messages
)
