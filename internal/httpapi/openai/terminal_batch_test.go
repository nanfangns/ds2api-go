package openai

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAISurfacesReturnAnswerFromTerminalBatch(t *testing.T) {
	for _, tc := range []struct {
		name, path, body string
	}{
		{"chat stream", "/v1/chat/completions", `{"model":"deepseek-flash","messages":[{"role":"user","content":"hi"}],"stream":true}`},
		{"chat nonstream", "/v1/chat/completions", `{"model":"deepseek-flash","messages":[{"role":"user","content":"hi"}],"stream":false}`},
		{"responses stream", "/v1/responses", `{"model":"deepseek-flash","input":"hi","stream":true}`},
		{"responses nonstream", "/v1/responses", `{"model":"deepseek-flash","input":"hi","stream":false}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &openAITestSurface{
				Store: mockOpenAIConfig{},
				Auth:  streamStatusAuthStub{},
				DS: streamStatusDSStub{resp: makeOpenAISSEHTTPResponse(
					`data: {"p":"response/thinking_content","v":"think"}`,
					`data: {"p":"response","o":"BATCH","v":[{"p":"fragments","o":"APPEND","v":[{"type":"RESPONSE","content":"answer"}]},{"p":"status","v":"FINISHED"}]}`,
					`data: [DONE]`,
				)},
			}
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer direct-token")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			newOpenAITestRouter(h).ServeHTTP(rec, req)
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "answer") || strings.Contains(rec.Body.String(), "upstream_empty_output") {
				t.Fatalf("answer missing: status=%d body=%s", rec.Code, rec.Body.String())
			}
		})
	}
}
