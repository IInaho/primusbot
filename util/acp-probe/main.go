// Command acp-probe is an end-to-end conformance checker for NekoCode's ACP
// (Agent Client Protocol) agent. It spawns the agent as a child process,
// speaks JSON-RPC over stdio exactly like a real editor client (Zed), and
// verifies every protocol method plus the error paths.
//
// Usage:
//
//	go build -o bin/nekocode-tui ./cmd/tui
//	go run ./util/acp-probe -agent "bin/nekocode-tui --acp --allow-client-mcp"
//
// Run with -with-prompt to also exercise a real model turn (requires the
// agent to have a working model configuration).
//
// The binary doubles as a minimal stdio MCP echo server when started with
// -mcp-echo; the probe uses that mode to verify the agent's baseline stdio
// MCP support without external dependencies.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
	"strings"
	"sync"
	"time"
)

const protocolVersion = 1

type rpcMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("rpc %d: %s", e.Code, e.Message) }

// probe is a minimal ACP client bound to one agent process.
type probe struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout io.ReadCloser

	writeMu sync.Mutex
	mu      sync.Mutex
	pending map[string]chan *rpcMessage

	updateMu sync.Mutex
	updates  []updateEvent

	cwd      string
	session1 string
	session2 string
	session3 string

	// session config state captured from session/new's configOptions.
	activeModelName string
	currentEffort   string
	hasEffortOption bool

	timeout time.Duration
	verbose bool
}

type updateEvent struct {
	Kind       string
	ToolCallID string
}

func main() {
	agentCmd := flag.String("agent", "nekocode-tui --acp --allow-client-mcp", "agent command line (spawned per protocol over stdio)")
	cwd := flag.String("cwd", "", "working directory for the agent and sessions (default: current directory)")
	timeout := flag.Duration("timeout", 30*time.Second, "per-request timeout")
	withPrompt := flag.Bool("with-prompt", false, "also run a real model turn via session/prompt")
	verbose := flag.Bool("v", false, "print every session/update notification")
	mcpEcho := flag.Bool("mcp-echo", false, "run as a minimal stdio MCP echo server (internal)")
	flag.Parse()

	if *mcpEcho {
		if err := runMCPEcho(); err != nil {
			fmt.Fprintln(os.Stderr, "mcp-echo:", err)
			os.Exit(1)
		}
		return
	}

	workDir := *cwd
	if workDir == "" {
		if dir, err := os.Getwd(); err == nil {
			workDir = dir
		}
	}

	p, err := startProbe(strings.Fields(*agentCmd), workDir, *timeout, *verbose)
	if err != nil {
		fmt.Fprintln(os.Stderr, "start agent:", err)
		os.Exit(1)
	}
	defer p.close()

	steps := probeSteps(*withPrompt)
	ctx := context.Background()
	passed, failed := 0, 0
	for _, step := range steps {
		fmt.Printf("[%02d] %-46s ", step.index, step.name)
		err := step.fn(ctx, p)
		switch {
		case err == nil:
			fmt.Println("PASS")
			passed++
		default:
			fmt.Printf("FAIL: %v\n", err)
			failed++
		}
	}

	p.printUpdateSummary()
	fmt.Printf("\n%d passed, %d failed\n", passed, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

type step struct {
	index int
	name  string
	fn    func(context.Context, *probe) error
}

func probeSteps(withPrompt bool) []step {
	steps := []step{
		{1, "rejects calls before initialize", func(ctx context.Context, p *probe) error {
			return p.expectErrorCode(ctx, newID(1), "session/list", nil, -32002)
		}},
		{2, "initialize handshake", func(ctx context.Context, p *probe) error {
			return p.checkInitialize(ctx)
		}},
		{3, "rejects duplicate initialize", func(ctx context.Context, p *probe) error {
			return p.expectErrorCode(ctx, newID(2), "initialize", map[string]any{"protocolVersion": 1}, -32600)
		}},
		{4, "unknown method -> -32601", func(ctx context.Context, p *probe) error {
			return p.expectErrorCode(ctx, newID(3), "no/such-method", nil, -32601)
		}},
		{5, "malformed JSON line -> -32700", func(ctx context.Context, p *probe) error {
			return p.expectParseError(ctx)
		}},
		{6, "wrong jsonrpc version -> -32600", func(ctx context.Context, p *probe) error {
			return p.expectErrorCode(ctx, newID(4), "session/list", nil, -32600, rawLine(`{"jsonrpc":"1.0","id":4,"method":"session/list"}`))
		}},
		{7, "session/new creates a session", func(ctx context.Context, p *probe) error {
			id, err := p.newSession(ctx, nil)
			if err != nil {
				return err
			}
			p.session1 = id
			return nil
		}},
		{8, "session/new rejects http MCP", func(ctx context.Context, p *probe) error {
			servers := []any{map[string]any{"type": "http", "name": "remote", "url": "https://mcp.example.com"}}
			return p.expectErrorCode(ctx, newID(5), "session/new", p.sessionParams(servers), -32602)
		}},
		{9, "session/new registers stdio MCP server", func(ctx context.Context, p *probe) error {
			self, err := os.Executable()
			if err != nil {
				return err
			}
			servers := []any{map[string]any{"name": "echo", "command": self, "args": []string{"-mcp-echo"}}}
			id, err := p.newSession(ctx, servers)
			if err != nil {
				return err
			}
			p.session2 = id
			return nil
		}},
		{10, "large integer request ID round trip", func(ctx context.Context, p *probe) error {
			const big = "9223372036854775807"
			msg, err := p.request(ctx, json.RawMessage(big), "session/list", nil)
			if err != nil {
				return err
			}
			if msg.Error != nil {
				return msg.Error
			}
			if string(msg.ID) != big {
				return fmt.Errorf("response id = %s, want %s", msg.ID, big)
			}
			return nil
		}},
		{11, "session/list without params returns array", func(ctx context.Context, p *probe) error {
			return p.checkList(ctx, nil, false)
		}},
		{12, "session/list filtered by cwd returns array", func(ctx context.Context, p *probe) error {
			return p.checkList(ctx, map[string]any{"cwd": p.cwd}, false)
		}},
		{13, "session/list rejects unknown cursor", func(ctx context.Context, p *probe) error {
			return p.expectErrorCode(ctx, newID(6), "session/list", map[string]any{"cursor": "nope"}, -32602)
		}},
		{14, "session/load unknown session -> -32602", func(ctx context.Context, p *probe) error {
			params := p.sessionParams(nil)
			params["sessionId"] = "does-not-exist"
			return p.expectErrorCode(ctx, newID(7), "session/load", params, -32602)
		}},
		// The agent only persists sessions once they contain messages, so a
		// freshly created empty session is not loadable yet. This step pins
		// that documented storage behavior.
		{15, "session/load empty session not persisted -> -32602", func(ctx context.Context, p *probe) error {
			params := p.sessionParams(nil)
			params["sessionId"] = p.session1
			return p.expectErrorCode(ctx, newID(8), "session/load", params, -32602)
		}},
		{16, "session/prompt unknown session -> -32602", func(ctx context.Context, p *probe) error {
			params := map[string]any{
				"sessionId": "does-not-exist",
				"prompt":    []any{map[string]any{"type": "text", "text": "hi"}},
			}
			return p.expectErrorCode(ctx, newID(9), "session/prompt", params, -32602)
		}},
		{17, "session/cancel on idle session succeeds", func(ctx context.Context, p *probe) error {
			_, err := p.call(ctx, newID(10), "session/cancel", map[string]any{"sessionId": p.session1})
			return err
		}},
		{18, "session/delete un-persisted session -> -32602", func(ctx context.Context, p *probe) error {
			return p.expectErrorCode(ctx, newID(11), "session/delete", map[string]any{"sessionId": p.session2}, -32602)
		}},
		{19, "session/new returns configOptions", func(ctx context.Context, p *probe) error {
			return p.checkConfigOptions(ctx)
		}},
		{20, "set_config_option round trip (idempotent)", func(ctx context.Context, p *probe) error {
			return p.setConfigRoundTrip(ctx)
		}},
		{21, "set_config_option rejects unknown ids -> -32602", func(ctx context.Context, p *probe) error {
			bad := map[string]any{"sessionId": p.session3, "configId": "nope", "value": "x"}
			if err := p.expectErrorCode(ctx, newID(12), "session/set_config_option", bad, -32602); err != nil {
				return err
			}
			badModel := map[string]any{"sessionId": p.session3, "configId": "model", "value": "missing-model"}
			return p.expectErrorCode(ctx, newID(13), "session/set_config_option", badModel, -32602)
		}},
	}

	if withPrompt {
		steps = append(steps,
			step{22, "session/prompt full turn (needs model)", func(ctx context.Context, p *probe) error {
				return p.runPromptTurn(ctx)
			}},
			step{23, "session/load replays the turn", func(ctx context.Context, p *probe) error {
				return p.checkLoadReplay(ctx)
			}},
			step{24, "session/list includes persisted session", func(ctx context.Context, p *probe) error {
				return p.checkList(ctx, map[string]any{"cwd": p.cwd}, true)
			}},
			step{25, "session/delete persisted session succeeds", func(ctx context.Context, p *probe) error {
				_, err := p.call(ctx, newID(14), "session/delete", map[string]any{"sessionId": p.session1})
				return err
			}},
		)
	}
	for i := range steps {
		steps[i].index = i + 1
	}
	return steps
}

func startProbe(argv []string, cwd string, timeout time.Duration, verbose bool) (*probe, error) {
	if len(argv) == 0 {
		return nil, errors.New("empty agent command")
	}
	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Dir = cwd
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("spawn %s: %w", strings.Join(argv, " "), err)
	}
	p := &probe{
		cmd:     cmd,
		stdin:   stdin,
		stdout:  stdout,
		pending: make(map[string]chan *rpcMessage),
		timeout: timeout,
		verbose: verbose,
		cwd:     cwd,
	}
	go p.readLoop()
	return p, nil
}

func (p *probe) close() {
	_ = p.stdin.Close()
	if p.cmd.Process != nil {
		done := make(chan struct{})
		go func() { _ = p.cmd.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			_ = p.cmd.Process.Kill()
			<-done
		}
	}
}

func (p *probe) logf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "acp-probe: "+format+"\n", args...)
}

// --- client plumbing ---------------------------------------------------------

func newID(n uint64) json.RawMessage {
	return json.RawMessage(fmt.Sprintf("%d", n))
}

func (p *probe) write(value any) error {
	body, err := json.Marshal(value)
	if err != nil {
		return err
	}
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	_, err = p.stdin.Write(append(body, '\n'))
	return err
}

func (p *probe) writeRaw(line string) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	_, err := p.stdin.Write([]byte(line + "\n"))
	return err
}

func (p *probe) request(ctx context.Context, id json.RawMessage, method string, params any) (*rpcMessage, error) {
	return p.requestWithTimeout(ctx, id, method, params, p.timeout)
}

func (p *probe) requestWithTimeout(ctx context.Context, id json.RawMessage, method string, params any, timeout time.Duration) (*rpcMessage, error) {
	if id == nil {
		return nil, errors.New("request needs an id")
	}
	body := map[string]any{"jsonrpc": "2.0", "id": id, "method": method}
	if params != nil {
		body["params"] = params
	}
	key := string(id)
	reply := make(chan *rpcMessage, 1)
	p.mu.Lock()
	p.pending[key] = reply
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.pending, key)
		p.mu.Unlock()
	}()
	if err := p.write(body); err != nil {
		return nil, err
	}
	select {
	case msg := <-reply:
		return msg, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(timeout):
		return nil, fmt.Errorf("no response to %s within %s", method, timeout)
	}
}

// call issues a request and fails on any JSON-RPC error response.
func (p *probe) call(ctx context.Context, id json.RawMessage, method string, params any) (json.RawMessage, error) {
	msg, err := p.request(ctx, id, method, params)
	if err != nil {
		return nil, err
	}
	if msg.Error != nil {
		return nil, msg.Error
	}
	return msg.Result, nil
}

// rawLine lets a test override the exact wire bytes (e.g. invalid jsonrpc).
type rawLine string

func (p *probe) expectErrorCode(ctx context.Context, id json.RawMessage, method string, params any, wantCode int, override ...rawLine) error {
	var msg *rpcMessage
	if len(override) > 0 {
		// The agent echoes the request id back even for protocol-level
		// errors, so route the raw line's id through the normal pending map.
		var raw struct {
			ID json.RawMessage `json:"id"`
		}
		if err := json.Unmarshal([]byte(override[0]), &raw); err != nil {
			return err
		}
		key := string(raw.ID)
		if key == "" {
			key = "null"
		}
		reply := make(chan *rpcMessage, 1)
		p.mu.Lock()
		p.pending[key] = reply
		p.mu.Unlock()
		defer func() {
			p.mu.Lock()
			delete(p.pending, key)
			p.mu.Unlock()
		}()
		if err := p.writeRaw(string(override[0])); err != nil {
			return err
		}
		select {
		case msg = <-reply:
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(p.timeout):
			return fmt.Errorf("no response to raw %s line", method)
		}
	} else {
		var err error
		msg, err = p.request(ctx, id, method, params)
		if err != nil {
			return err
		}
	}
	if msg.Error == nil {
		return fmt.Errorf("expected error %d, got result %s", wantCode, msg.Result)
	}
	if msg.Error.Code != wantCode {
		return fmt.Errorf("expected error %d, got %d (%s)", wantCode, msg.Error.Code, msg.Error.Message)
	}
	return nil
}

func (p *probe) readLoop() {
	scanner := bufio.NewScanner(p.stdout)
	scanner.Buffer(make([]byte, 64*1024), 16<<20)
	for scanner.Scan() {
		var msg rpcMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			p.logf("unparseable agent output: %v", err)
			continue
		}
		switch {
		case msg.Method == "session/update":
			p.recordUpdate(msg.Params)
		case msg.Method == "session/request_permission":
			p.answerPermission(&msg)
		case msg.Method != "":
			if p.verbose {
				p.logf("notification %s", msg.Method)
			}
		case len(msg.ID) != 0:
			p.resolve(&msg)
		default:
			p.logf("agent line without id or method: %s", scanner.Text())
		}
	}
}

func (p *probe) resolve(msg *rpcMessage) {
	p.mu.Lock()
	reply := p.pending[string(msg.ID)]
	p.mu.Unlock()
	if reply == nil {
		p.logf("response for unknown id %s", msg.ID)
		return
	}
	select {
	case reply <- msg:
	default:
	}
}

func (p *probe) answerPermission(msg *rpcMessage) {
	var params struct {
		Options []struct {
			OptionID string `json:"optionId"`
		} `json:"options"`
	}
	_ = json.Unmarshal(msg.Params, &params)
	option := "reject_once"
	for _, candidate := range params.Options {
		if candidate.OptionID == "allow_once" {
			option = "allow_once"
			break
		}
	}
	reply := map[string]any{
		"jsonrpc": "2.0",
		"id":      msg.ID,
		"result": map[string]any{
			"outcome": map[string]any{"outcome": "selected", "optionId": option},
		},
	}
	if err := p.write(reply); err != nil {
		p.logf("answer permission: %v", err)
	}
}

func (p *probe) recordUpdate(params json.RawMessage) {
	var payload struct {
		Update struct {
			SessionUpdate string `json:"sessionUpdate"`
			ToolCallID    string `json:"toolCallId"`
		} `json:"update"`
	}
	if err := json.Unmarshal(params, &payload); err != nil {
		return
	}
	p.updateMu.Lock()
	p.updates = append(p.updates, updateEvent{Kind: payload.Update.SessionUpdate, ToolCallID: payload.Update.ToolCallID})
	p.updateMu.Unlock()
	if p.verbose {
		p.logf("update %s %s", payload.Update.SessionUpdate, payload.Update.ToolCallID)
	}
}

func (p *probe) updateEvents() []updateEvent {
	p.updateMu.Lock()
	defer p.updateMu.Unlock()
	return append([]updateEvent(nil), p.updates...)
}

func (p *probe) resetUpdates() {
	p.updateMu.Lock()
	p.updates = nil
	p.updateMu.Unlock()
}

func (p *probe) printUpdateSummary() {
	counts := make(map[string]int)
	for _, event := range p.updateEvents() {
		counts[event.Kind]++
	}
	if len(counts) == 0 {
		fmt.Println("\nno session/update notifications received")
		return
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	sort.Strings(names)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		parts = append(parts, fmt.Sprintf("%s=%d", name, counts[name]))
	}
	fmt.Printf("\nsession/update summary: %s\n", strings.Join(parts, " "))
}

// --- step helpers ------------------------------------------------------------

// session fields are set by the steps as they create sessions.
func (p *probe) sessionParams(mcpServers []any) map[string]any {
	params := map[string]any{"cwd": p.cwd}
	if mcpServers == nil {
		mcpServers = []any{}
	}
	params["mcpServers"] = mcpServers
	return params
}

func (p *probe) newSession(ctx context.Context, mcpServers []any) (string, error) {
	result, err := p.call(ctx, newID(nextCounter()), "session/new", p.sessionParams(mcpServers))
	if err != nil {
		return "", err
	}
	var payload struct {
		SessionID string `json:"sessionId"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return "", err
	}
	if payload.SessionID == "" {
		return "", errors.New("empty sessionId")
	}
	return payload.SessionID, nil
}

func (p *probe) checkInitialize(ctx context.Context) error {
	result, err := p.call(ctx, newID(nextCounter()), "initialize", map[string]any{"protocolVersion": protocolVersion})
	if err != nil {
		return err
	}
	var payload struct {
		ProtocolVersion   int `json:"protocolVersion"`
		AgentCapabilities struct {
			LoadSession     bool `json:"loadSession"`
			MCPCapabilities struct {
				HTTP bool `json:"http"`
				SSE  bool `json:"sse"`
			} `json:"mcpCapabilities"`
			SessionCapabilities struct {
				List   *struct{} `json:"list"`
				Delete *struct{} `json:"delete"`
			} `json:"sessionCapabilities"`
		} `json:"agentCapabilities"`
		AgentInfo struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"agentInfo"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return err
	}
	if payload.ProtocolVersion != protocolVersion {
		return fmt.Errorf("protocolVersion = %d, want %d", payload.ProtocolVersion, protocolVersion)
	}
	if !payload.AgentCapabilities.LoadSession {
		return errors.New("loadSession capability missing")
	}
	if payload.AgentCapabilities.MCPCapabilities.HTTP || payload.AgentCapabilities.MCPCapabilities.SSE {
		return errors.New("http/sse MCP advertised but unsupported")
	}
	if payload.AgentCapabilities.SessionCapabilities.List == nil || payload.AgentCapabilities.SessionCapabilities.Delete == nil {
		return errors.New("session list/delete capabilities missing")
	}
	if payload.AgentInfo.Name == "" || payload.AgentInfo.Version == "" {
		return errors.New("agentInfo incomplete")
	}
	return nil
}

func (p *probe) checkList(ctx context.Context, params map[string]any, wantKnown bool) error {
	result, err := p.call(ctx, newID(nextCounter()), "session/list", params)
	if err != nil {
		return err
	}
	var payload struct {
		Sessions []struct {
			SessionID string `json:"sessionId"`
		} `json:"sessions"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return err
	}
	if !wantKnown {
		return nil
	}
	for _, session := range payload.Sessions {
		if session.SessionID == p.session1 {
			return nil
		}
	}
	return fmt.Errorf("session %s missing from %d listed", p.session1, len(payload.Sessions))
}

func (p *probe) expectParseError(ctx context.Context) error {
	// A parse error is answered with a null id; route it via pending["null"].
	reply := make(chan *rpcMessage, 1)
	p.mu.Lock()
	p.pending["null"] = reply
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.pending, "null")
		p.mu.Unlock()
	}()
	if err := p.writeRaw("not-json"); err != nil {
		return err
	}
	select {
	case msg := <-reply:
		if msg.Error == nil || msg.Error.Code != -32700 {
			return fmt.Errorf("expected -32700, got %+v", msg.Error)
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(p.timeout):
		return errors.New("no parse-error response")
	}
}

func (p *probe) runPromptTurn(ctx context.Context) error {
	// Create a fresh session so it is the agent's active session when the
	// first prompt arrives — exactly how a real client starts a thread.
	id, err := p.newSession(ctx, nil)
	if err != nil {
		return err
	}
	p.session1 = id
	p.resetUpdates()
	params := map[string]any{
		"sessionId": p.session1,
		"prompt":    []any{map[string]any{"type": "text", "text": "请只回复两个字母: ok"}},
	}
	msg, err := p.requestWithTimeout(ctx, newID(nextCounter()), "session/prompt", params, 3*time.Minute)
	if err != nil {
		return err
	}
	if msg.Error != nil {
		return fmt.Errorf("prompt turn needs a configured model (%v); run with a working model config", msg.Error)
	}
	var result struct {
		StopReason string `json:"stopReason"`
	}
	if err := json.Unmarshal(msg.Result, &result); err != nil {
		return err
	}
	if result.StopReason == "" {
		return fmt.Errorf("stopReason missing: %s", msg.Result)
	}
	sawMessage := false
	sawUsage := false
	for _, event := range p.updateEvents() {
		switch event.Kind {
		case "agent_message_chunk":
			sawMessage = true
		case "usage_update":
			sawUsage = true
		}
	}
	if !sawMessage {
		return errors.New("no agent_message_chunk received during turn")
	}
	if !sawUsage {
		return errors.New("no usage_update received during turn")
	}
	return p.checkToolCallPairing()
}

// checkToolCallPairing verifies every tool_call_update refers to a tool_call
// that was announced first.
func (p *probe) checkToolCallPairing() error {
	seen := make(map[string]bool)
	for _, event := range p.updateEvents() {
		switch event.Kind {
		case "tool_call":
			seen[event.ToolCallID] = true
		case "tool_call_update":
			if !seen[event.ToolCallID] {
				return fmt.Errorf("tool_call_update for unknown toolCallId %q", event.ToolCallID)
			}
		}
	}
	return nil
}

func (p *probe) checkLoadReplay(ctx context.Context) error {
	p.resetUpdates()
	params := p.sessionParams(nil)
	params["sessionId"] = p.session1
	if _, err := p.call(ctx, newID(nextCounter()), "session/load", params); err != nil {
		return err
	}
	kinds := make(map[string]bool)
	for _, event := range p.updateEvents() {
		kinds[event.Kind] = true
	}
	if !kinds["user_message_chunk"] {
		return errors.New("replay missing user_message_chunk")
	}
	if !kinds["agent_message_chunk"] {
		return errors.New("replay missing agent_message_chunk")
	}
	if !kinds["usage_update"] {
		return errors.New("replay missing usage_update after reconnect")
	}
	return p.checkToolCallPairing()
}

var counter struct {
	mu sync.Mutex
	n  uint64
}

func nextCounter() uint64 {
	counter.mu.Lock()
	defer counter.mu.Unlock()
	counter.n++
	return counter.n + 100 // keep clear of the hardcoded ids in early steps
}

// --- session config steps ----------------------------------------------------

type configOptionView struct {
	ID           string `json:"id"`
	Type         string `json:"type"`
	CurrentValue any    `json:"currentValue"`
}

// checkConfigOptions creates a fresh session and validates the configOptions
// in the response: a model selector, an optional reasoning effort selector,
// and the permission mode selector. The probe stays silent about boolean
// config support (like a conservative client), so full_access must come back
// as a select option.
func (p *probe) checkConfigOptions(ctx context.Context) error {
	result, err := p.call(ctx, newID(nextCounter()), "session/new", p.sessionParams(nil))
	if err != nil {
		return err
	}
	var payload struct {
		SessionID     string             `json:"sessionId"`
		ConfigOptions []configOptionView `json:"configOptions"`
	}
	if err := json.Unmarshal(result, &payload); err != nil {
		return err
	}
	if payload.SessionID == "" {
		return errors.New("empty sessionId")
	}
	p.session3 = payload.SessionID
	var hasModel, hasFullAccess bool
	for _, option := range payload.ConfigOptions {
		switch option.ID {
		case "model":
			hasModel = option.Type == "select"
			p.activeModelName, _ = option.CurrentValue.(string)
		case "reasoning_effort":
			p.hasEffortOption = option.Type == "select"
			p.currentEffort, _ = option.CurrentValue.(string)
		case "full_access":
			hasFullAccess = option.Type == "select"
			mode, _ := option.CurrentValue.(string)
			if mode != "manual" && mode != "full" {
				return fmt.Errorf("full_access current value = %q", mode)
			}
		}
	}
	if !hasModel {
		return errors.New("model select option missing")
	}
	if p.activeModelName == "" {
		return errors.New("model option has no current value")
	}
	if !hasFullAccess {
		return errors.New("full_access select option missing")
	}
	return nil
}

// setConfigRoundTrip exercises successful config writes. Model and effort are
// set to their current values (idempotent, no user-visible change); the
// permission mode is toggled via select value ids and restored.
func (p *probe) setConfigRoundTrip(ctx context.Context) error {
	set := func(params map[string]any) error {
		result, err := p.call(ctx, newID(nextCounter()), "session/set_config_option", params)
		if err != nil {
			return err
		}
		var payload struct {
			ConfigOptions []configOptionView `json:"configOptions"`
		}
		if err := json.Unmarshal(result, &payload); err != nil {
			return err
		}
		if len(payload.ConfigOptions) == 0 {
			return errors.New("response missing configOptions")
		}
		return nil
	}
	if err := set(map[string]any{"sessionId": p.session3, "configId": "model", "value": p.activeModelName}); err != nil {
		return fmt.Errorf("set model: %w", err)
	}
	if p.hasEffortOption {
		if err := set(map[string]any{"sessionId": p.session3, "configId": "reasoning_effort", "value": p.currentEffort}); err != nil {
			return fmt.Errorf("set effort: %w", err)
		}
	}
	if err := set(map[string]any{"sessionId": p.session3, "configId": "full_access", "value": "full"}); err != nil {
		return fmt.Errorf("enable full access: %w", err)
	}
	if err := set(map[string]any{"sessionId": p.session3, "configId": "full_access", "value": "manual"}); err != nil {
		return fmt.Errorf("disable full access: %w", err)
	}
	return nil
}

// --- MCP echo server ---------------------------------------------------------

func runMCPEcho() error {
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 64*1024), 16<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()
	for scanner.Scan() {
		var req struct {
			ID     json.RawMessage `json:"id"`
			Method string          `json:"method"`
			Params json.RawMessage `json:"params"`
		}
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil || len(req.ID) == 0 {
			continue // ignore notifications and malformed lines
		}
		var result any
		switch req.Method {
		case "initialize":
			result = map[string]any{
				"protocolVersion": "2024-11-05",
				"capabilities":    map[string]any{},
				"serverInfo":      map[string]string{"name": "acp-probe-echo", "version": "0.0.1"},
			}
		case "tools/list":
			result = map[string]any{"tools": []any{map[string]any{
				"name":        "echo",
				"description": "echo the text argument",
				"inputSchema": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"text": map[string]any{"type": "string"},
					},
					"required": []string{"text"},
				},
			}}}
		case "tools/call":
			var params struct {
				Arguments map[string]any `json:"arguments"`
			}
			_ = json.Unmarshal(req.Params, &params)
			text, _ := params.Arguments["text"].(string)
			result = map[string]any{
				"content": []any{map[string]any{"type": "text", "text": text}},
				"isError": false,
			}
		default:
			body, _ := json.Marshal(map[string]any{
				"jsonrpc": "2.0", "id": req.ID,
				"error": map[string]any{"code": -32601, "message": "method not found: " + req.Method},
			})
			fmt.Fprintln(out, string(body))
			_ = out.Flush()
			continue
		}
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": req.ID, "result": result})
		fmt.Fprintln(out, string(body))
		_ = out.Flush()
	}
	return scanner.Err()
}
