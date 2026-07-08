// Package sse provides SSE (Server-Sent Events) parsing helpers.
package sse

import "strings"

// SSELineData extracts the data payload from an SSE "data: " line.
// Returns the data string and true if the line is a data line, or ("", false) otherwise.
func SSELineData(line string) (string, bool) {
	if !strings.HasPrefix(line, "data: ") {
		return "", false
	}
	return strings.TrimPrefix(line, "data: "), true
}
