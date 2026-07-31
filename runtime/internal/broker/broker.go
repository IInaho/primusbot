package broker

import "nekocode/protocol"

func cloneMap(input map[string]any) map[string]any {
	if input == nil {
		return nil
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = cloneValue(value)
	}
	return output
}

func cloneValue(value any) any {
	switch value := value.(type) {
	case map[string]any:
		return cloneMap(value)
	case []any:
		output := make([]any, len(value))
		for i := range value {
			output[i] = cloneValue(value[i])
		}
		return output
	case []string:
		return append([]string(nil), value...)
	default:
		return value
	}
}

func cloneQuestions(input []protocol.QuestionItem) []protocol.QuestionItem {
	output := append([]protocol.QuestionItem(nil), input...)
	for i := range output {
		output[i].Options = append([]protocol.QuestionOption(nil), input[i].Options...)
	}
	return output
}
