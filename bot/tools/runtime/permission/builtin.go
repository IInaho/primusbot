package permission

// Builtin rules: the default policy baked into NekoCode. User/project rules
// are evaluated alongside these (deny from any source wins; user/project can
// add ask/allow to override the builtin defaults for unmatched calls).
//
// Bash rules use command-prefix matching (claude-code style): a specifier
// like "npm run *" matches commands starting with "npm run ". The bash
// matcher handles the wildcard logic; here we only declare the policies.
//
// Bash uses explicit authorization: builtins block obviously unsafe commands,
// force prompts for destructive ones, and allow common read-only inspection
// commands. Everything else falls to the engine caller's default ask until a
// user/project/remembered allow rule exists.

var builtinRules = []Rule{
	// --- Bash: hard-deny (irreversible / privilege escalation) ---
	{Tool: "bash", Specifier: "sudo *", Effect: EffectDeny, Source: "builtin"},
	{Tool: "bash", Specifier: "sudo", Effect: EffectDeny, Source: "builtin"},
	{Tool: "bash", Specifier: "eval *", Effect: EffectDeny, Source: "builtin"},
	{Tool: "bash", Specifier: "dd *", Effect: EffectDeny, Source: "builtin"},
	{Tool: "bash", Specifier: "mkfs*", Effect: EffectDeny, Source: "builtin"},
	{Tool: "bash", Specifier: "ssh *", Effect: EffectDeny, Source: "builtin"},
	{Tool: "bash", Specifier: "| bash", Effect: EffectDeny, Source: "builtin"},
	{Tool: "bash", Specifier: "| sh", Effect: EffectDeny, Source: "builtin"},

	// --- Bash: ask (destructive but reversible within workspace) ---
	{Tool: "bash", Specifier: "rm *", Effect: EffectAsk, Source: "builtin"},
	{Tool: "bash", Specifier: "rmdir *", Effect: EffectAsk, Source: "builtin"},
	{Tool: "bash", Specifier: "kill *", Effect: EffectAsk, Source: "builtin"},
	{Tool: "bash", Specifier: "pkill *", Effect: EffectAsk, Source: "builtin"},
	{Tool: "bash", Specifier: "chmod *", Effect: EffectAsk, Source: "builtin"},
	{Tool: "bash", Specifier: "chown *", Effect: EffectAsk, Source: "builtin"},
	{Tool: "bash", Specifier: "git push *", Effect: EffectAsk, Source: "builtin"},
	{Tool: "bash", Specifier: "git push", Effect: EffectAsk, Source: "builtin"},
	{Tool: "bash", Specifier: "git reset --hard *", Effect: EffectAsk, Source: "builtin"},
	{Tool: "bash", Specifier: "mv *", Effect: EffectAsk, Source: "builtin"},
	{Tool: "bash", Specifier: "shutdown *", Effect: EffectAsk, Source: "builtin"},
	{Tool: "bash", Specifier: "reboot *", Effect: EffectAsk, Source: "builtin"},

	// --- Bash: allow common read-only inspection ---
	{Tool: "bash", Specifier: "ls *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "pwd", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "whoami", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "date", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "printenv *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "which *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "uname *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "hostname", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "wc *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "cat *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "head *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "tail *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "less *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "more *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "du *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "df *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "free *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "uptime", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "pgrep *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "file *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "stat *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "go version", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "go env *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "go doc *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "go vet *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "go fmt *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "git status *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "git log *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "git diff *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "git show *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "git blame *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "git tag *", Effect: EffectAllow, Source: "builtin"},
	{Tool: "bash", Specifier: "git remote *", Effect: EffectAllow, Source: "builtin"},

	// --- File tools: workspace writes default to allow (path boundary enforces scope) ---
	{Tool: "write", Effect: EffectAllow, Source: "builtin"},
	{Tool: "edit", Effect: EffectAllow, Source: "builtin"},
	{Tool: "read", Effect: EffectAllow, Source: "builtin"},
	{Tool: "list", Effect: EffectAllow, Source: "builtin"},
	{Tool: "tree", Effect: EffectAllow, Source: "builtin"},
	{Tool: "glob", Effect: EffectAllow, Source: "builtin"},
	{Tool: "grep", Effect: EffectAllow, Source: "builtin"},
	{Tool: "diff", Effect: EffectAllow, Source: "builtin"},

	// --- Other tools: default allow (stateless / non-destructive) ---
	{Tool: "todo_write", Effect: EffectAllow, Source: "builtin"},
	{Tool: "task", Effect: EffectAllow, Source: "builtin"},
	{Tool: "web_search", Effect: EffectAllow, Source: "builtin"},
	{Tool: "web_fetch", Effect: EffectAllow, Source: "builtin"},
	{Tool: "question", Effect: EffectAllow, Source: "builtin"},
}

// BuiltinRules returns a copy of the builtin rule set.
func BuiltinRules() []Rule {
	out := make([]Rule, len(builtinRules))
	copy(out, builtinRules)
	return out
}
