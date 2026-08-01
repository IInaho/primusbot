package bot

import (
	"context"
	"errors"

	agentcore "nekocode/bot/agent"
	"nekocode/bot/config"
	"nekocode/bot/extension/plugin"
	"nekocode/bot/tools/runtime/permission"
	"nekocode/protocol"
)

func (b *Bot) Steer(ctx context.Context, msg string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	b.getAgent().Steer(msg)
	return nil
}

func (b *Bot) getAgent() *agentcore.Agent {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.ag
}

func (b *Bot) Run(ctx context.Context, input string, host RunHost) (string, error) {
	b.ensureSessionIdentity()
	release := b.bindHost(host)
	defer release()

	ag := b.getAgent()
	finished := make(chan struct{})
	watchDone := make(chan struct{})
	go func() {
		defer close(watchDone)
		select {
		case <-ctx.Done():
			ag.Abort()
		case <-finished:
		}
	}()
	defer func() {
		close(finished)
		<-watchDone
	}()
	output, err := b.runAgent(input, host.Step)
	return output, err
}

func (b *Bot) runAgent(input string, onStep func(ev protocol.StepEvent)) (string, error) {
	ag := b.getAgent()
	result := ag.Run(input, onStep)
	ag.Executor().SetPlanMode(false)
	b.ctxMgr.SetSystemPrompt(b.promptBuilder.BuildStatic())
	var compactionErr error
	if b.ctxMgr.NeedsSummarization() {
		compactionErr = b.ctxMgr.Summarize()
	}
	result.Error = errors.Join(
		result.Error,
		compactionErr,
		b.saveSession(),
	)
	return result.FinalOutput, result.Error
}

// bindHost exposes the current synchronous operation to callbacks registered
// on long-lived agents and tools.
func (b *Bot) bindHost(host RunHost) func() {
	b.hostMu.Lock()
	b.runHost = host
	b.hostMu.Unlock()
	return func() {
		b.hostMu.Lock()
		b.runHost = nil
		b.hostMu.Unlock()
	}
}

func (b *Bot) currentHost() RunHost {
	b.hostMu.RLock()
	defer b.hostMu.RUnlock()
	return b.runHost
}

func (b *Bot) stream(delta string) {
	if host := b.currentHost(); host != nil {
		host.Text(delta)
	}
}

func (b *Bot) reasoning(delta string) {
	if host := b.currentHost(); host != nil {
		host.Reason(delta)
	}
}

func (b *Bot) phase(phase string) {
	if host := b.currentHost(); host != nil {
		host.Phase(phase)
	}
}

func (b *Bot) todos(items []protocol.TodoItem) {
	if host := b.currentHost(); host != nil {
		host.Todos(items)
	}
}

func (b *Bot) confirm(request protocol.ConfirmRequest) protocol.ConfirmReply {
	if host := b.currentHost(); host != nil {
		return host.Confirm(request)
	}
	return protocol.ConfirmReply{Allowed: false}
}

func (b *Bot) ask(request protocol.QuestionRequest) protocol.QuestionReply {
	if host := b.currentHost(); host != nil {
		return host.Ask(request)
	}
	return protocol.QuestionReply{Rejected: true}
}

func (b *Bot) configureAgent(ag *agentcore.Agent) {
	if ag == nil {
		return
	}
	ag.Executor().SetProjectStore(b.cwd)
	ag.Executor().SetPermissionPolicy(toPermDecl(b.cfg.Permissions), b.cwd, b.home)
}

func (b *Bot) confirmInstall(source string, candidate *plugin.Plugin, remote bool) bool {
	reply := b.confirm(protocol.NewConfirmRequest(
		"/plugin install",
		map[string]any{"source": source, "summary": plugin.ConfirmSummary(candidate, remote)},
		protocol.ConfirmKindInstall,
	))
	return reply.Allowed
}

func toPermDecl(config *config.PermissionsConfig) permission.PermissionsDecl {
	if config == nil {
		return permission.PermissionsDecl{}
	}
	return permission.PermissionsDecl{
		Allow: config.Allow, Ask: config.Ask, Deny: config.Deny,
		Sandbox: toSandboxDecl(config.Sandbox),
	}
}

func toSandboxDecl(input map[string]config.SandboxConfig) map[string]permission.SandboxProfile {
	if len(input) == 0 {
		return nil
	}
	output := make(map[string]permission.SandboxProfile, len(input))
	for rule, profile := range input {
		output[rule] = permission.SandboxProfile{
			SandboxMode: profile.SandboxMode, Network: profile.Network,
			WritableRoots: append([]string(nil), profile.WritableRoots...),
		}
	}
	return output
}

func (b *Bot) CommandNames() []string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cmd.Names()
}

func (b *Bot) ExecuteCommand(ctx context.Context, input string, host RunHost) (protocol.CommandResult, error) {
	release := b.bindHost(host)
	defer release()

	resp, handled := b.cmd.Execute(ctx, input, b.ctxMgr)
	if !handled {
		return protocol.CommandResult{Action: protocol.CommandIgnored}, nil
	}

	if err := b.saveSession(); err != nil {
		return protocol.CommandResult{}, err
	}

	wantsAgent := b.cmd.TakeContinuation()
	result := protocol.CommandResult{
		Action: protocol.CommandHandled,
		Output: resp,
	}
	if wantsAgent {
		result.Action = protocol.CommandContinue
	}
	return result, nil
}
