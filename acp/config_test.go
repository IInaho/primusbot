package acp

import (
	"encoding/json"
	"io"
	"testing"

	controlruntime "nekocode/runtime"
)

func newConfigTestServer() (*server, *fakeBackend) {
	backend := &fakeBackend{
		models: []controlruntime.ModelOption{
			{Name: "flash", Model: "gpt-flash", ReasoningEfforts: []string{"low", "high"}},
			{Name: "pro", Model: "gpt-pro"},
		},
		activeModel:   "flash",
		currentEffort: "low",
	}
	s := &server{backend: backend, cwd: "/workspace", active: make(map[string]*activeTurn), conn: newConnection(io.Discard)}
	backend.current = "session-1"
	return s, backend
}

func TestConfigOptions(t *testing.T) {
	s, _ := newConfigTestServer()
	options := s.configOptions()
	if len(options) != 3 {
		t.Fatalf("options = %#v", options)
	}
	model := options[0]
	if model["id"] != "model" || model["type"] != "select" || model["currentValue"] != "flash" {
		t.Fatalf("model option = %#v", model)
	}
	values := model["options"].([]map[string]any)
	if len(values) != 2 || values[0]["value"] != "flash" || values[1]["description"] != "gpt-pro" {
		t.Fatalf("model values = %#v", values)
	}
	effort := options[1]
	if effort["id"] != "reasoning_effort" || effort["currentValue"] != "low" {
		t.Fatalf("effort option = %#v", effort)
	}
	if effortValues := effort["options"].([]map[string]any); len(effortValues) != 3 || effortValues[0]["value"] != "auto" {
		t.Fatalf("effort values = %#v", effortValues)
	}
	full := options[2]
	if full["id"] != "full_access" || full["type"] != "select" || full["currentValue"] != "manual" {
		t.Fatalf("full_access option = %#v", full)
	}
	if values := full["options"].([]map[string]any); len(values) != 2 || values[1]["value"] != "full" {
		t.Fatalf("full_access values = %#v", values)
	}

	// A client that advertised boolean config support gets the toggle form.
	s.mu.Lock()
	s.clientBooleanConfig = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.clientBooleanConfig = false
		s.mu.Unlock()
	}()
	boolean := s.configOptions()[2]
	if boolean["type"] != "boolean" || boolean["currentValue"] != false {
		t.Fatalf("boolean full_access option = %#v", boolean)
	}
}

func TestConfigOptionsOmitEffortWithoutCapability(t *testing.T) {
	s, backend := newConfigTestServer()
	backend.activeModel = "pro" // no ReasoningEfforts
	if options := s.configOptions(); len(options) != 2 {
		t.Fatalf("options = %#v", options)
	}
}

func TestSetConfigOption(t *testing.T) {
	s, backend := newConfigTestServer()

	// Switching to the active model is an idempotent but valid request.
	result, rpcErr := s.setConfigOption(configParams("session-1", "model", `"flash"`))
	if rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if backend.switchedModel != "flash" {
		t.Fatalf("switchedModel = %q", backend.switchedModel)
	}
	if !hasConfigOption(result, "model") {
		t.Fatalf("response missing configOptions: %#v", result)
	}

	if _, rpcErr := s.setConfigOption(configParams("session-1", "reasoning_effort", `"auto"`)); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if backend.effortSet != "auto" {
		t.Fatalf("effortSet = %q", backend.effortSet)
	}

	if _, rpcErr := s.setConfigOption(booleanParams("session-1", true)); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if !backend.fullAccess {
		t.Fatal("full access was not enabled")
	}

	// The select form is accepted regardless of client capability.
	if _, rpcErr := s.setConfigOption(configParams("session-1", "full_access", `"manual"`)); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if backend.fullAccess {
		t.Fatal("full access was not disabled")
	}
	if _, rpcErr := s.setConfigOption(configParams("session-1", "full_access", `"full"`)); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if !backend.fullAccess {
		t.Fatal("full access was not enabled via select form")
	}

	// Rejections.
	cases := []struct {
		name   string
		params json.RawMessage
	}{
		{"unknown model", configParams("session-1", "model", `"nope"`)},
		{"unknown effort", configParams("session-1", "reasoning_effort", `"ultra"`)},
		{"unknown config", configParams("session-1", "something", `"x"`)},
		{"unknown session", configParams("ghost", "model", `"flash"`)},
		{"model needs string", boolValueForConfig("session-1", "model", true)},
		{"full_access needs boolean or mode", configParams("session-1", "full_access", `"yes"`)},
	}
	for _, tt := range cases {
		if _, rpcErr := s.setConfigOption(tt.params); rpcErr == nil {
			t.Fatalf("%s was accepted", tt.name)
		}
	}
}

func TestSessionConfigIsRestoredPerSession(t *testing.T) {
	s, backend := newConfigTestServer()
	s.sessionConfigs = map[string]sessionConfig{
		"session-1": {Model: "flash", ReasoningEffort: "high"},
		"session-2": {Model: "pro", FullAccess: true},
	}
	if err := s.applySessionConfig("session-2"); err != nil {
		t.Fatal(err)
	}
	if backend.activeModel != "pro" || !backend.fullAccess {
		t.Fatalf("session-2 config not applied: model=%q full=%v", backend.activeModel, backend.fullAccess)
	}
	if err := s.applySessionConfig("session-1"); err != nil {
		t.Fatal(err)
	}
	if backend.activeModel != "flash" || backend.currentEffort != "high" || backend.fullAccess {
		t.Fatalf("session-1 config not restored: model=%q effort=%q full=%v", backend.activeModel, backend.currentEffort, backend.fullAccess)
	}
}

func TestModelSwitchResetsEffortToConfiguredDefault(t *testing.T) {
	s, backend := newConfigTestServer()
	backend.models[1].ReasoningEffort = ""
	if _, rpcErr := s.setConfigOption(configParams("session-1", "model", `"pro"`)); rpcErr != nil {
		t.Fatal(rpcErr)
	}
	if backend.currentEffort != "" {
		t.Fatalf("reasoning effort leaked from previous model: %q", backend.currentEffort)
	}
}

func TestCleanupRestoresConnectionDefaults(t *testing.T) {
	s, backend := newConfigTestServer()
	s.defaultConfig = sessionConfig{Model: "flash", ReasoningEffort: "low"}
	backend.activeModel = "pro"
	backend.currentEffort = "high"
	backend.fullAccess = true
	if err := s.cleanup(); err != nil {
		t.Fatal(err)
	}
	if backend.activeModel != "flash" || backend.currentEffort != "low" || backend.fullAccess {
		t.Fatalf("defaults not restored: model=%q effort=%q full=%v", backend.activeModel, backend.currentEffort, backend.fullAccess)
	}
	if backend.mcpCleared != 1 {
		t.Fatalf("MCP cleanup calls = %d, want 1", backend.mcpCleared)
	}
}

func TestCleanupRestoresEveryModelEffort(t *testing.T) {
	s, backend := newConfigTestServer()
	s.defaultConfig = sessionConfig{Model: "flash", ReasoningEffort: "low"}
	s.defaultEfforts = map[string]string{"flash": "low", "pro": ""}

	backend.activeModel = "pro"
	if err := backend.SetSessionReasoning("high"); err != nil {
		t.Fatal(err)
	}
	if err := s.cleanup(); err != nil {
		t.Fatal(err)
	}
	if backend.activeModel != "flash" || backend.currentEffort != "low" {
		t.Fatalf("active default not restored: model=%q effort=%q", backend.activeModel, backend.currentEffort)
	}
	for _, model := range backend.models {
		if model.Name == "pro" && model.ReasoningEffort != "" {
			t.Fatalf("inactive model effort leaked: %q", model.ReasoningEffort)
		}
	}
}

func configParams(sessionID, configID, value string) json.RawMessage {
	body, _ := json.Marshal(map[string]any{"sessionId": sessionID, "configId": configID, "value": json.RawMessage(value)})
	return body
}

func booleanParams(sessionID string, value bool) json.RawMessage {
	return boolValueForConfig(sessionID, "full_access", value)
}

func boolValueForConfig(sessionID, configID string, value bool) json.RawMessage {
	body, _ := json.Marshal(map[string]any{
		"sessionId": sessionID, "configId": configID,
		"type": "boolean", "value": value,
	})
	return body
}

func hasConfigOption(result any, id string) bool {
	response, ok := result.(map[string]any)
	if !ok {
		return false
	}
	options, ok := response["configOptions"].([]map[string]any)
	if !ok {
		return false
	}
	for _, option := range options {
		if option["id"] == id {
			return true
		}
	}
	return false
}
