package openai

import (
	"strings"
	"testing"

	"ds2api/internal/promptcompat"
)

type mockOpenAIConfig struct {
	aliases             map[string]string
	autoDeleteMode      string
	toolMode            string
	earlyEmit           string
	responsesTTL        int
	embedProv           string
	currentInputEnabled bool
	currentInputMin     int
	thinkingInjection   *bool
	thinkingPrompt      string
}

func (m mockOpenAIConfig) ModelAliases() map[string]string     { return m.aliases }
func (m mockOpenAIConfig) ToolcallMode() string                { return m.toolMode }
func (m mockOpenAIConfig) ToolcallEarlyEmitConfidence() string { return m.earlyEmit }
func (m mockOpenAIConfig) ResponsesStoreTTLSeconds() int       { return m.responsesTTL }
func (m mockOpenAIConfig) EmbeddingsProvider() string          { return m.embedProv }
func (m mockOpenAIConfig) AutoDeleteMode() string {
	if m.autoDeleteMode == "" {
		return "none"
	}
	return m.autoDeleteMode
}
func (m mockOpenAIConfig) AutoDeleteSessions() bool      { return false }
func (m mockOpenAIConfig) CurrentInputFileEnabled() bool { return m.currentInputEnabled }
func (m mockOpenAIConfig) CurrentInputFileMinChars() int {
	return m.currentInputMin
}
func (m mockOpenAIConfig) ThinkingInjectionEnabled() bool {
	if m.thinkingInjection == nil {
		return false
	}
	return *m.thinkingInjection
}
func (m mockOpenAIConfig) ThinkingInjectionPrompt() string { return m.thinkingPrompt }

func TestNormalizeOpenAIChatRequestRejectsAliases(t *testing.T) {
	cfg := mockOpenAIConfig{aliases: map[string]string{"my-model": "deepseek-flash"}}
	_, err := promptcompat.NormalizeOpenAIChatRequest(cfg, map[string]any{"model": "my-model", "messages": []any{map[string]any{"role": "user", "content": "hello"}}}, "")
	if err == nil {
		t.Fatal("unsupported alias accepted")
	}
}

func TestNormalizeOpenAIChatRequestDisablesThinkingWithParameter(t *testing.T) {
	out, err := promptcompat.NormalizeOpenAIChatRequest(mockOpenAIConfig{}, map[string]any{"model": "deepseek-flash", "messages": []any{map[string]any{"role": "user", "content": "hello"}}, "thinking": false}, "")
	if err != nil || out.Thinking || out.Search {
		t.Fatalf("unexpected request flags: %+v, %v", out, err)
	}
}

func TestNormalizeOpenAIResponsesRequestAlwaysAcceptsWideInput(t *testing.T) {
	req := map[string]any{
		"model": "deepseek-flash",
		"input": "hi",
	}

	out, err := promptcompat.NormalizeOpenAIResponsesRequest(mockOpenAIConfig{
		aliases: map[string]string{},
	}, req, "")
	if err != nil {
		t.Fatalf("unexpected error for wide input request: %v", err)
	}
	if out.Surface != "openai_responses" {
		t.Fatalf("unexpected surface: %q", out.Surface)
	}
	if !strings.Contains(out.FinalPrompt, "::user::hi") {
		t.Fatalf("unexpected final prompt: %q", out.FinalPrompt)
	}
}
