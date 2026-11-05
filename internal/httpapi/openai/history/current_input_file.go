package history

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"ds2api/internal/auth"
	"ds2api/internal/config"
	dsclient "ds2api/internal/deepseek/client"
	"ds2api/internal/httpapi/openai/shared"
	"ds2api/internal/privatecontext"
	"ds2api/internal/promptcompat"
	"ds2api/internal/util"
)

const (
	currentInputContentType = "text/plain; charset=utf-8"
	currentInputPurpose     = "assistants"
)

type CurrentInputConfigReader interface {
	CurrentInputFileEnabled() bool
	CurrentInputFileMinChars() int
}

type CurrentInputUploader interface {
	UploadFile(ctx context.Context, a *auth.RequestAuth, req dsclient.UploadFileRequest, maxAttempts int) (*dsclient.UploadFileResult, error)
}

type Service struct {
	Store CurrentInputConfigReader
	DS    CurrentInputUploader
}

// ApplyCurrentInputFile puts the conversation in one neutral private-context
// file and keeps system instructions and tool schemas in the live prompt.
func (s Service) ApplyCurrentInputFile(ctx context.Context, a *auth.RequestAuth, stdReq promptcompat.StandardRequest) (promptcompat.StandardRequest, error) {
	if stdReq.CurrentInputFileApplied || s.DS == nil || s.Store == nil || a == nil || !s.Store.CurrentInputFileEnabled() {
		return stdReq, nil
	}
	fileText, liveMessages, ok := preparePrivateContext(s.Store, stdReq)
	if !ok {
		return stdReq, nil
	}
	for {
		result, err := s.uploadPrivateContext(ctx, a, stdReq.ResolvedModel, fileText)
		if err != nil {
			// Existing caller-provided file IDs belong to the current account. Do
			// not silently switch them to a different account.
			if dsclient.IsAccountRestrictedError(err) && len(stdReq.RefFileIDs) == 0 && a.SwitchAccount(ctx) {
				continue
			}
			return stdReq, fmt.Errorf("upload private context: %w", err)
		}
		if result == nil {
			return stdReq, errors.New("upload private context returned no result")
		}
		fileID := strings.TrimSpace(result.ID)
		if fileID == "" {
			return stdReq, errors.New("upload private context returned empty file id")
		}
		stdReq.HistoryText = fileText
		stdReq.CurrentInputFileApplied = true
		stdReq.CurrentInputFileID = fileID
		stdReq.CurrentToolsFileID = ""
		stdReq.RefFileIDs = prependUniqueRefFileIDs(stdReq.RefFileIDs, fileID)
		stdReq.FinalPrompt, stdReq.ToolNames = promptcompat.BuildOpenAIPrompt(liveMessages, stdReq.ToolsRaw, "", stdReq.ToolChoice, stdReq.Thinking)
		stdReq.PromptTokenText = fileText + "\n" + stdReq.FinalPrompt
		return stdReq, nil
	}
}

func (s Service) uploadPrivateContext(ctx context.Context, a *auth.RequestAuth, modelID, text string) (*dsclient.UploadFileResult, error) {
	var random [12]byte
	if _, err := rand.Read(random[:]); err != nil {
		return nil, fmt.Errorf("generate private context filename: %w", err)
	}
	modelType := "default"
	if resolvedType, ok := config.GetModelType(modelID); ok {
		modelType = resolvedType
	}
	req := dsclient.UploadFileRequest{
		Filename:    hex.EncodeToString(random[:]) + ".txt",
		ContentType: currentInputContentType,
		Purpose:     currentInputPurpose,
		ModelType:   modelType,
		Data:        []byte(text),
	}
	return privatecontext.Upload(ctx, s.DS, a, req, 3, privatecontext.Key(a, modelType, text))
}

func (s Service) ReuploadAppliedCurrentInputFile(ctx context.Context, a *auth.RequestAuth, stdReq promptcompat.StandardRequest) (promptcompat.StandardRequest, error) {
	if !stdReq.CurrentInputFileApplied || s.DS == nil || a == nil || strings.TrimSpace(stdReq.HistoryText) == "" {
		return stdReq, nil
	}
	result, err := s.uploadPrivateContext(ctx, a, stdReq.ResolvedModel, stdReq.HistoryText)
	if err != nil {
		return stdReq, fmt.Errorf("upload private context: %w", err)
	}
	if result == nil {
		return stdReq, errors.New("upload private context returned no result")
	}
	fileID := strings.TrimSpace(result.ID)
	if fileID == "" {
		return stdReq, errors.New("upload private context returned empty file id")
	}
	stdReq.RefFileIDs = replaceGeneratedCurrentInputRefs(stdReq.RefFileIDs, stdReq.CurrentInputFileID, stdReq.CurrentToolsFileID, fileID)
	stdReq.CurrentInputFileID = fileID
	stdReq.CurrentToolsFileID = ""
	return stdReq, nil
}

func latestUserInputForFile(messages []any) (int, string) {
	for i := len(messages) - 1; i >= 0; i-- {
		msg, ok := messages[i].(map[string]any)
		if !ok || strings.ToLower(strings.TrimSpace(shared.AsString(msg["role"]))) != "user" {
			continue
		}
		text := promptcompat.NormalizeOpenAIContentForPrompt(msg["content"])
		if strings.TrimSpace(text) == "" {
			return -1, ""
		}
		return i, text
	}
	return -1, ""
}

func prependUniqueRefFileIDs(existing []string, fileIDs ...string) []string {
	out := make([]string, 0, len(existing)+len(fileIDs))
	seen := map[string]struct{}{}
	for _, id := range append(fileIDs, existing...) {
		trimmed := strings.TrimSpace(id)
		key := strings.ToLower(trimmed)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		out = append(out, trimmed)
		seen[key] = struct{}{}
	}
	return out
}

func replaceGeneratedCurrentInputRefs(existing []string, oldHistoryID, oldToolsID, newHistoryID string) []string {
	filtered := make([]string, 0, len(existing))
	for _, id := range existing {
		if strings.TrimSpace(id) == "" || strings.EqualFold(id, oldHistoryID) || strings.EqualFold(id, oldToolsID) {
			continue
		}
		filtered = append(filtered, id)
	}
	return prependUniqueRefFileIDs(filtered, newHistoryID)
}

func preparePrivateContext(store CurrentInputConfigReader, stdReq promptcompat.StandardRequest) (string, []any, bool) {
	if store == nil || !store.CurrentInputFileEnabled() || stdReq.CurrentInputFileApplied {
		return "", nil, false
	}
	if _, latest := latestUserInputForFile(stdReq.Messages); strings.TrimSpace(latest) == "" {
		return "", nil, false
	}
	_, history := promptcompat.SplitMessagesForPrivateContext(stdReq.Messages)
	text := promptcompat.BuildOpenAIPrivateContextTranscript(history)
	if strings.TrimSpace(text) == "" || len([]rune(text)) < store.CurrentInputFileMinChars() {
		return "", nil, false
	}
	return text, promptcompat.BuildPrivateContextLiveMessages(stdReq.Messages), true
}

// EstimateCurrentInputFileTokens computes the same private context and live
// prompt as ApplyCurrentInputFile before contacting the upstream service.
func EstimateCurrentInputFileTokens(store CurrentInputConfigReader, stdReq promptcompat.StandardRequest) (int, bool) {
	text, messages, ok := preparePrivateContext(store, stdReq)
	if !ok {
		return 0, false
	}
	prompt, _ := promptcompat.BuildOpenAIPrompt(messages, stdReq.ToolsRaw, "", stdReq.ToolChoice, stdReq.Thinking)
	return util.CountPromptTokens(text+"\n"+prompt, strings.TrimSpace(stdReq.ResolvedModel)), true
}
