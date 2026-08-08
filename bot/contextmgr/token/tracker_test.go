package token

import "testing"

func TestTracker_RecordPrompt(t *testing.T) {
	var tr Tracker
	tr.RecordPrompt(1000)
	if tr.PromptEstimate() <= 0 {
		t.Error("after RecordUsage, PromptEstimate should use API data")
	}
}

func TestTracker_AddNew(t *testing.T) {
	var tr Tracker
	tr.RecordPrompt(1000) // calibrate
	tr.AddNew(400)        // ~100 new tokens
	if tr.PromptEstimate() <= 1000 {
		t.Error("AddNew should increase estimate beyond baseline")
	}
}

func TestTracker_ResetOnRecord(t *testing.T) {
	var tr Tracker
	tr.RecordPrompt(100)
	tr.AddNew(1000) // add pending tokens
	estBefore := tr.PromptEstimate()
	tr.RecordPrompt(200) // new API call resets pending
	if tr.PromptEstimate() >= estBefore {
		t.Error("new RecordUsage should reset newMessageTokens, lowering estimate")
	}
}

func TestTracker_CacheStats(t *testing.T) {
	var tr Tracker
	h, m := tr.CacheStats()
	if h != 0 || m != 0 {
		t.Error("initial cache stats should be zero")
	}
	tr.RecordCache(100, 50)
	h, m = tr.CacheStats()
	if h != 100 || m != 50 {
		t.Errorf("after record: hit=%d miss=%d, want 100/50", h, m)
	}
}

func TestTracker_CacheHitRatio(t *testing.T) {
	var tr Tracker
	if r := tr.CacheHitRatio(); r != 0 {
		t.Error("initial ratio should be 0")
	}
	tr.RecordCache(75, 25)
	if r := tr.CacheHitRatio(); r != 0.75 {
		t.Errorf("ratio = %f, want 0.75", r)
	}
}

func TestTracker_NoAPIData(t *testing.T) {
	var tr Tracker
	if tr.PromptEstimate() != 0 {
		t.Error("without API data, PromptEstimate should be 0")
	}
}

func TestTrackerResetModelClearsPromptCalibration(t *testing.T) {
	var tracker Tracker
	tracker.RecordPrompt(1000)
	tracker.RecordCache(800, 200)
	tracker.AddEstimated(50)
	tracker.ResetModel()
	if got := tracker.PromptEstimate(); got != 0 {
		t.Fatalf("prompt estimate after model reset = %d, want 0", got)
	}
	if hit, miss := tracker.CacheStats(); hit != 0 || miss != 0 {
		t.Fatalf("cache after model reset = %d/%d, want 0/0", hit, miss)
	}
}

func TestTrackerSnapshotRestore(t *testing.T) {
	var tr Tracker
	tr.RecordPrompt(1000)
	tr.AddNew(400)
	tr.RecordCache(75, 25)
	tr.RecordSubagent(300, 20, 10)

	var restored Tracker
	restored.Restore(tr.Snapshot())

	hit, miss := restored.CacheStats()
	if hit != 75 || miss != 25 {
		t.Fatalf("cache stats = %d/%d, want 75/25", hit, miss)
	}
	if got := restored.PromptEstimate(); got != 1100 {
		t.Fatalf("prompt estimate = %d, want 1100", got)
	}
	sub := restored.SubStats()
	if sub.Count != 1 || sub.TotalTokens != 300 || sub.CacheHitTokens != 20 || sub.CacheMissTokens != 10 {
		t.Fatalf("sub stats = %+v", sub)
	}
}
