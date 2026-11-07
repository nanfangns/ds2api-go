package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchModeReachesUpstreamAcrossOpenAISurfaces(t *testing.T) {
	tests := []struct {
		name, path, body string
		wantSearch       bool
	}{
		{"chat default", "/v1/chat/completions", `{"model":"deepseek-flash","messages":[{"role":"user","content":"hello"}]}`, false},
		{"chat search model streaming", "/v1/chat/completions", `{"model":"deepseek-flash-search","messages":[{"role":"user","content":"hello"}],"stream":true}`, true},
		{"chat explicit enable", "/v1/chat/completions", `{"model":"deepseek-flash","messages":[{"role":"user","content":"hello"}],"search_enabled":true}`, true},
		{"chat explicit disable", "/v1/chat/completions", `{"model":"deepseek-flash-search","messages":[{"role":"user","content":"hello"}],"search_enabled":false}`, false},
		{"responses default", "/v1/responses", `{"model":"deepseek-flash","input":"hello"}`, false},
		{"responses search model streaming", "/v1/responses", `{"model":"deepseek-flash-search","input":"hello","stream":true}`, true},
		{"responses extra body enable", "/v1/responses", `{"model":"deepseek-flash","input":"hello","extra_body":{"search_enabled":true}}`, true},
		{"responses override priority", "/v1/responses", `{"model":"deepseek-flash-search","input":"hello","search_enabled":false,"extra_body":{"search_enabled":true}}`, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ds := &inlineUploadDSStub{}
			h := &openAITestSurface{Store: mockOpenAIConfig{}, Auth: streamStatusAuthStub{}, DS: ds}
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer direct-token")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			newOpenAITestRouter(h).ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "ok") {
				t.Fatalf("completion failed: status=%d body=%s", rec.Code, rec.Body.String())
			}
			if got := ds.completionReq["search_enabled"]; got != tc.wantSearch {
				t.Fatalf("search_enabled = %#v, want %v", got, tc.wantSearch)
			}
			if got := ds.completionReq["model_type"]; got != "default" {
				t.Fatalf("model_type = %#v, want default", got)
			}
		})
	}
}

func TestSearchResponseKeepsTextAndNonStreamingCitationLinks(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
		wantLink         bool
	}{
		{"chat", "/v1/chat/completions", `{"model":"deepseek-flash-search","messages":[{"role":"user","content":"hello"}]}`, true},
		{"chat stream", "/v1/chat/completions", `{"model":"deepseek-flash-search","messages":[{"role":"user","content":"hello"}],"stream":true}`, false},
		{"responses", "/v1/responses", `{"model":"deepseek-flash-search","input":"hello"}`, true},
		{"responses stream", "/v1/responses", `{"model":"deepseek-flash-search","input":"hello","stream":true}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ds := &inlineUploadDSStub{completionResp: makeOpenAISSEHTTPResponse(
				`data: {"p":"response/fragments/-1/results","v":[{"url":"https://example.com/source","cite_index":1}]}`,
				`data: {"p":"response/fragments/-1","o":"BATCH","v":[{"p":"status","v":"FINISHED"}]}`,
				`data: {"p":"response/content","v":"[citation:1] Answer text"}`,
				`data: [DONE]`,
			)}
			h := &openAITestSurface{Store: mockOpenAIConfig{}, Auth: streamStatusAuthStub{}, DS: ds}
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer direct-token")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			newOpenAITestRouter(h).ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Answer text") {
				t.Fatalf("search answer missing: status=%d body=%s", rec.Code, rec.Body.String())
			}
			if tc.wantLink && !strings.Contains(rec.Body.String(), "https://example.com/source") {
				t.Fatalf("citation link missing: %s", rec.Body.String())
			}
		})
	}
}
