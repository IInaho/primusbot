package qqbot

import (
	"testing"
)

func TestParseFrameHello(t *testing.T) {
	frame, ok := parseFrame([]byte(`{"op":10,"d":{"heartbeat_interval":41250}}`))
	if !ok {
		t.Fatal("parseFrame() ok = false")
	}
	if frame.Op != opHello {
		t.Fatalf("Op = %d, want %d", frame.Op, opHello)
	}
	interval, ok := frame.helloInterval()
	if !ok || interval != 41250 {
		t.Fatalf("helloInterval() = %d, %v", interval, ok)
	}

	// 缺失 heartbeat_interval 的 Hello 视为非法。
	bad, _ := parseFrame([]byte(`{"op":10,"d":{}}`))
	if _, ok := bad.helloInterval(); ok {
		t.Fatal("helloInterval() ok = true, want false")
	}
}

func TestParseFrameDispatchReady(t *testing.T) {
	frame, ok := parseFrame([]byte(`{
		"op": 0,
		"s": 42,
		"t": "READY",
		"d": {"session_id": "sess-abc", "user": {"id": "bot"}}
	}`))
	if !ok {
		t.Fatal("parseFrame() ok = false")
	}
	if frame.Op != opDispatch || frame.T != "READY" || frame.S != 42 {
		t.Fatalf("unexpected frame: %+v", frame)
	}
	if got := frame.readySessionID(); got != "sess-abc" {
		t.Fatalf("readySessionID() = %q", got)
	}
}

func TestParseFrameInvalidSession(t *testing.T) {
	resumable, _ := parseFrame([]byte(`{"op":9,"d":true}`))
	if !resumable.invalidSessionResumable() {
		t.Fatal("d=true should be resumable")
	}
	fresh, _ := parseFrame([]byte(`{"op":9,"d":false}`))
	if fresh.invalidSessionResumable() {
		t.Fatal("d=false should not be resumable")
	}
	// d 缺失按不可恢复处理。
	missing, _ := parseFrame([]byte(`{"op":9}`))
	if missing.invalidSessionResumable() {
		t.Fatal("missing d should not be resumable")
	}
}

func TestParseFrameNotJSON(t *testing.T) {
	if _, ok := parseFrame([]byte(`not-json`)); ok {
		t.Fatal("parseFrame() ok = true, want false")
	}
}
