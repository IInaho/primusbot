package web

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"nekocode/bot/tools/runtime/core"
	"nekocode/bot/tools/runtime/toolutil"
	utilhttp "nekocode/util/http"
	"nekocode/util/text"
)

type WebFetchTool struct {
	toolutil.SafeReadOnlyTool
	client *http.Client
}

func NewWebFetchTool() *WebFetchTool {
	transport := utilhttp.NewSharedTransport()
	// Deliberately ignore process proxy variables. The custom DialContext
	// validates and pins the actual destination IP; an HTTP proxy would make
	// it validate only the proxy address and would weaken the SSRF boundary.
	transport.Proxy = nil
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("invalid destination: %w", err)
		}
		ips, err := net.DefaultResolver.LookupIP(ctx, "ip", host)
		if err != nil {
			return nil, fmt.Errorf("resolve destination: %w", err)
		}
		var lastErr error
		for _, ip := range ips {
			if isPrivateIP(ip) {
				return nil, fmt.Errorf("private network access denied")
			}
			conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			if err == nil {
				return conn, nil
			}
			lastErr = err
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, fmt.Errorf("destination has no public IP address")
	}
	c := &http.Client{Transport: transport, Timeout: 15 * time.Second}
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 {
			return fmt.Errorf("too many redirects")
		}
		return validateURL(req.URL.String())
	}
	return &WebFetchTool{client: c}
}

func (t *WebFetchTool) Name() string { return "web_fetch" }

func (t *WebFetchTool) Description() string {
	return "Fetch a public web page as text; private, intranet, and loopback addresses are rejected. When quoting, cite the source URL and keep quotes ≤125 characters."
}

func (t *WebFetchTool) Parameters() []core.Parameter {
	return []core.Parameter{
		{Name: "url", Type: "string", Required: true, Description: "Web page URL to fetch"},
		{Name: "prompt", Type: "string", Required: false, Description: "Content extraction hint, e.g. 'extract API parameters'"},
	}
}

func (t *WebFetchTool) Execute(ctx context.Context, args map[string]any) (string, error) {
	rawURL, err := toolutil.RequireStringArg(args, "url")
	if err != nil {
		return "", err
	}

	if err := validateURL(rawURL); err != nil {
		return "", fmt.Errorf("URL validation failed: %w", err)
	}

	prompt := toolutil.OptStringArg(args, "prompt", "")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("User-Agent", "NekoCode/1.0")
	req.Header.Set("Accept", "text/html,text/plain,*/*")

	resp, err := t.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	contentType := resp.Header.Get("Content-Type")
	var content string
	if strings.Contains(contentType, "text/html") {
		content = html2md(string(body))
	} else {
		content = string(body)
	}

	content = toolutil.StripAnsi(content)

	if content == "" {
		return "Page content is empty", nil
	}

	if prompt != "" {
		content = extractRelevant(content, prompt)
	}

	content = text.TruncateByRune(content, 3000)
	return content, nil
}

func validateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %v", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("only http/https allowed")
	}

	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("missing hostname")
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("private network access denied")
		}
	} else {
		ips, err := net.LookupIP(host)
		if err != nil {
			return fmt.Errorf("DNS lookup failed: %v", err)
		}
		for _, ip := range ips {
			if isPrivateIP(ip) {
				return fmt.Errorf("private network access denied")
			}
		}
	}

	return nil
}

func isPrivateIP(ip net.IP) bool {
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	privateBlocks := []string{
		"100.64.0.0/10",
		"198.18.0.0/15",
		"fec0::/10",
	}
	for _, cidr := range privateBlocks {
		_, block, _ := net.ParseCIDR(cidr)
		if block != nil && block.Contains(ip) {
			return true
		}
	}
	return false
}

func extractRelevant(content, prompt string) string {
	keywords := strings.Fields(prompt)
	if len(keywords) == 0 {
		return content
	}

	paragraphs := strings.Split(content, "\n\n")
	var relevant []string
	for _, p := range paragraphs {
		pLower := strings.ToLower(p)
		for _, kw := range keywords {
			if strings.Contains(pLower, strings.ToLower(kw)) {
				relevant = append(relevant, p)
				break
			}
		}
	}
	if len(relevant) == 0 {
		return content
	}
	return strings.Join(relevant, "\n\n")
}
