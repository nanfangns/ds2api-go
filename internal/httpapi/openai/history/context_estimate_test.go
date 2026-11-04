package history

import (
	"context"
	"testing"

	"ds2api/internal/auth"
	dsclient "ds2api/internal/deepseek/client"
	"ds2api/internal/promptcompat"
	"ds2api/internal/util"
)

type estimateConfig struct{}

func (estimateConfig) CurrentInputFileEnabled() bool { return true }
func (estimateConfig) CurrentInputFileMinChars() int { return 0 }

type estimateUploader struct{}

func (estimateUploader) UploadFile(context.Context, *auth.RequestAuth, dsclient.UploadFileRequest, int) (*dsclient.UploadFileResult, error) {
	return &dsclient.UploadFileResult{ID: "file-test"}, nil
}

func TestEstimateMatchesUploadedTranscriptAndContinuation(t *testing.T) {
	for _, tools := range []any{nil, []any{map[string]any{"type": "function", "function": map[string]any{"name": "read_file", "parameters": map[string]any{"type": "object"}}}}} {
		req := promptcompat.StandardRequest{
			ResolvedModel: "deepseek-flash",
			Messages:      []any{map[string]any{"role": "user", "content": "hello"}, map[string]any{"role": "assistant", "content": "reply"}, map[string]any{"role": "user", "content": "next"}},
			ToolsRaw:      tools, ToolChoice: promptcompat.DefaultToolChoicePolicy(), FinalPrompt: "original prompt should not be counted twice",
		}
		estimate, ok := EstimateCurrentInputFileTokens(estimateConfig{}, req)
		if !ok {
			t.Fatal("expected an upload estimate")
		}
		actual, err := (Service{Store: estimateConfig{}, DS: estimateUploader{}}).ApplyCurrentInputFile(context.Background(), &auth.RequestAuth{}, req)
		if err != nil {
			t.Fatal(err)
		}
		want := util.CountPromptTokens(actual.PromptTokenText, req.ResolvedModel)
		if estimate != want {
			t.Fatalf("estimate=%d actual=%d", estimate, want)
		}
	}
}
