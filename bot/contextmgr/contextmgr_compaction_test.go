package contextmgr

import (
	"strings"
	"testing"
	"time"

	"nekocode/bot/provider/types"
)

func TestAutoCompactIfNeeded_NoStrategy(t *testing.T) {
	m := New(Config{SystemPrompt: "test prompt"})
	m.state.compressor = nil
	if _, err := m.AutoCompactIfNeeded(); err != nil {
		t.Errorf("AutoCompactIfNeeded error: %v", err)
	}
}

func TestAutoCompactReportsActualOverflowWhenNothingCanBeTrimmed(t *testing.T) {
	m := New(Config{SystemPrompt: strings.Repeat("system context ", 20), ContextWindow: 1})
	compacted, err := m.AutoCompactIfNeeded()
	if compacted || err == nil || !strings.Contains(err.Error(), "context full") {
		t.Fatalf("AutoCompactIfNeeded() = %v, %v; want untrimmed context full", compacted, err)
	}
}

func TestSummarizeDoesNotBlockOrOverwriteConcurrentHistory(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	m := New(Config{
		SystemPrompt: "system",
		Summarizer: func(msgs []types.Message, prev string) (string, error) {
			close(started)
			<-release
			return "<summary>Compacted context summary that is long enough to pass validation.</summary>", nil
		},
	})
	for i := 0; i < 8; i++ {
		m.Add("user", "old question")
		m.AddAssistant(types.Message{Content: "old answer"})
	}

	compactDone := make(chan error, 1)
	go func() {
		_, err := m.Summarize()
		compactDone <- err
	}()
	<-started

	addDone := make(chan struct{})
	go func() {
		m.Add("user", "message added during summary")
		close(addDone)
	}()
	select {
	case <-addDone:
	case <-time.After(time.Second):
		t.Fatal("Add blocked while summarizer was running")
	}

	close(release)
	if err := <-compactDone; err == nil || !strings.Contains(err.Error(), "context changed") {
		t.Fatalf("Summarize() error = %v, want stale-summary rejection", err)
	}
	if !containsContent(m.Build(), "message added during summary") {
		t.Fatal("concurrent message was overwritten by stale summary")
	}
}

func TestManagerSummarizeUsesDefaultCompressionStrategy(t *testing.T) {
	m := New(Config{
		SystemPrompt: "system",
		Summarizer: func(msgs []types.Message, prev string) (string, error) {
			return "<summary>Compacted context summary that replaces the hidden full conversation history.</summary>", nil
		},
	})
	for i := 1; i <= 8; i++ {
		m.Add("user", "old question")
		m.Add("assistant", "old answer")
	}

	if compacted, err := m.Summarize(); err != nil {
		t.Fatalf("Summarize() error: %v", err)
	} else if !compacted {
		t.Fatal("Summarize() did not compact")
	}

	snap := m.Snapshot()
	if len(snap.Messages) >= 16 {
		t.Fatalf("messages were not replaced: got %d", len(snap.Messages))
	}
	if report := m.Report(); report.CompactCount != 1 || report.Archived == 0 || report.TrimCount != report.Archived {
		t.Fatalf("compaction counters = %+v", report)
	}

	exported := m.Build()
	joined := joinContents(exported)
	if !strings.Contains(joined, "Compacted context summary") {
		t.Fatalf("exported context missing archive: %q", joined)
	}
}

func joinContents(msgs []types.Message) string {
	var b strings.Builder
	for _, msg := range msgs {
		b.WriteString(msg.Content)
		b.WriteByte('\n')
	}
	return b.String()
}
