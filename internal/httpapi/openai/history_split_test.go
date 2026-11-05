package openai

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/auth"
	dsclient "ds2api/internal/deepseek/client"
	"ds2api/internal/promptcompat"
)

func historySplitTestMessages() []any {
	toolCalls := []any{
		map[string]any{
			"name":      "search",
			"arguments": map[string]any{"query": "docs"},
		},
	}
	return []any{
		map[string]any{"role": "system", "content": "system instructions"},
		map[string]any{"role": "user", "content": "first user turn"},
		map[string]any{
			"role":              "assistant",
			"content":           "",
			"reasoning_content": "hidden reasoning",
			"tool_calls":        toolCalls,
		},
		map[string]any{
			"role":         "tool",
			"name":         "search",
			"tool_call_id": "call-1",
			"content":      "tool result",
		},
		map[string]any{"role": "user", "content": "latest user turn"},
	}
}

type streamStatusManagedAuthStub struct{}

func (streamStatusManagedAuthStub) Determine(_ *http.Request) (*auth.RequestAuth, error) {
	return &auth.RequestAuth{
		UseConfigToken: true,
		DeepSeekToken:  "managed-token",
		CallerID:       "caller:test",
		AccountID:      "acct:test",
		TriedAccounts:  map[string]bool{},
	}, nil
}

func (streamStatusManagedAuthStub) DetermineCaller(_ *http.Request) (*auth.RequestAuth, error) {
	return (&streamStatusManagedAuthStub{}).Determine(nil)
}

func (streamStatusManagedAuthStub) Release(_ *auth.RequestAuth) {}

func TestBuildPrivateContextTranscriptPreservesConversation(t *testing.T) {
	transcript := buildOpenAICurrentInputContextTranscript(historySplitTestMessages())
	for _, want := range []string{"System:", "User:", "Assistant:", "Tool:", "first user turn", "tool result", "latest user turn", "hidden reasoning", "::tc::"} {
		if !strings.Contains(transcript, want) {
			t.Fatalf("missing %q in %q", want, transcript)
		}
	}
	for _, old := range []string{"DS2API_HISTORY", "<|DSML|", "<|System|>"} {
		if strings.Contains(transcript, old) {
			t.Fatalf("legacy marker %q in %q", old, transcript)
		}
	}
}

func TestApplyCurrentInputFileSkipsShortInputWhenThresholdNotReached(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{
		Store: mockOpenAIConfig{
			currentInputEnabled: true,
			currentInputMin:     20,
		},
		DS: ds,
	}
	req := map[string]any{
		"model": "deepseek-flash",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}
	stdReq, err := promptcompat.NormalizeOpenAIChatRequest(h.Store, req, "")
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}

	out, err := h.applyCurrentInputFile(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, stdReq)
	if err != nil {
		t.Fatalf("apply current input file failed: %v", err)
	}
	if len(ds.uploadCalls) != 0 {
		t.Fatalf("expected no upload on first turn, got %d", len(ds.uploadCalls))
	}
	if out.FinalPrompt != stdReq.FinalPrompt {
		t.Fatalf("expected prompt unchanged on first turn")
	}
}

func TestApplyThinkingInjectionAppendsLatestUserPrompt(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{
		Store: mockOpenAIConfig{
			thinkingInjection: boolPtr(true),
		},
		DS: ds,
	}
	req := map[string]any{
		"model": "deepseek-flash",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}
	stdReq, err := promptcompat.NormalizeOpenAIChatRequest(h.Store, req, "")
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}

	out, err := h.applyCurrentInputFile(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, stdReq)
	if err != nil {
		t.Fatalf("apply thinking injection failed: %v", err)
	}
	if len(ds.uploadCalls) != 0 {
		t.Fatalf("expected no upload for first short turn, got %d", len(ds.uploadCalls))
	}
	if !strings.Contains(out.FinalPrompt, promptcompat.ThinkingInjectionMarker) {
		t.Fatalf("expected thinking injection after latest user message, got %s", out.FinalPrompt)
	}
}

func TestApplyThinkingInjectionUsesCustomPrompt(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{
		Store: mockOpenAIConfig{
			thinkingInjection: boolPtr(true),
			thinkingPrompt:    "custom thinking format",
		},
		DS: ds,
	}
	req := map[string]any{
		"model": "deepseek-flash",
		"messages": []any{
			map[string]any{"role": "user", "content": "hello"},
		},
	}
	stdReq, err := promptcompat.NormalizeOpenAIChatRequest(h.Store, req, "")
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}

	out, err := h.applyCurrentInputFile(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, stdReq)
	if err != nil {
		t.Fatalf("apply thinking injection failed: %v", err)
	}
	if !strings.Contains(out.FinalPrompt, "custom thinking format") {
		t.Fatalf("expected custom thinking injection after latest user message, got %s", out.FinalPrompt)
	}
}

func TestApplyCurrentInputFileDisabledPassThrough(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{
		Store: mockOpenAIConfig{
			currentInputEnabled: false,
		},
		DS: ds,
	}
	req := map[string]any{
		"model":    "deepseek-flash",
		"messages": historySplitTestMessages(),
	}
	stdReq, err := promptcompat.NormalizeOpenAIChatRequest(h.Store, req, "")
	if err != nil {
		t.Fatalf("normalize failed: %v", err)
	}

	out, err := h.applyCurrentInputFile(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, stdReq)
	if err != nil {
		t.Fatalf("apply current input file failed: %v", err)
	}
	if len(ds.uploadCalls) != 0 {
		t.Fatalf("expected no uploads when both split modes are disabled, got %d", len(ds.uploadCalls))
	}
	if out.CurrentInputFileApplied || out.HistoryText != "" {
		t.Fatalf("expected direct pass-through, got current_input=%v history=%q", out.CurrentInputFileApplied, out.HistoryText)
	}
	if !strings.Contains(out.FinalPrompt, "first user turn") || !strings.Contains(out.FinalPrompt, "latest user turn") {
		t.Fatalf("expected original prompt context to stay inline, got %s", out.FinalPrompt)
	}
}

func TestApplyPrivateContextFirstTurn(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{Store: mockOpenAIConfig{currentInputEnabled: true, currentInputMin: 10, thinkingInjection: boolPtr(true)}, DS: ds}
	req := map[string]any{"model": "deepseek-flash", "messages": []any{map[string]any{"role": "user", "content": "first turn content that is long enough"}}}
	standard, err := promptcompat.NormalizeOpenAIChatRequest(h.Store, req, "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := h.applyCurrentInputFile(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, standard)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.uploadCalls) != 1 || len(ds.uploadCalls[0].Filename) != 28 || !strings.HasSuffix(ds.uploadCalls[0].Filename, ".txt") {
		t.Fatalf("unexpected upload: %+v", ds.uploadCalls)
	}
	body := string(ds.uploadCalls[0].Data)
	if !strings.Contains(body, "User:\nfirst turn content") || strings.Contains(body, "DS2API") {
		t.Fatalf("bad private context: %q", body)
	}
	if !strings.Contains(out.FinalPrompt, promptcompat.ThinkingInjectionMarker) || !strings.Contains(out.FinalPrompt, promptcompat.PrivateContextLivePrompt) {
		t.Fatalf("missing live instruction: %q", out.FinalPrompt)
	}
	if strings.Contains(out.FinalPrompt, "first turn content") || !strings.Contains(out.PromptTokenText, "first turn content") {
		t.Fatalf("bad context accounting: %+v", out)
	}
}

func TestPrivateContextAccountsForSystemAndHistory(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{Store: mockOpenAIConfig{currentInputEnabled: true}, DS: ds}
	standard, err := promptcompat.NormalizeOpenAIChatRequest(h.Store, map[string]any{"model": "deepseek-flash", "messages": historySplitTestMessages()}, "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := h.applyCurrentInputFile(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, standard)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"system instructions", "first user turn", "latest user turn", promptcompat.PrivateContextLivePrompt} {
		if !strings.Contains(out.PromptTokenText, want) {
			t.Fatalf("missing %q in %q", want, out.PromptTokenText)
		}
	}
	if strings.Contains(out.FinalPrompt, "latest user turn") || !strings.Contains(out.FinalPrompt, "system instructions") {
		t.Fatalf("wrong live prompt %q", out.FinalPrompt)
	}
}

func TestPrivateContextSeparatesSystemFromConversation(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{Store: mockOpenAIConfig{currentInputEnabled: true}, DS: ds}
	standard, err := promptcompat.NormalizeOpenAIChatRequest(h.Store, map[string]any{"model": "deepseek-flash", "messages": historySplitTestMessages()}, "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := h.applyCurrentInputFile(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, standard)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.uploadCalls) != 1 || !out.CurrentInputFileApplied {
		t.Fatalf("unexpected upload: %+v", ds.uploadCalls)
	}
	body := string(ds.uploadCalls[0].Data)
	if strings.Contains(body, "system instructions") || !strings.Contains(body, "latest user turn") || !strings.Contains(body, "tool result") {
		t.Fatalf("bad history: %q", body)
	}
	if !strings.Contains(out.FinalPrompt, "system instructions") {
		t.Fatalf("lost system instruction: %q", out.FinalPrompt)
	}
}

func TestPrivateContextKeepsToolSchemasInLivePrompt(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{Store: mockOpenAIConfig{currentInputEnabled: true}, DS: ds}
	tools := []any{map[string]any{"type": "function", "function": map[string]any{"name": "search", "description": "search docs", "parameters": map[string]any{"type": "object"}}}}
	standard, err := promptcompat.NormalizeOpenAIChatRequest(h.Store, map[string]any{"model": "deepseek-flash", "messages": historySplitTestMessages(), "tools": tools}, "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := h.applyCurrentInputFile(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, standard)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.uploadCalls) != 1 || len(out.RefFileIDs) != 1 {
		t.Fatalf("expected one history file: %+v %v", ds.uploadCalls, out.RefFileIDs)
	}
	if !strings.Contains(out.FinalPrompt, "Description: search docs") || !strings.Contains(out.FinalPrompt, "::tc::") {
		t.Fatalf("missing tool details: %q", out.FinalPrompt)
	}
	if strings.Contains(out.FinalPrompt, "DS2API_TOOLS") || strings.Contains(string(ds.uploadCalls[0].Data), "Description: search docs") {
		t.Fatal("unexpected tool file or history schema")
	}
}

func TestPrivateContextCarriesHistoryText(t *testing.T) {
	ds := &inlineUploadDSStub{}
	h := &openAITestSurface{Store: mockOpenAIConfig{currentInputEnabled: true}, DS: ds}
	standard, err := promptcompat.NormalizeOpenAIChatRequest(h.Store, map[string]any{"model": "deepseek-flash", "messages": historySplitTestMessages()}, "")
	if err != nil {
		t.Fatal(err)
	}
	out, err := h.applyCurrentInputFile(context.Background(), &auth.RequestAuth{DeepSeekToken: "token"}, standard)
	if err != nil {
		t.Fatal(err)
	}
	if len(ds.uploadCalls) != 1 || out.HistoryText != string(ds.uploadCalls[0].Data) || !strings.Contains(out.HistoryText, "latest user turn") {
		t.Fatalf("history lost: %q", out.HistoryText)
	}
}

func TestChatCompletionsCurrentInputFileMapsManagedAuthFailureTo401(t *testing.T) {
	ds := &inlineUploadDSStub{
		uploadErr: &dsclient.RequestFailure{Op: "upload file", Kind: dsclient.FailureManagedUnauthorized, Message: "expired token"},
	}
	h := &openAITestSurface{
		Store: mockOpenAIConfig{
			currentInputEnabled: true,
		},
		Auth: streamStatusManagedAuthStub{},
		DS:   ds,
	}
	reqBody, _ := json.Marshal(map[string]any{
		"model":    "deepseek-flash",
		"messages": historySplitTestMessages(),
		"stream":   false,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))
	req.Header.Set("Authorization", "Bearer managed-key")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ChatCompletions(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Please re-login the account in admin") {
		t.Fatalf("expected managed auth error message, got %s", rec.Body.String())
	}
}

func TestResponsesCurrentInputFileMapsDirectAuthFailureTo401(t *testing.T) {
	ds := &inlineUploadDSStub{
		uploadErr: &dsclient.RequestFailure{Op: "upload file", Kind: dsclient.FailureDirectUnauthorized, Message: "invalid token"},
	}
	h := &openAITestSurface{
		Store: mockOpenAIConfig{
			currentInputEnabled: true,
		},
		Auth: streamStatusAuthStub{},
		DS:   ds,
	}
	r := chi.NewRouter()
	registerOpenAITestRoutes(r, h)
	reqBody, _ := json.Marshal(map[string]any{
		"model":    "deepseek-flash",
		"messages": historySplitTestMessages(),
		"stream":   false,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(string(reqBody)))
	req.Header.Set("Authorization", "Bearer direct-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "Invalid token") {
		t.Fatalf("expected direct auth error message, got %s", rec.Body.String())
	}
}

func TestChatCompletionsCurrentInputFileUploadFailureReturnsInternalServerError(t *testing.T) {
	ds := &inlineUploadDSStub{uploadErr: errors.New("boom")}
	h := &openAITestSurface{
		Store: mockOpenAIConfig{
			currentInputEnabled: true,
		},
		Auth: streamStatusAuthStub{},
		DS:   ds,
	}
	reqBody, _ := json.Marshal(map[string]any{
		"model":    "deepseek-flash",
		"messages": historySplitTestMessages(),
		"stream":   false,
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))
	req.Header.Set("Authorization", "Bearer direct-token")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.ChatCompletions(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestCurrentInputFileWorksAcrossAutoDeleteModes(t *testing.T) {
	for _, mode := range []string{"none", "single", "all"} {
		t.Run(mode, func(t *testing.T) {
			ds := &inlineUploadDSStub{}
			h := &openAITestSurface{
				Store: mockOpenAIConfig{
					autoDeleteMode:      mode,
					currentInputEnabled: true,
				},
				Auth: streamStatusAuthStub{},
				DS:   ds,
			}
			reqBody, _ := json.Marshal(map[string]any{
				"model":    "deepseek-flash",
				"messages": historySplitTestMessages(),
				"stream":   false,
			})
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(string(reqBody)))
			req.Header.Set("Authorization", "Bearer direct-token")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			h.ChatCompletions(rec, req)

			if rec.Code != http.StatusOK {
				t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
			}
			if len(ds.uploadCalls) != 1 {
				t.Fatalf("expected current input upload for mode=%s, got %d", mode, len(ds.uploadCalls))
			}
			historyText := string(ds.uploadCalls[0].Data)
			if !strings.Contains(historyText, "User:") || !strings.Contains(historyText, "latest user turn") {
				t.Fatalf("expected uploaded history text to use numbered transcript format, got %s", historyText)
			}
			if ds.completionReq == nil {
				t.Fatalf("expected completion payload for mode=%s", mode)
			}
			promptText, _ := ds.completionReq["prompt"].(string)
			if !strings.Contains(promptText, "Using the prior conversation already provided, respond directly to the user's latest request.") || strings.Contains(promptText, "first user turn") || strings.Contains(promptText, "latest user turn") {
				t.Fatalf("unexpected prompt for mode=%s: %s", mode, promptText)
			}
		})
	}
}

func boolPtr(v bool) *bool {
	return &v
}

func TestPrivateContextFlowsThroughOpenAISurfaces(t *testing.T) {
	for _, route := range []string{"/v1/chat/completions", "/v1/responses"} {
		t.Run(route, func(t *testing.T) {
			ds := &inlineUploadDSStub{}
			h := &openAITestSurface{Store: mockOpenAIConfig{currentInputEnabled: true}, Auth: streamStatusAuthStub{}, DS: ds}
			r := chi.NewRouter()
			registerOpenAITestRoutes(r, h)
			body, err := json.Marshal(map[string]any{
				"model": "deepseek-flash", "messages": historySplitTestMessages(), "stream": false,
				"tools": []any{map[string]any{"type": "function", "function": map[string]any{"name": "search", "description": "search docs", "parameters": map[string]any{"type": "object"}}}},
			})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, route, strings.NewReader(string(body)))
			req.Header.Set("Authorization", "Bearer direct-token")
			rec := httptest.NewRecorder()
			r.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
			}
			if len(ds.uploadCalls) != 1 || len(ds.uploadCalls[0].Filename) != 28 || !strings.HasSuffix(ds.uploadCalls[0].Filename, ".txt") {
				t.Fatalf("unexpected uploads: %+v", ds.uploadCalls)
			}
			if ds.completionReq == nil {
				t.Fatal("missing completion payload")
			}
			promptText, _ := ds.completionReq["prompt"].(string)
			if !strings.Contains(promptText, promptcompat.PrivateContextLivePrompt) || !strings.Contains(promptText, "Description: search docs") || !strings.Contains(promptText, "::tc::") {
				t.Fatalf("bad prompt: %q", promptText)
			}
			for _, bad := range []string{"DS2API", "<|DSML|", "<|System|>"} {
				if strings.Contains(promptText, bad) || strings.Contains(string(ds.uploadCalls[0].Data), bad) {
					t.Fatalf("old marker %q leaked", bad)
				}
			}
			refs, _ := ds.completionReq["ref_file_ids"].([]any)
			if len(refs) != 1 || refs[0] != "file-inline-1" {
				t.Fatalf("unexpected refs: %v", refs)
			}
		})
	}
}
