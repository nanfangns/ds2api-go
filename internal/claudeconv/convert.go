package claudeconv

import "strings"

type ClaudeMappingProvider interface {
	ClaudeMapping() map[string]string
}

func ConvertClaudeToDeepSeek(claudeReq map[string]any, mappingProvider ClaudeMappingProvider, defaultClaudeModel string) map[string]any {
	messages, _ := claudeReq["messages"].([]any)
	model, _ := claudeReq["model"].(string)
	if model == "" {
		model = defaultClaudeModel
	}

	mapping := map[string]string{}
	if mappingProvider != nil {
		mapping = mappingProvider.ClaudeMapping()
	}
	dsModel := mapping["fast"]
	if dsModel == "" {
		dsModel = "deepseek-chat"
	}

	modelLower := strings.ToLower(model)
	if strings.Contains(modelLower, "opus") || strings.Contains(modelLower, "reasoner") || strings.Contains(modelLower, "slow") {
		if slow := mapping["slow"]; slow != "" {
			dsModel = slow
		}
	}

	convertedMessages := make([]any, 0, len(messages)+1)
	if system, ok := claudeReq["system"].(string); ok && system != "" {
		convertedMessages = append(convertedMessages, map[string]any{"role": "system", "content": system})
	}
	convertedMessages = append(convertedMessages, messages...)

	out := map[string]any{"model": dsModel, "messages": convertedMessages}
	for _, k := range []string{"temperature", "top_p", "stream"} {
		if v, ok := claudeReq[k]; ok {
			out[k] = v
		}
	}
	if stopSeq, ok := claudeReq["stop_sequences"]; ok {
		out["stop"] = stopSeq
	}
	return out
}
