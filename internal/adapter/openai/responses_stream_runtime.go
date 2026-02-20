package openai

import (
	"encoding/json"
	"net/http"
	"strings"

	openaifmt "ds2api/internal/format/openai"
	"ds2api/internal/sse"
	streamengine "ds2api/internal/stream"
	"ds2api/internal/util"
)

type responsesStreamRuntime struct {
	w        http.ResponseWriter
	rc       *http.ResponseController
	canFlush bool

	responseID  string
	model       string
	finalPrompt string
	toolNames   []string

	thinkingEnabled bool
	searchEnabled   bool

	bufferToolContent   bool
	emitEarlyToolDeltas bool
	toolCallsEmitted    bool

	sieve             toolStreamSieveState
	thinking          strings.Builder
	text              strings.Builder
	streamToolCallIDs map[int]string

	persistResponse func(obj map[string]any)
}

func newResponsesStreamRuntime(
	w http.ResponseWriter,
	rc *http.ResponseController,
	canFlush bool,
	responseID string,
	model string,
	finalPrompt string,
	thinkingEnabled bool,
	searchEnabled bool,
	toolNames []string,
	bufferToolContent bool,
	emitEarlyToolDeltas bool,
	persistResponse func(obj map[string]any),
) *responsesStreamRuntime {
	return &responsesStreamRuntime{
		w:                   w,
		rc:                  rc,
		canFlush:            canFlush,
		responseID:          responseID,
		model:               model,
		finalPrompt:         finalPrompt,
		thinkingEnabled:     thinkingEnabled,
		searchEnabled:       searchEnabled,
		toolNames:           toolNames,
		bufferToolContent:   bufferToolContent,
		emitEarlyToolDeltas: emitEarlyToolDeltas,
		streamToolCallIDs:   map[int]string{},
		persistResponse:     persistResponse,
	}
}

func (s *responsesStreamRuntime) sendEvent(event string, payload map[string]any) {
	b, _ := json.Marshal(payload)
	_, _ = s.w.Write([]byte("event: " + event + "\n"))
	_, _ = s.w.Write([]byte("data: "))
	_, _ = s.w.Write(b)
	_, _ = s.w.Write([]byte("\n\n"))
	if s.canFlush {
		_ = s.rc.Flush()
	}
}

func (s *responsesStreamRuntime) sendCreated() {
	s.sendEvent("response.created", openaifmt.BuildResponsesCreatedPayload(s.responseID, s.model))
}

func (s *responsesStreamRuntime) sendDone() {
	_, _ = s.w.Write([]byte("data: [DONE]\n\n"))
	if s.canFlush {
		_ = s.rc.Flush()
	}
}

func (s *responsesStreamRuntime) finalize() {
	finalThinking := s.thinking.String()
	finalText := s.text.String()
	if s.bufferToolContent {
		for _, evt := range flushToolSieve(&s.sieve, s.toolNames) {
			if evt.Content != "" {
				s.sendEvent("response.output_text.delta", openaifmt.BuildResponsesTextDeltaPayload(s.responseID, evt.Content))
			}
			if len(evt.ToolCalls) > 0 {
				s.toolCallsEmitted = true
				s.sendEvent("response.output_tool_call.done", openaifmt.BuildResponsesToolCallDonePayload(s.responseID, util.FormatOpenAIStreamToolCalls(evt.ToolCalls)))
			}
		}
	}

	obj := openaifmt.BuildResponseObject(s.responseID, s.model, s.finalPrompt, finalThinking, finalText, s.toolNames)
	if s.toolCallsEmitted {
		obj["status"] = "completed"
	}
	if s.persistResponse != nil {
		s.persistResponse(obj)
	}
	s.sendEvent("response.completed", openaifmt.BuildResponsesCompletedPayload(obj))
	s.sendDone()
}

func (s *responsesStreamRuntime) onParsed(parsed sse.LineResult) streamengine.ParsedDecision {
	if !parsed.Parsed {
		return streamengine.ParsedDecision{}
	}
	if parsed.ContentFilter || parsed.ErrorMessage != "" || parsed.Stop {
		return streamengine.ParsedDecision{Stop: true}
	}

	contentSeen := false
	for _, p := range parsed.Parts {
		if p.Text == "" {
			continue
		}
		if p.Type != "thinking" && s.searchEnabled && sse.IsCitation(p.Text) {
			continue
		}
		contentSeen = true
		if p.Type == "thinking" {
			if !s.thinkingEnabled {
				continue
			}
			s.thinking.WriteString(p.Text)
			s.sendEvent("response.reasoning.delta", openaifmt.BuildResponsesReasoningDeltaPayload(s.responseID, p.Text))
			continue
		}

		s.text.WriteString(p.Text)
		if !s.bufferToolContent {
			s.sendEvent("response.output_text.delta", openaifmt.BuildResponsesTextDeltaPayload(s.responseID, p.Text))
			continue
		}
		for _, evt := range processToolSieveChunk(&s.sieve, p.Text, s.toolNames) {
			if evt.Content != "" {
				s.sendEvent("response.output_text.delta", openaifmt.BuildResponsesTextDeltaPayload(s.responseID, evt.Content))
			}
			if len(evt.ToolCallDeltas) > 0 {
				if !s.emitEarlyToolDeltas {
					continue
				}
				s.toolCallsEmitted = true
				s.sendEvent("response.output_tool_call.delta", openaifmt.BuildResponsesToolCallDeltaPayload(s.responseID, formatIncrementalStreamToolCallDeltas(evt.ToolCallDeltas, s.streamToolCallIDs)))
			}
			if len(evt.ToolCalls) > 0 {
				s.toolCallsEmitted = true
				s.sendEvent("response.output_tool_call.done", openaifmt.BuildResponsesToolCallDonePayload(s.responseID, util.FormatOpenAIStreamToolCalls(evt.ToolCalls)))
			}
		}
	}

	return streamengine.ParsedDecision{ContentSeen: contentSeen}
}
