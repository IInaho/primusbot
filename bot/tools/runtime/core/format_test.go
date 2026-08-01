package core

import "testing"

func TestToToolDefs(t *testing.T) {
	defs := ToToolDefs(nil)
	if len(defs) != 0 {
		t.Error("expected empty")
	}

	descs := []Descriptor{{
		Name: "test", Description: "a test tool",
		Parameters: []Parameter{
			{Name: "path", Type: "string", Required: true, Description: "file path"},
			{
				Name: "items", Type: "array", Required: false, Description: "work items",
				Items: &Schema{
					Type: "object", Required: []string{"status"},
					Properties: map[string]Schema{
						"status": {Type: "string", Enum: []string{"pending", "done"}},
					},
				},
			},
		},
	}}
	defs = ToToolDefs(descs)
	if len(defs) != 1 {
		t.Fatal("expected 1 def")
	}
	d := defs[0]
	if d.Type != "function" {
		t.Error("bad type")
	}
	if d.Function.Name != "test" {
		t.Error("bad name")
	}
	if d.Function.Parameters.Type != "object" {
		t.Error("bad params type")
	}
	if d.Function.Parameters.Required[0] != "path" {
		t.Error("bad required")
	}
	if len(d.Function.Parameters.Properties) != 2 {
		t.Error("bad props count")
	}
	items := d.Function.Parameters.Properties["items"].Items
	if items == nil || items.Type != "object" || len(items.Required) != 1 || items.Properties["status"].Enum[1] != "done" {
		t.Fatalf("nested schema was not preserved: %+v", items)
	}
}

func TestFormatArgs(t *testing.T) {
	got := FormatArgs(map[string]any{
		"b":             "plain",
		"a":             "x,y",
		"_preview":      "hidden",
		"_sub_callback": "hidden",
	})
	want := `a="x,y",b=plain`
	if got != want {
		t.Fatalf("FormatArgs() = %q, want %q", got, want)
	}
}

func TestToolCallResultEffectiveOutput(t *testing.T) {
	if got := (ToolCallResult{Output: "ok"}).EffectiveOutput(); got != "ok" {
		t.Fatalf("EffectiveOutput() = %q, want ok", got)
	}
	if got := (ToolCallResult{Output: "ok", Error: "bad"}).EffectiveOutput(); got != "bad" {
		t.Fatalf("EffectiveOutput() = %q, want bad", got)
	}
}

func TestExecutionModeAliases(t *testing.T) {
	if ModeParallel == ModeSequential {
		t.Fatal("execution mode aliases collapsed")
	}
}
