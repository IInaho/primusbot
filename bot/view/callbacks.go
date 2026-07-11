package view

type RunCallbacks struct {
	Text   func(delta string)
	Reason func(delta string)
	Step   func(action, toolName, toolArgs, output string)
}
