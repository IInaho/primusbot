package runtime

import (
	"nekocode/runtime/internal/connectors"
	"nekocode/runtime/internal/redaction"
)

type Connector = connectors.Connector
type ConnectorStatusViewer = connectors.ConnectorStatusViewer
type ConnectorFactory = connectors.ConnectorFactory

func RedactInputText(input string) string {
	return redaction.RedactInputText(input)
}
