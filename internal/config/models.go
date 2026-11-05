package config

import "strings"

type ModelInfo struct {
	ID            string `json:"id"`
	Object        string `json:"object"`
	Created       int64  `json:"created"`
	OwnedBy       string `json:"owned_by"`
	ContextLength int    `json:"context_length,omitempty"`
	Multimodal    bool   `json:"multimodal,omitempty"`
	Permission    []any  `json:"permission,omitempty"`
}

type ModelAliasReader interface {
	ModelAliases() map[string]string
}

var DeepSeekModels = []ModelInfo{
	{ID: "deepseek-flash", Object: "model", Created: 1677610602, OwnedBy: "deepseek", Multimodal: true, Permission: []any{}},
}

func GetModelConfig(model string) (thinking bool, search bool, ok bool) {
	if strings.EqualFold(strings.TrimSpace(model), "deepseek-flash") {
		return true, false, true
	}
	return false, false, false
}

func GetModelType(model string) (string, bool) {
	if strings.EqualFold(strings.TrimSpace(model), "deepseek-flash") {
		return "default", true
	}
	return "", false
}

func IsSupportedDeepSeekModel(model string) bool {
	_, _, ok := GetModelConfig(model)
	return ok
}

func IsNoThinkingModel(string) bool { return false }

func DefaultModelAliases() map[string]string { return map[string]string{} }

func ResolveModel(_ ModelAliasReader, requested string) (string, bool) {
	if strings.EqualFold(strings.TrimSpace(requested), "deepseek-flash") {
		return "deepseek-flash", true
	}
	return "", false
}

func lower(s string) string {
	b := []byte(s)
	for i, c := range b {
		if c >= 'A' && c <= 'Z' {
			b[i] = c + 32
		}
	}
	return string(b)
}

func OpenAIModelsResponse() map[string]any {
	models := append([]ModelInfo(nil), DeepSeekModels...)
	for i := range models {
		models[i].ContextLength = EffectiveUserFacingContextLimitTokens
	}
	return map[string]any{"object": "list", "data": models}
}

func OpenAIModelByID(store ModelAliasReader, id string) (ModelInfo, bool) {
	canonical, ok := ResolveModel(store, id)
	if !ok {
		return ModelInfo{}, false
	}
	for _, model := range DeepSeekModels {
		if model.ID == canonical {
			model.ContextLength = EffectiveUserFacingContextLimitTokens
			return model, true
		}
	}
	return ModelInfo{}, false
}
