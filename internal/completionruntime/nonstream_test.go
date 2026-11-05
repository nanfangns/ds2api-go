package completionruntime

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/account"
	"ds2api/internal/assistantturn"
	"ds2api/internal/auth"
	"ds2api/internal/config"
	dsclient "ds2api/internal/deepseek/client"
	"ds2api/internal/promptcompat"
)

type fakeDeepSeekCaller struct {
	responses          []*http.Response
	payloads           []map[string]any
	uploads            []dsclient.UploadFileRequest
	completionAccounts []string
	sessionByAccount   bool
}

type currentInputRuntimeConfig struct{}

func (currentInputRuntimeConfig) CurrentInputFileEnabled() bool { return true }
func (currentInputRuntimeConfig) CurrentInputFileMinChars() int { return 0 }

func (f *fakeDeepSeekCaller) CreateSession(_ context.Context, a *auth.RequestAuth, _ int) (string, error) {
	if f.sessionByAccount && a != nil && a.AccountID != "" {
		return "session-" + a.AccountID, nil
	}
	return "session-1", nil
}

func (f *fakeDeepSeekCaller) GetPow(context.Context, *auth.RequestAuth, int) (string, error) {
	return "pow", nil
}

func (f *fakeDeepSeekCaller) UploadFile(_ context.Context, a *auth.RequestAuth, req dsclient.UploadFileRequest, _ int) (*dsclient.UploadFileResult, error) {
	f.uploads = append(f.uploads, req)
	if a != nil && a.AccountID != "" {
		return &dsclient.UploadFileResult{ID: "file-runtime-" + a.AccountID}, nil
	}
	return &dsclient.UploadFileResult{ID: "file-runtime-1"}, nil
}

func (f *fakeDeepSeekCaller) CallCompletion(_ context.Context, a *auth.RequestAuth, payload map[string]any, _ string, _ int) (*http.Response, error) {
	f.payloads = append(f.payloads, payload)
	if a != nil {
		f.completionAccounts = append(f.completionAccounts, a.AccountID)
	}
	if len(f.responses) == 0 {
		return sseHTTPResponse(http.StatusOK, `data: {"p":"response/content","v":"fallback"}`), nil
	}
	resp := f.responses[0]
	f.responses = f.responses[1:]
	return resp, nil
}

func TestExecuteNonStreamWithRetryBuildsCanonicalTurn(t *testing.T) {
	ds := &fakeDeepSeekCaller{responses: []*http.Response{sseHTTPResponse(
		http.StatusOK,
		`data: {"response_message_id":42,"p":"response/content","v":"<tool_calls><invoke name=\"Write\"><parameter name=\"content\">{\"x\":1}</parameter></invoke></tool_calls>"}`,
	)}}
	stdReq := promptcompat.StandardRequest{
		Surface:         "test",
		ResponseModel:   "deepseek-flash",
		PromptTokenText: "prompt",
		FinalPrompt:     "final prompt",
		ToolNames:       []string{"Write"},
		ToolsRaw: []any{map[string]any{
			"name": "Write",
			"input_schema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"content": map[string]any{"type": "string"},
				},
			},
		}},
	}

	result, outErr := ExecuteNonStreamWithRetry(context.Background(), ds, &auth.RequestAuth{}, stdReq, Options{})
	if outErr != nil {
		t.Fatalf("unexpected output error: %#v", outErr)
	}
	if result.SessionID != "session-1" {
		t.Fatalf("session mismatch: %q", result.SessionID)
	}
	if got := result.Turn.ResponseMessageID; got != 42 {
		t.Fatalf("response message id mismatch: %d", got)
	}
	if len(result.Turn.ToolCalls) != 1 {
		t.Fatalf("expected one tool call, got %d", len(result.Turn.ToolCalls))
	}
	if _, ok := result.Turn.ToolCalls[0].Input["content"].(string); !ok {
		t.Fatalf("expected schema-normalized string argument, got %#v", result.Turn.ToolCalls[0].Input["content"])
	}
	if result.Turn.Usage.InputTokens == 0 || result.Turn.Usage.TotalTokens == 0 {
		t.Fatalf("expected usage to be populated, got %#v", result.Turn.Usage)
	}
}

func TestExecuteNonStreamWithRetrySwitchesManagedAccountBeforeFinal429(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{
		"keys":["managed-key"],
		"accounts":[
			{"email":"acc1@test.com","password":"pwd"},
			{"email":"acc2@test.com","password":"pwd"}
		]
	}`)
	store := config.LoadStore()
	resolver := auth.NewResolver(store, account.NewPool(store), func(_ context.Context, acc config.Account) (string, error) {
		return "token-" + acc.Identifier(), nil
	})
	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer managed-key")
	a, err := resolver.Determine(req)
	if err != nil {
		t.Fatalf("determine failed: %v", err)
	}
	defer resolver.Release(a)

	ds := &fakeDeepSeekCaller{
		sessionByAccount: true,
		responses: []*http.Response{
			sseHTTPResponse(http.StatusOK, `data: {"response_message_id":11,"p":"response/thinking_content","v":"first empty"}`),
			sseHTTPResponse(http.StatusOK, `data: {"response_message_id":12,"p":"response/thinking_content","v":"retry empty"}`),
			sseHTTPResponse(http.StatusOK, `data: {"response_message_id":21,"p":"response/content","v":"ok from second account"}`),
		},
	}
	stdReq := promptcompat.StandardRequest{
		Surface:         "test",
		ResponseModel:   "deepseek-flash",
		PromptTokenText: "prompt",
		FinalPrompt:     "final prompt",
		Thinking:        true,
	}

	result, outErr := ExecuteNonStreamWithRetry(context.Background(), ds, a, stdReq, Options{RetryEnabled: true})
	if outErr != nil {
		t.Fatalf("unexpected output error after account switch retry: %#v", outErr)
	}
	if result.Turn.Text != "ok from second account" {
		t.Fatalf("text mismatch after switch retry: %q", result.Turn.Text)
	}
	if result.SessionID != "session-acc2@test.com" {
		t.Fatalf("expected switched account session, got %q", result.SessionID)
	}
	wantAccounts := []string{"acc1@test.com", "acc1@test.com", "acc2@test.com"}
	if len(ds.completionAccounts) != len(wantAccounts) {
		t.Fatalf("completion account count mismatch: got %v want %v", ds.completionAccounts, wantAccounts)
	}
	for i, want := range wantAccounts {
		if ds.completionAccounts[i] != want {
			t.Fatalf("completion account %d = %q want %q (all=%v)", i, ds.completionAccounts[i], want, ds.completionAccounts)
		}
	}
	if got := ds.payloads[2]["chat_session_id"]; got != "session-acc2@test.com" {
		t.Fatalf("switched payload session mismatch: %#v", got)
	}
	if prompt, _ := ds.payloads[2]["prompt"].(string); strings.Contains(prompt, "Previous reply had no visible output") {
		t.Fatalf("expected fresh switched-account prompt without empty-output suffix, got %q", prompt)
	}
}

func TestExecuteNonStreamWithRetryReuploadsCurrentInputFileAfterAccountSwitch(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{
		"keys":["managed-key"],
		"accounts":[
			{"email":"acc1@test.com","password":"pwd"},
			{"email":"acc2@test.com","password":"pwd"}
		]
	}`)
	store := config.LoadStore()
	resolver := auth.NewResolver(store, account.NewPool(store), func(_ context.Context, acc config.Account) (string, error) {
		return "token-" + acc.Identifier(), nil
	})
	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer managed-key")
	a, err := resolver.Determine(req)
	if err != nil {
		t.Fatalf("determine failed: %v", err)
	}
	defer resolver.Release(a)

	ds := &fakeDeepSeekCaller{
		sessionByAccount: true,
		responses: []*http.Response{
			sseHTTPResponse(http.StatusOK, `data: {"response_message_id":11,"p":"response/thinking_content","v":"first empty"}`),
			sseHTTPResponse(http.StatusOK, `data: {"response_message_id":12,"p":"response/thinking_content","v":"retry empty"}`),
			sseHTTPResponse(http.StatusOK, `data: {"response_message_id":21,"p":"response/content","v":"ok from second account"}`),
		},
	}
	stdReq := promptcompat.StandardRequest{
		Surface:        "test",
		RequestedModel: "deepseek-flash",
		ResolvedModel:  "deepseek-flash",
		ResponseModel:  "deepseek-flash",
		Messages: []any{
			map[string]any{"role": "user", "content": "large current input"},
		},
		PromptTokenText: "large current input",
		FinalPrompt:     "large current input",
		Thinking:        true,
	}

	result, outErr := ExecuteNonStreamWithRetry(context.Background(), ds, a, stdReq, Options{
		RetryEnabled:     true,
		CurrentInputFile: currentInputRuntimeConfig{},
	})
	if outErr != nil {
		t.Fatalf("unexpected output error after account switch retry: %#v", outErr)
	}
	if result.Turn.Text != "ok from second account" {
		t.Fatalf("text mismatch after switch retry: %q", result.Turn.Text)
	}
	if len(ds.uploads) != 2 {
		t.Fatalf("expected current input file uploaded once per account, got %d", len(ds.uploads))
	}
	refIDs, _ := ds.payloads[2]["ref_file_ids"].([]any)
	if len(refIDs) != 1 || refIDs[0] != "file-runtime-acc2@test.com" {
		t.Fatalf("expected switched account ref_file_ids to use reuploaded file, got %#v", ds.payloads[2]["ref_file_ids"])
	}
}

func TestExecuteNonStreamWithRetryUsesParentMessageForEmptyRetry(t *testing.T) {
	ds := &fakeDeepSeekCaller{responses: []*http.Response{
		sseHTTPResponse(http.StatusOK, `data: {"response_message_id":77,"p":"response/thinking_content","v":"plan"}`),
		sseHTTPResponse(http.StatusOK, `data: {"response_message_id":78,"p":"response/content","v":"ok"}`),
	}}
	stdReq := promptcompat.StandardRequest{
		Surface:         "test",
		ResponseModel:   "deepseek-flash",
		PromptTokenText: "prompt",
		FinalPrompt:     "final prompt",
	}

	result, outErr := ExecuteNonStreamWithRetry(context.Background(), ds, &auth.RequestAuth{}, stdReq, Options{RetryEnabled: true})
	if outErr != nil {
		t.Fatalf("unexpected output error: %#v", outErr)
	}
	if result.Attempts != 1 {
		t.Fatalf("expected one retry, got %d", result.Attempts)
	}
	if len(ds.payloads) != 2 {
		t.Fatalf("expected two completion calls, got %d", len(ds.payloads))
	}
	if got := ds.payloads[1]["parent_message_id"]; got != 77 {
		t.Fatalf("retry parent_message_id mismatch: %#v", got)
	}
	if result.Turn.Text != "ok" {
		t.Fatalf("retry text mismatch: %q", result.Turn.Text)
	}
}

func TestExecuteNonStreamWithRetryConvertsReferenceMarkers(t *testing.T) {
	ds := &fakeDeepSeekCaller{responses: []*http.Response{sseHTTPResponse(
		http.StatusOK,
		`data: {"p":"response/content","v":"答案[reference:0]。","citation":{"cite_index":0,"url":"https://example.com/ref"}}`,
	)}}
	stdReq := promptcompat.StandardRequest{
		Surface:         "test",
		ResponseModel:   "deepseek-flash-search",
		PromptTokenText: "prompt",
		FinalPrompt:     "final prompt",
		Search:          true,
	}

	result, outErr := ExecuteNonStreamWithRetry(context.Background(), ds, &auth.RequestAuth{}, stdReq, Options{})
	if outErr != nil {
		t.Fatalf("unexpected output error: %#v", outErr)
	}
	want := "答案[0](https://example.com/ref)。"
	if result.Turn.Text != want {
		t.Fatalf("text mismatch: got %q want %q", result.Turn.Text, want)
	}
}

func TestStartCompletionAppliesCurrentInputFileGlobally(t *testing.T) {
	ds := &fakeDeepSeekCaller{responses: []*http.Response{sseHTTPResponse(http.StatusOK, `data: {"p":"response/content","v":"ok"}`)}}
	stdReq := promptcompat.StandardRequest{
		Surface:         "test_adapter",
		RequestedModel:  "deepseek-flash",
		ResolvedModel:   "deepseek-flash",
		ResponseModel:   "deepseek-flash",
		PromptTokenText: "first user turn",
		FinalPrompt:     "first user turn",
		Messages: []any{
			map[string]any{"role": "user", "content": "first user turn"},
		},
	}

	start, outErr := StartCompletion(context.Background(), ds, &auth.RequestAuth{DeepSeekToken: "token"}, stdReq, Options{
		CurrentInputFile: currentInputRuntimeConfig{},
	})
	if outErr != nil {
		t.Fatalf("unexpected output error: %#v", outErr)
	}
	if len(ds.uploads) != 1 {
		t.Fatalf("expected current input upload, got %d", len(ds.uploads))
	}
	if got := ds.uploads[0].Filename; !strings.HasSuffix(got, ".txt") || len(got) != 28 {
		t.Fatalf("upload filename=%q want randomized filename", got)
	}
	if len(ds.payloads) != 1 {
		t.Fatalf("expected one completion payload, got %d", len(ds.payloads))
	}
	refIDs, _ := ds.payloads[0]["ref_file_ids"].([]any)
	if len(refIDs) != 1 || refIDs[0] != "file-runtime-1" {
		t.Fatalf("expected uploaded file id in ref_file_ids, got %#v", ds.payloads[0]["ref_file_ids"])
	}
	prompt, _ := ds.payloads[0]["prompt"].(string)
	if !strings.Contains(prompt, "Using the prior conversation already provided, respond directly to the user's latest request.") {
		t.Fatalf("expected continuation prompt, got %q", prompt)
	}
	if !start.Request.CurrentInputFileApplied || !strings.Contains(start.Request.PromptTokenText, "first user turn") {
		t.Fatalf("expected prepared request to carry current input file state, got %#v", start.Request)
	}
}

func sseHTTPResponse(status int, lines ...string) *http.Response {
	body := strings.Join(lines, "\n")
	if !strings.HasSuffix(body, "\n") {
		body += "\n"
	}
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func TestStartCompletionQuarantinesMutedAccountAndRetries(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["managed-key"],"accounts":[{"email":"first@test.com","password":"pwd"},{"email":"second@test.com","password":"pwd"}]}`)
	store := config.LoadStore()
	resolver := auth.NewResolver(store, account.NewPool(store), func(_ context.Context, acc config.Account) (string, error) { return "token-" + acc.Identifier(), nil })
	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer managed-key")
	a, err := resolver.Determine(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Release(a)
	muted := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"data":{"biz_code":5,"biz_msg":"user is muted","biz_data":{"is_muted":1}}}`))}
	ds := &fakeDeepSeekCaller{sessionByAccount: true, responses: []*http.Response{muted, sseHTTPResponse(http.StatusOK, `data: {"p":"response/content","v":"ok"}`)}}
	result, outErr := StartCompletion(context.Background(), ds, a, promptcompat.StandardRequest{ResponseModel: "deepseek-flash"}, Options{})
	if outErr != nil {
		t.Fatalf("unexpected error: %#v", outErr)
	}
	defer func() {
		if err := result.Response.Body.Close(); err != nil {
			t.Errorf("close response: %v", err)
		}
	}()
	if a.AccountID != "second@test.com" || len(ds.completionAccounts) != 2 {
		t.Fatalf("did not switch accounts: %v", ds.completionAccounts)
	}
	first, _ := store.FindAccount("first@test.com")
	if first.PoolStatus != "muted" || first.MuteUntil == 0 {
		t.Fatalf("restriction not recorded: %+v", first)
	}
	if _, ok := resolver.Pool.Acquire("first@test.com", nil); ok {
		t.Fatal("muted account was not quarantined")
	}
}

func TestStartCompletionDoesNotSwitchWithExternalFile(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["managed-key"],"accounts":[{"email":"first@test.com","password":"pwd"},{"email":"second@test.com","password":"pwd"}]}`)
	store := config.LoadStore()
	resolver := auth.NewResolver(store, account.NewPool(store), func(_ context.Context, acc config.Account) (string, error) { return "token-" + acc.Identifier(), nil })
	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer managed-key")
	a, err := resolver.Determine(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Release(a)
	muted := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"data":{"biz_code":14,"biz_msg":"user is muted"}}`))}
	ds := &fakeDeepSeekCaller{responses: []*http.Response{muted}}
	result, outErr := StartCompletion(context.Background(), ds, a, promptcompat.StandardRequest{RefFileIDs: []string{"external-file"}}, Options{})
	if outErr == nil || outErr.Status != http.StatusServiceUnavailable || result.Response != nil {
		t.Fatalf("expected a clear restriction error, got result=%#v err=%#v", result, outErr)
	}
	if a.AccountID != "first@test.com" || len(ds.completionAccounts) != 1 {
		t.Fatalf("must not reuse an external file across accounts: account=%s calls=%v", a.AccountID, ds.completionAccounts)
	}
}

func TestRetryDoesNotSwitchWithExternalFile(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["managed-key"],"accounts":[{"email":"first@test.com","password":"pwd"},{"email":"second@test.com","password":"pwd"}]}`)
	store := config.LoadStore()
	resolver := auth.NewResolver(store, account.NewPool(store), func(_ context.Context, acc config.Account) (string, error) { return "token-" + acc.Identifier(), nil })
	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer managed-key")
	a, err := resolver.Determine(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Release(a)
	attempted := false
	stdReq := promptcompat.StandardRequest{CurrentInputFileID: "generated", RefFileIDs: []string{"generated", "external"}}
	if canRetryOnAlternateAccount(context.Background(), a, stdReq, true, &assistantturn.OutputError{Status: http.StatusTooManyRequests}, true, &attempted) {
		t.Fatal("retry incorrectly switched accounts with an external file")
	}
	if attempted || a.AccountID != "first@test.com" {
		t.Fatalf("account changed while external file was attached: %s", a.AccountID)
	}
}

func TestDirectTokenRestrictionReturnsClearError(t *testing.T) {
	muted := &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"data":{"biz_code":14,"biz_msg":"user is muted"}}`))}
	ds := &fakeDeepSeekCaller{responses: []*http.Response{muted}}
	result, outErr := StartCompletion(context.Background(), ds, &auth.RequestAuth{DeepSeekToken: "direct"}, promptcompat.StandardRequest{}, Options{})
	if outErr == nil || outErr.Status != http.StatusServiceUnavailable || result.Response != nil || len(ds.payloads) != 1 {
		t.Fatalf("expected one clear restriction error: result=%#v err=%#v calls=%d", result, outErr, len(ds.payloads))
	}
}
