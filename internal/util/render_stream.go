package util

// BuildOpenAIChatStreamDeltaChoice is kept for backward compatibility.
// Prefer internal/format/openai.BuildChatStreamDeltaChoice for new code.
func BuildOpenAIChatStreamDeltaChoice(index int, delta map[string]any) map[string]any {
	return map[string]any{
		"delta": delta,
		"index": index,
	}
}

// BuildOpenAIChatStreamFinishChoice is kept for backward compatibility.
// Prefer internal/format/openai.BuildChatStreamFinishChoice for new code.
func BuildOpenAIChatStreamFinishChoice(index int, finishReason string) map[string]any {
	return map[string]any{
		"delta":         map[string]any{},
		"index":         index,
		"finish_reason": finishReason,
	}
}

// BuildOpenAIChatStreamChunk is kept for backward compatibility.
// Prefer internal/format/openai.BuildChatStreamChunk for new code.
func BuildOpenAIChatStreamChunk(completionID string, created int64, model string, choices []map[string]any, usage map[string]any) map[string]any {
	out := map[string]any{
		"id":      completionID,
		"object":  "chat.completion.chunk",
		"created": created,
		"model":   model,
		"choices": choices,
	}
	if len(usage) > 0 {
		out["usage"] = usage
	}
	return out
}

// BuildOpenAIChatUsage is kept for backward compatibility.
// Prefer internal/format/openai.BuildChatUsage for new code.
func BuildOpenAIChatUsage(finalPrompt, finalThinking, finalText string) map[string]any {
	promptTokens := EstimateTokens(finalPrompt)
	reasoningTokens := EstimateTokens(finalThinking)
	completionTokens := EstimateTokens(finalText)
	return map[string]any{
		"prompt_tokens":     promptTokens,
		"completion_tokens": reasoningTokens + completionTokens,
		"total_tokens":      promptTokens + reasoningTokens + completionTokens,
		"completion_tokens_details": map[string]any{
			"reasoning_tokens": reasoningTokens,
		},
	}
}

// BuildOpenAIResponsesCreatedPayload is kept for backward compatibility.
// Prefer internal/format/openai.BuildResponsesCreatedPayload for new code.
func BuildOpenAIResponsesCreatedPayload(responseID, model string) map[string]any {
	return map[string]any{
		"type":   "response.created",
		"id":     responseID,
		"object": "response",
		"model":  model,
		"status": "in_progress",
	}
}

// BuildOpenAIResponsesTextDeltaPayload is kept for backward compatibility.
// Prefer internal/format/openai.BuildResponsesTextDeltaPayload for new code.
func BuildOpenAIResponsesTextDeltaPayload(responseID, delta string) map[string]any {
	return map[string]any{
		"type":  "response.output_text.delta",
		"id":    responseID,
		"delta": delta,
	}
}

// BuildOpenAIResponsesReasoningDeltaPayload is kept for backward compatibility.
// Prefer internal/format/openai.BuildResponsesReasoningDeltaPayload for new code.
func BuildOpenAIResponsesReasoningDeltaPayload(responseID, delta string) map[string]any {
	return map[string]any{
		"type":  "response.reasoning.delta",
		"id":    responseID,
		"delta": delta,
	}
}

// BuildOpenAIResponsesToolCallDeltaPayload is kept for backward compatibility.
// Prefer internal/format/openai.BuildResponsesToolCallDeltaPayload for new code.
func BuildOpenAIResponsesToolCallDeltaPayload(responseID string, toolCalls []map[string]any) map[string]any {
	return map[string]any{
		"type":       "response.output_tool_call.delta",
		"id":         responseID,
		"tool_calls": toolCalls,
	}
}

// BuildOpenAIResponsesToolCallDonePayload is kept for backward compatibility.
// Prefer internal/format/openai.BuildResponsesToolCallDonePayload for new code.
func BuildOpenAIResponsesToolCallDonePayload(responseID string, toolCalls []map[string]any) map[string]any {
	return map[string]any{
		"type":       "response.output_tool_call.done",
		"id":         responseID,
		"tool_calls": toolCalls,
	}
}

// BuildOpenAIResponsesCompletedPayload is kept for backward compatibility.
// Prefer internal/format/openai.BuildResponsesCompletedPayload for new code.
func BuildOpenAIResponsesCompletedPayload(response map[string]any) map[string]any {
	return map[string]any{
		"type":     "response.completed",
		"response": response,
	}
}
