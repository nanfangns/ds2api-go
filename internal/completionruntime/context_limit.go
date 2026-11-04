package completionruntime

import (
	"fmt"
	"net/http"
	"strings"

	"ds2api/internal/assistantturn"
	"ds2api/internal/config"
	"ds2api/internal/httpapi/openai/history"
	"ds2api/internal/promptcompat"
	"ds2api/internal/util"
)

func promptTextForContextGate(stdReq promptcompat.StandardRequest) string {
	prompt := strings.TrimSpace(stdReq.PromptTokenText)
	if prompt == "" {
		return stdReq.FinalPrompt
	}
	return prompt
}

func estimatedUserInputTokens(stdReq promptcompat.StandardRequest) int {
	model := strings.TrimSpace(stdReq.ResolvedModel)
	if model == "" {
		model = strings.TrimSpace(stdReq.RequestedModel)
	}
	return util.CountPromptTokens(promptTextForContextGate(stdReq), model) + stdReq.RefFileTokens
}

func userFacingTokenEstimate(internalEstimated int) int {
	if internalEstimated <= 0 {
		return 0
	}
	logical := config.EffectiveLogicalContextLimitTokens
	if logical <= 0 {
		return 0
	}
	return int((int64(internalEstimated)*int64(config.EffectiveUserFacingContextLimitTokens) + int64(logical)/2) / int64(logical))
}

func requestedCompletionTokens(stdReq promptcompat.StandardRequest) int {
	for _, key := range []string{"max_completion_tokens", "max_tokens"} {
		value, ok := stdReq.PassThrough[key]
		if !ok {
			continue
		}
		switch n := value.(type) {
		case float64:
			if n > 0 {
				return int(n)
			}
		case int:
			if n > 0 {
				return n
			}
		case int64:
			if n > 0 {
				return int(n)
			}
		}
	}
	return 0
}

func contextLengthExceededMessage(stdReq promptcompat.StandardRequest, internalEstimated int) string {
	messages := userFacingTokenEstimate(internalEstimated)
	completion := requestedCompletionTokens(stdReq)
	return fmt.Sprintf(
		"This model's maximum context length is %d tokens. However, you requested %d tokens (%d in the messages, %d in the completion). Please reduce the length of the messages or completion.",
		config.EffectiveUserFacingContextLimitTokens,
		messages+completion,
		messages,
		completion,
	)
}

// ContextGateError estimates the context that will actually be sent upstream.
// When current-input-file mode is about to upload the transcript, it estimates
// that replacement payload instead of charging the same history twice.
func ContextGateError(stdReq promptcompat.StandardRequest, currentInputConfig history.CurrentInputConfigReader) *assistantturn.OutputError {
	estimated := estimatedUserInputTokens(stdReq)
	if !stdReq.CurrentInputFileApplied {
		if uploadedTokens, willUpload := history.EstimateCurrentInputFileTokens(currentInputConfig, stdReq); willUpload {
			estimated = uploadedTokens + stdReq.RefFileTokens
		}
	}
	if estimated <= config.EffectiveLogicalContextLimitTokens {
		return nil
	}
	return &assistantturn.OutputError{
		Status:  http.StatusBadRequest,
		Message: contextLengthExceededMessage(stdReq, estimated),
		Code:    "invalid_request_error",
	}
}
