package openai

import "strings"

func (h *Handler) toolcallFeatureMatchEnabled() bool {
	if h == nil || h.Store == nil {
		return true
	}
	mode := strings.TrimSpace(strings.ToLower(h.Store.ToolcallMode()))
	return mode == "" || mode == "feature_match"
}

func (h *Handler) toolcallEarlyEmitHighConfidence() bool {
	if h == nil || h.Store == nil {
		return true
	}
	level := strings.TrimSpace(strings.ToLower(h.Store.ToolcallEarlyEmitConfidence()))
	return level == "" || level == "high"
}
