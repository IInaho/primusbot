package kernel

// Gate is a retry budget: each Try consumes one attempt and reports whether
// the budget is still within the limit. It carries no policy, logging, or
// hint text — those belong to the caller.
type Gate struct {
	maxRetries int
	retries    int
}

// NewGate creates a Gate allowing up to maxRetries retries.
func NewGate(maxRetries int) *Gate {
	return &Gate{maxRetries: maxRetries}
}

// Try consumes one retry attempt and reports whether it is within budget.
func (g *Gate) Try() bool {
	g.retries++
	return g.retries <= g.maxRetries
}

// Reset restores the full retry budget.
func (g *Gate) Reset() { g.retries = 0 }
