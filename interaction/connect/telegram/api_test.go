package telegram

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSendMessageUsesHTMLParseMode(t *testing.T) {
	var body map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	client := newAPIClient("token")
	client.base = server.URL

	if err := client.sendMessage(context.Background(), 123, "<b>Done</b>"); err != nil {
		t.Fatal(err)
	}
	if body["parse_mode"] != "HTML" {
		t.Fatalf("parse_mode = %#v", body["parse_mode"])
	}
	if body["disable_web_page_preview"] != true {
		t.Fatalf("disable_web_page_preview = %#v", body["disable_web_page_preview"])
	}
}
