package runtime

import (
	"nekocode/runtime/internal/connectors"
	"nekocode/runtime/internal/redaction"
)

type Connector = connectors.Connector
type ConnectorFactory = connectors.ConnectorFactory

func RedactInputText(input string) string {
	return redaction.RedactInputText(input)
}
