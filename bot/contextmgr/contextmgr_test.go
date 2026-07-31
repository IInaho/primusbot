package contextmgr

import "testing"

func TestNewBuildsLayeredContext(t *testing.T) {
	manager := New(Config{SystemPrompt: "system"})
	manager.Add("user", "hello")

	messages := manager.Build()
	if len(messages) != 2 || messages[0].Content != "system" || messages[1].Content != "hello" {
		t.Fatalf("Build() = %#v", messages)
	}
}
