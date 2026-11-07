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
	for _, input := range []string{"deepseek-flash-search", " DeepSeek-Flash-Search "} {
		got, ok := ResolveModel(nil, input)
		thinking, search, valid := GetModelConfig(got)
		modelType, typeOK := GetModelType(got)
		if !ok || got != "deepseek-flash-search" || !valid || !thinking || !search || !typeOK || modelType != "default" {
			t.Fatalf("unexpected search model configuration: %q %v %v %v %v %q %v", got, ok, thinking, search, valid, modelType, typeOK)
		}
	}
	list := OpenAIModelsResponse()["data"].([]ModelInfo)
	if len(list) != 2 || list[0].ID != "deepseek-flash" || list[1].ID != "deepseek-flash-search" {
		t.Fatalf("unexpected models: %+v", list)
	}
	for _, model := range list {
		if !model.Multimodal || model.ContextLength != EffectiveUserFacingContextLimitTokens {
			t.Fatalf("unexpected model capabilities: %+v", model)
		}
	}
}
