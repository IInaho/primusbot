package main

import (
	"context"
	"fmt"
	"strings"
)

type connectorConnect func(context.Context, string, []string) (string, error)

// connectorBootstrapStatus reports how one connector was started.
type connectorBootstrapStatus struct {
	name    string
	message string
}

// connectorBootstrap starts one connector from its environment.
type connectorBootstrap func(context.Context, connectorConnect, func(string) string) (string, error)

// connectorBootstraps maps every known connector to its bootstrap.
var connectorBootstraps = map[string]connectorBootstrap{
	"feishu":   bootstrapFeishu,
	"telegram": bootstrapTelegram,
	"qqbot":    bootstrapQQBot,
	"wecom":    bootstrapWeCom,
}

// bootstrapConnectors makes the daemon self-contained. NEKOCODE_CONNECTORS
// selects one or more transports; each transport can bootstrap credentials
// from its own environment variables or start from persisted connect.json.
func bootstrapConnectors(ctx context.Context, connect connectorConnect, getenv func(string) string) ([]connectorBootstrapStatus, error) {
	names, err := selectedConnectors(getenv)
	if err != nil {
		return nil, err
	}
	statuses := make([]connectorBootstrapStatus, 0, len(names))
	for _, name := range names {
		boot := connectorBootstraps[name]
		if boot == nil {
			return nil, fmt.Errorf("unknown connector %q", name)
		}
		message, err := boot(ctx, connect, getenv)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, connectorBootstrapStatus{name: name, message: message})
	}
	return statuses, nil
}

func selectedConnectors(getenv func(string) string) ([]string, error) {
	raw := strings.TrimSpace(getenv("NEKOCODE_CONNECTORS"))
	if raw == "" {
		var inferred []string
		if strings.TrimSpace(getenv("NEKOCODE_FEISHU_APP_ID")) != "" || strings.TrimSpace(getenv("NEKOCODE_FEISHU_APP_SECRET")) != "" {
			inferred = append(inferred, "feishu")
		}
		if strings.TrimSpace(getenv("NEKOCODE_TELEGRAM_BOT_TOKEN")) != "" {
			inferred = append(inferred, "telegram")
		}
		if strings.TrimSpace(getenv("NEKOCODE_QQBOT_APP_ID")) != "" || strings.TrimSpace(getenv("NEKOCODE_QQBOT_APP_SECRET")) != "" {
			inferred = append(inferred, "qqbot")
		}
		if strings.TrimSpace(getenv("NEKOCODE_WECOM_BOT_ID")) != "" || strings.TrimSpace(getenv("NEKOCODE_WECOM_BOT_SECRET")) != "" {
			inferred = append(inferred, "wecom")
		}
		return inferred, nil
	}
	if strings.EqualFold(raw, "none") || strings.EqualFold(raw, "off") {
		return nil, nil
	}
	seen := make(map[string]bool)
	var names []string
	for _, item := range strings.Split(raw, ",") {
		name := strings.ToLower(strings.TrimSpace(item))
		if connectorBootstraps[name] == nil {
			return nil, fmt.Errorf("unknown connector %q; available: feishu, telegram, qqbot, wecom", name)
		}
		if !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	return names, nil
}

func bootstrapFeishu(ctx context.Context, connect connectorConnect, getenv func(string) string) (string, error) {
	return bootstrapCredentialPair(ctx, connect, "feishu",
		"NEKOCODE_FEISHU_APP_ID", getenv("NEKOCODE_FEISHU_APP_ID"),
		"NEKOCODE_FEISHU_APP_SECRET", getenv("NEKOCODE_FEISHU_APP_SECRET"), nil, true)
}

func bootstrapTelegram(ctx context.Context, connect connectorConnect, getenv func(string) string) (string, error) {
	token := strings.TrimSpace(getenv("NEKOCODE_TELEGRAM_BOT_TOKEN"))
	if token != "" {
		if _, err := connect(ctx, "telegram", []string{"add", token}); err != nil {
			return "", fmt.Errorf("telegram: save bot token: %w", err)
		}
	}
	return connect(ctx, "telegram", nil)
}

func bootstrapQQBot(ctx context.Context, connect connectorConnect, getenv func(string) string) (string, error) {
	if err := configureQQBotSandbox(ctx, connect, getenv); err != nil {
		return "", err
	}
	return bootstrapCredentialPair(ctx, connect, "qqbot",
		"NEKOCODE_QQBOT_APP_ID", getenv("NEKOCODE_QQBOT_APP_ID"),
		"NEKOCODE_QQBOT_APP_SECRET", getenv("NEKOCODE_QQBOT_APP_SECRET"),
		[]string{"start"}, false)
}

func bootstrapWeCom(ctx context.Context, connect connectorConnect, getenv func(string) string) (string, error) {
	return bootstrapCredentialPair(ctx, connect, "wecom",
		"NEKOCODE_WECOM_BOT_ID", getenv("NEKOCODE_WECOM_BOT_ID"),
		"NEKOCODE_WECOM_BOT_SECRET", getenv("NEKOCODE_WECOM_BOT_SECRET"), nil, true)
}

// bootstrapCredentialPair validates a credential pair, saves it when present,
// and starts the connector with startArgs. QQBot's "add" already starts the
// connection, so its startAfterAdd is false and "add"'s own message is kept.
func bootstrapCredentialPair(ctx context.Context, connect connectorConnect, name, idEnv, id, secretEnv, secret string, startArgs []string, startAfterAdd bool) (string, error) {
	id, secret, err := credentialPair(id, secret, idEnv, secretEnv)
	if err != nil {
		return "", err
	}
	if id != "" {
		message, err := connect(ctx, name, []string{"add", id, secret})
		if err != nil {
			return "", fmt.Errorf("%s: save credentials: %w", name, err)
		}
		if !startAfterAdd {
			return message, nil
		}
	}
	message, err := connect(ctx, name, startArgs)
	if err != nil {
		return "", fmt.Errorf("%s: start connector: %w", name, err)
	}
	return message, nil
}

// credentialPair trims and validates that a credential pair is complete.
func credentialPair(id, secret, idEnv, secretEnv string) (string, string, error) {
	id = strings.TrimSpace(id)
	secret = strings.TrimSpace(secret)
	if (id == "") != (secret == "") {
		return "", "", fmt.Errorf("%s and %s must be set together", idEnv, secretEnv)
	}
	return id, secret, nil
}

func configureQQBotSandbox(ctx context.Context, connect connectorConnect, getenv func(string) string) error {
	sandbox := strings.ToLower(strings.TrimSpace(getenv("NEKOCODE_QQBOT_SANDBOX")))
	if sandbox == "" {
		return nil
	}
	switch sandbox {
	case "1", "true", "yes", "on":
		sandbox = "on"
	case "0", "false", "no", "off":
		sandbox = "off"
	default:
		return fmt.Errorf("NEKOCODE_QQBOT_SANDBOX must be true or false")
	}
	if _, err := connect(ctx, "qqbot", []string{"sandbox", sandbox}); err != nil {
		return fmt.Errorf("qqbot: configure sandbox: %w", err)
	}
	return nil
}
