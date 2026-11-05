package config

import "testing"

func TestWhaleModelScope(t *testing.T) {
	for _, input := range []string{"deepseek-flash", " DeepSeek-Flash "} {
		got, ok := ResolveModel(nil, input)
		if !ok || got != "deepseek-flash" {
			t.Fatalf("ResolveModel(%q) = %q, %v", input, got, ok)
		}
		thinking, search, valid := GetModelConfig(got)
		if !valid || !thinking || search {
			t.Fatalf("unexpected defaults: %v %v %v", thinking, search, valid)
		}
		modelType, valid := GetModelType(got)
		if !valid || modelType != "default" {
			t.Fatalf("unexpected type: %q %v", modelType, valid)
		}
	}
	for _, input := range []string{"deepseek-v4-flash", "deepseek-v4-pro", "deepseek-v4-vision", "deepseek-v4-flash-search", "deepseek-flash-nothinking", "gpt-4o", "claude-opus-4-6"} {
		if _, ok := ResolveModel(nil, input); ok {
			t.Fatalf("retired model %q unexpectedly accepted", input)
		}
	}
	list := OpenAIModelsResponse()["data"].([]ModelInfo)
	if len(list) != 1 || list[0].ID != "deepseek-flash" || !list[0].Multimodal || list[0].ContextLength != EffectiveUserFacingContextLimitTokens {
		t.Fatalf("unexpected models: %+v", list)
	}
}
