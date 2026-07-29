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

func TestSendHTMLFallsBackToPlainOnParseFailure(t *testing.T) {
	var bodies []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		bodies = append(bodies, body)
		if len(bodies) == 1 {
			w.WriteHeader(400)
			_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: can't parse entities"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true,"result":{}}`))
	}))
	defer server.Close()

	client := newAPIClient("token")
	client.base = server.URL

	if err := client.sendMessage(context.Background(), 123, "<b>broken"); err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 {
		t.Fatalf("requests = %d, want 2 (html + plain retry)", len(bodies))
	}
	if _, ok := bodies[1]["parse_mode"]; ok {
		t.Fatalf("plain retry should omit parse_mode, got %#v", bodies[1]["parse_mode"])
	}
}

func TestEditMessageSwallowsNotModified(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = w.Write([]byte(`{"ok":false,"description":"Bad Request: message is not modified"}`))
	}))
	defer server.Close()

	client := newAPIClient("token")
	client.base = server.URL

	if err := client.editMessage(context.Background(), 1, 2, "same", &emptyKeyboard); err != nil {
		t.Fatalf("not-modified should be swallowed: %v", err)
	}
}

func TestSendMessageWithKeyboardReturnsMessageID(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"result":{"message_id":777}}`))
	}))
	defer server.Close()

	client := newAPIClient("token")
	client.base = server.URL

	id, err := client.sendMessageWithKeyboard(context.Background(), 1, "text", emptyKeyboard)
	if err != nil {
		t.Fatal(err)
	}
	if id != 777 {
		t.Fatalf("message id = %d, want 777", id)
	}
}
