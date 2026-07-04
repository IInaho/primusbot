package impl

// UnavailableError is returned when no sandbox backend could be used.
// Callers should treat it as a signal to request host-execution permission.
type UnavailableError struct {
	Reason string
}

func (e UnavailableError) Error() string {
	if e.Reason == "" {
		return "sandbox unavailable"
	}
	return "sandbox unavailable: " + e.Reason
}
