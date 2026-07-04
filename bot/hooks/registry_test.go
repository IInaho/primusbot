package hooks

import "testing"

func TestRegistryCountsAndGovernanceStatsReset(t *testing.T) {
	r := NewRegistry()
	r.Register(Hook{
		Name:  "hint",
		Point: PreTurn,
		On: func(s State) *Result {
			return &Result{Hint: &Hint{Type: "test", Content: "ok"}}
		},
	})

	results := r.Evaluate(PreTurn, "", false)
	if len(results) != 1 {
		t.Fatalf("results = %d, want 1", len(results))
	}
	counts := r.HookCountsSnapshot()
	if counts.Evaluations != 1 || counts.Hints != 1 {
		t.Fatalf("counts = %+v, want one evaluation and hint", counts)
	}

	_ = r.GovernanceStats()
	counts = r.HookCountsSnapshot()
	if counts != (HookCounts{}) {
		t.Fatalf("counts after GovernanceStats = %+v, want zero", counts)
	}
	if audit := r.HookAuditSnapshot(); len(audit) != 1 || audit[0].Hook != "hint" || audit[0].Action != "hint" {
		t.Fatalf("audit after GovernanceStats = %+v, want preserved hint event", audit)
	}
}

func TestEmptyRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("NewRegistry returned nil")
	}
	if len(r.List()) != 0 {
		t.Error("expected empty hook list")
	}
}

func TestRegistryStatePatchWritesBackPolicyKeys(t *testing.T) {
	r := NewRegistry()
	r.Register(Hook{
		Name:  "patch",
		Point: PreTurn,
		On: func(s State) *Result {
			return &Result{StatePatch: &StatePatch{
				Ints:    map[string]int64{"policy:block": 1},
				Strings: map[string]string{"policy:reason": "blocked"},
			}}
		},
	})

	r.Evaluate(PreTurn, "", false)
	if got := r.store["policy:block"]; got != 1 {
		t.Fatalf("policy int patch = %d, want 1", got)
	}
	if got := r.strVals["policy:reason"]; got != "blocked" {
		t.Fatalf("policy string patch = %q, want blocked", got)
	}
}

func TestResetSession(t *testing.T) {
	r := NewRegistry()
	r.Set(StoreToolResultCount, 3)
	r.Set(StoreQuotaReads, 5)
	r.Register(Hook{
		Name:  "hint",
		Point: PreTurn,
		On: func(s State) *Result {
			return &Result{Hint: &Hint{Type: "test", Content: "ok"}}
		},
	})
	r.Evaluate(PreTurn, "", false)

	r.ResetSession()
	if r.store[StoreToolResultCount] != 0 || r.store[StoreQuotaReads] != 0 {
		t.Error("store should be empty after session reset")
	}
	if audit := r.HookAuditSnapshot(); len(audit) != 0 {
		t.Fatalf("audit after session reset = %+v, want empty", audit)
	}
}

func TestRegistryHookAuditRecordsAction(t *testing.T) {
	r := NewRegistry()
	r.Register(Hook{
		Name:  "block",
		Point: PreToolUse,
		On: func(s State) *Result {
			return &Result{BlockTool: &BlockTool{Tool: "edit", Reason: "read first"}}
		},
	})

	r.Evaluate(PreToolUse, "edit", false)
	audit := r.HookAuditSnapshot()
	if len(audit) != 1 {
		t.Fatalf("audit len = %d, want 1", len(audit))
	}
	if audit[0].Hook != "block" || audit[0].Point != PreToolUse || audit[0].Action != "block_tool" || audit[0].Detail != "read first" {
		t.Fatalf("audit = %+v", audit[0])
	}
}

func TestResetTurnClearsOnlyTurnScopedKeys(t *testing.T) {
	r := NewRegistry()
	r.Set(KeyPrefixCounter+"x", 1)
	r.Set(KeyPrefixPolicy+"x", 1)
	r.Set(KeyPrefixSession+"x", 1)
	r.Set(KeyPrefixGauge+"x", 1)
	r.Set(KeyPrefixValue+"x", 1)
	r.Set(KeyPrefixTurn+"x", 1)
	r.Set(KeyPrefixFlag+"x", 1)
	r.SetStr(KeyPrefixValue+"sx", "value")
	r.SetStr(KeyPrefixTurn+"sx", "turn")
	r.SetStr(KeyPrefixSession+"sx", "session")

	r.ResetTurn()

	if r.store[KeyPrefixCounter+"x"] != 1 || r.store[KeyPrefixPolicy+"x"] != 1 || r.store[KeyPrefixSession+"x"] != 1 {
		t.Fatalf("persistent keys after ResetTurn = %+v", r.store)
	}
	for _, k := range []string{KeyPrefixGauge + "x", KeyPrefixValue + "x", KeyPrefixTurn + "x", KeyPrefixFlag + "x"} {
		if _, ok := r.store[k]; ok {
			t.Fatalf("turn-scoped key %q was not cleared", k)
		}
	}
	if _, ok := r.strVals[KeyPrefixValue+"sx"]; ok {
		t.Fatal("value string key was not cleared")
	}
	if _, ok := r.strVals[KeyPrefixTurn+"sx"]; ok {
		t.Fatal("turn string key was not cleared")
	}
	if r.strVals[KeyPrefixSession+"sx"] != "session" {
		t.Fatalf("session string key = %q, want session", r.strVals[KeyPrefixSession+"sx"])
	}
}

func TestRegistryUnregisterWhere(t *testing.T) {
	r := NewRegistry()
	r.Register(Hook{Name: "keep", Point: PreTurn})
	r.Register(Hook{Name: "drop", Point: PreTurn})

	r.UnregisterWhere(func(h Hook) bool { return h.Name == "drop" })
	hooks := r.List()
	if len(hooks) != 1 || hooks[0].Name != "keep" {
		t.Fatalf("hooks = %+v, want only keep", hooks)
	}
}
