package history

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"ds2api/internal/account"
	"ds2api/internal/auth"
	"ds2api/internal/config"
	dsclient "ds2api/internal/deepseek/client"
	"ds2api/internal/promptcompat"
)

type privateCaptureUploader struct {
	requests  []dsclient.UploadFileRequest
	accounts  []string
	muteFirst bool
}

func (u *privateCaptureUploader) UploadFile(_ context.Context, a *auth.RequestAuth, req dsclient.UploadFileRequest, _ int) (*dsclient.UploadFileResult, error) {
	u.requests = append(u.requests, req)
	u.accounts = append(u.accounts, a.AccountID)
	if u.muteFirst && len(u.requests) == 1 {
		a.RecordAccountFailure([]byte(`{"data":{"biz_code":14,"biz_msg":"user is muted"}}`))
		return nil, &dsclient.RequestFailure{Kind: dsclient.FailureAccountRestricted, Message: "account is muted"}
	}
	return &dsclient.UploadFileResult{ID: "file-private", AccountID: a.AccountID}, nil
}

func TestPrivateContextSendsNeutralHistoryAndRetriesRestrictedAccount(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["key"],"accounts":[{"email":"first@example.test","token":"first"},{"email":"second@example.test","token":"second"}]}`)
	store := config.LoadStore()
	resolver := auth.NewResolver(store, account.NewPool(store), func(_ context.Context, acc config.Account) (string, error) { return "token-" + acc.Identifier(), nil })
	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer key")
	a, err := resolver.Determine(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Release(a)
	uploader := &privateCaptureUploader{muteFirst: true}
	standard := promptcompat.StandardRequest{
		ResolvedModel: "deepseek-flash",
		Messages: []any{
			map[string]any{"role": "system", "content": "Keep this instruction."},
			map[string]any{"role": "user", "content": "Find a useful answer."},
			map[string]any{"role": "assistant", "tool_calls": []any{map[string]any{"function": map[string]any{"name": "search", "arguments": `{"query":"docs"}`}}}},
			map[string]any{"role": "tool", "name": "search", "tool_call_id": "call-1", "content": "Found docs."},
			map[string]any{"role": "user", "content": "Summarize it."},
		},
		ToolsRaw:   []any{map[string]any{"type": "function", "function": map[string]any{"name": "search", "description": "Search docs", "parameters": map[string]any{"type": "object"}}}},
		ToolChoice: promptcompat.DefaultToolChoicePolicy(),
	}
	got, err := (Service{Store: store, DS: uploader}).ApplyCurrentInputFile(context.Background(), a, standard)
	if err != nil {
		t.Fatal(err)
	}
	if len(uploader.requests) != 2 || uploader.accounts[0] == uploader.accounts[1] {
		t.Fatalf("expected one restricted and one healthy account: %v", uploader.accounts)
	}
	first, _ := store.FindAccount("first@example.test")
	if first.PoolStatus != "muted" {
		t.Fatalf("restriction not recorded: %q", first.PoolStatus)
	}
	if len(got.RefFileIDs) != 1 || got.RefFileIDs[0] != "file-private" {
		t.Fatalf("unexpected references: %v", got.RefFileIDs)
	}
	if !strings.Contains(got.FinalPrompt, "Keep this instruction.") || !strings.Contains(got.FinalPrompt, "Search docs") {
		t.Fatalf("live prompt lost instructions or tools: %q", got.FinalPrompt)
	}
	if !strings.Contains(got.HistoryText, "Found docs.") || !strings.Contains(got.HistoryText, "::invoke name=\"search\"::") {
		t.Fatalf("history lost tool context: %q", got.HistoryText)
	}
	if strings.Contains(got.HistoryText, "Keep this instruction.") {
		t.Fatal("system instructions should remain in the live prompt")
	}
	for _, upload := range uploader.requests {
		if len(upload.Filename) != 28 || !strings.HasSuffix(upload.Filename, ".txt") {
			t.Fatalf("unexpected private filename %q", upload.Filename)
		}
		for _, old := range []string{"DS2API", "deepseek_to_api", "<|DSML|", "<|System|>"} {
			if strings.Contains(upload.Filename, old) || strings.Contains(string(upload.Data), old) || strings.Contains(got.FinalPrompt, old) {
				t.Fatalf("legacy marker %q in generated upstream content", old)
			}
		}
	}
}

func TestPrivateContextDoesNotSwitchAccountsForExistingFiles(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["key"],"accounts":[{"email":"first@example.test","token":"first"},{"email":"second@example.test","token":"second"}]}`)
	store := config.LoadStore()
	resolver := auth.NewResolver(store, account.NewPool(store), func(_ context.Context, acc config.Account) (string, error) { return "token-" + acc.Identifier(), nil })
	req, _ := http.NewRequest(http.MethodPost, "/", nil)
	req.Header.Set("Authorization", "Bearer key")
	a, err := resolver.Determine(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resolver.Release(a)
	uploader := &privateCaptureUploader{muteFirst: true}
	standard := promptcompat.StandardRequest{ResolvedModel: "deepseek-flash", Messages: []any{map[string]any{"role": "user", "content": "Describe this attached image."}}, RefFileIDs: []string{"existing-file"}}
	_, err = (Service{Store: store, DS: uploader}).ApplyCurrentInputFile(context.Background(), a, standard)
	if !dsclient.IsAccountRestrictedError(err) || len(uploader.accounts) != 1 {
		t.Fatalf("expected restriction without account switch, accounts=%v err=%v", uploader.accounts, err)
	}
}
