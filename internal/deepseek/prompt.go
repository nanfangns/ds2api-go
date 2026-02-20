package deepseek

import "ds2api/internal/prompt"

func MessagesPrepare(messages []map[string]any) string {
	return prompt.MessagesPrepare(messages)
}
