package acp

import (
	"encoding/json"
	"testing"
)

func TestWireTypes(t *testing.T) {
	response := initializeResponse{
		ProtocolVersion: protocolVersion,
		AgentCapabilities: agentCapabilities{
			LoadSession:         true,
			PromptCapabilities:  promptCapabilities{},
			MCPCapabilities:     mcpCapabilities{},
			SessionCapabilities: sessionCapabilities{List: &struct{}{}, Delete: &struct{}{}},
		},
		AgentInfo:   implementation{Name: "nekocode", Version: "dev"},
		AuthMethods: []any{},
	}
	b, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got["protocolVersion"] != float64(1) {
		t.Fatalf("protocolVersion = %#v", got["protocolVersion"])
	}
	caps := got["agentCapabilities"].(map[string]any)
	if caps["loadSession"] != true {
		t.Fatalf("loadSession = %#v", caps["loadSession"])
	}
}
