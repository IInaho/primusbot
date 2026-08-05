package lspcore

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"
)

func TestConnBidirectional(t *testing.T) {
	caR, caW := io.Pipe()
	acR, acW := io.Pipe()
	// Close the writers at the end so both readLoop goroutines see EOF and exit
	// (in production the subprocess pipe EOFs on kill; here nothing else closes it).
	defer caW.Close()
	defer acW.Close()

	notif := make(chan string, 4)
	var client *conn
	client = newConn(caW, acR,
		func(method string, _ json.RawMessage) { notif <- method },
		func(id int64, _ string, _ json.RawMessage) { _ = client.reply(id, map[string]any{"ok": true}) })

	var server *conn
	server = newConn(acW, caR,
		func(string, json.RawMessage) {},
		func(id int64, method string, _ json.RawMessage) { _ = server.reply(id, map[string]any{"echo": method}) })

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	res, err := client.call(ctx, "ping", map[string]any{"x": 1})
	if err != nil {
		t.Fatalf("client call: %v", err)
	}
	if !strings.Contains(string(res), `"echo":"ping"`) {
		t.Fatalf("unexpected response: %s", res)
	}

	if err := server.notify("textDocument/publishDiagnostics", map[string]any{}); err != nil {
		t.Fatalf("server notify: %v", err)
	}
	select {
	case m := <-notif:
		if m != "textDocument/publishDiagnostics" {
			t.Fatalf("notify method = %q", m)
		}
	case <-ctx.Done():
		t.Fatal("notification not delivered")
	}

	sres, err := server.call(ctx, "workspace/configuration", nil)
	if err != nil {
		t.Fatalf("server→client call: %v", err)
	}
	if !strings.Contains(string(sres), `"ok":true`) {
		t.Fatalf("server→client reply: %s", sres)
	}
}

func TestReadFrame(t *testing.T) {
	in := "Content-Length: 17\r\nContent-Type: x\r\n\r\n" + `{"jsonrpc":"2.0"}` + "Content-Length: 2\r\n\r\n{}"
	r := bufio.NewReader(strings.NewReader(in))
	first, err := readFrame(r)
	if err != nil || string(first) != `{"jsonrpc":"2.0"}` {
		t.Fatalf("first frame = %q, err %v", first, err)
	}
	second, err := readFrame(r)
	if err != nil || string(second) != `{}` {
		t.Fatalf("second frame = %q, err %v", second, err)
	}
	if _, err := readFrame(r); err == nil {
		t.Fatal("expected EOF on third read")
	}
}

func TestReadFrameCapsOversizedBody(t *testing.T) {
	in := "Content-Length: 99999999999\r\n\r\n{}"
	r := bufio.NewReader(strings.NewReader(in))
	if _, err := readFrame(r); err == nil || !strings.Contains(err.Error(), "frame cap") {
		t.Fatalf("expected frame cap error, got %v", err)
	}
}
