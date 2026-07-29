package runtime

import (
	"context"

	internalconnectors "nekocode/runtime/internal/connectors"
)

type Connector = internalconnectors.Connector
type ConnectorFactory = internalconnectors.ConnectorFactory

func (r *Manager) RegisterConnector(name string, factory ConnectorFactory) {
	r.connectors.Register(name, factory)
}

func (r *Manager) Connect(ctx context.Context, name string, args []string) (string, error) {
	if err := r.ensureOpen(); err != nil {
		return "", err
	}
	return r.connectors.Handle(ctx, append([]string{name}, args...))
}

func (r *Manager) Disconnect(name string) (string, error) {
	if err := r.ensureOpen(); err != nil {
		return "", err
	}
	return r.connectors.Disconnect(name)
}

func (r *Manager) ConnectView() ConnectView {
	return r.connectors.View()
}
