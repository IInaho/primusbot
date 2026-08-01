package web

import (
	"context"
	"net"
	"net/http"
	"testing"
)

func TestWebFetchTool(t *testing.T) {
	wf := &WebFetchTool{}
	_, err := wf.Execute(context.Background(), nil)
	if err == nil {
		t.Error("expected error for missing url")
	}
}

func TestValidateURLRejectsPrivateNetworks(t *testing.T) {
	for _, rawURL := range []string{
		"http://127.0.0.1/",
		"http://169.254.169.254/latest/meta-data/",
		"http://10.0.0.1/",
		"http://[::1]/",
		"http://100.64.0.1/",
	} {
		if err := validateURL(rawURL); err == nil {
			t.Errorf("validateURL(%q) allowed a private address", rawURL)
		}
	}
	if isPrivateIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("public address was classified as private")
	}
}

func TestWebFetchRedirectRejectsPrivateTarget(t *testing.T) {
	client := NewWebFetchTool().client
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/private", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.CheckRedirect(req, nil); err == nil {
		t.Fatal("redirect to private network was allowed")
	}
}

func TestWebFetchIgnoresEnvironmentProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://127.0.0.1:7897")
	t.Setenv("HTTPS_PROXY", "http://127.0.0.1:7897")

	client := NewWebFetchTool().client
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transport type = %T, want *http.Transport", client.Transport)
	}
	if transport.Proxy != nil {
		t.Fatal("web_fetch must connect directly so destination-IP SSRF checks cannot be bypassed by a proxy")
	}
}
