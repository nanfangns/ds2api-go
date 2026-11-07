package client

import (
	"strings"

	"ds2api/internal/config"
	"ds2api/internal/sse"
)

// upstreamStreamShape records protocol structure only, never prompts,
// responses, credentials, file IDs, or account identifiers.
type upstreamStreamShape struct {
	events               int
	thinkingFragments    int
	responseFragments    int
	otherFragments       int
	fragmentsWithoutOp   int
	fragmentContentParts int
	textContentParts     int
	thinkingContentParts int
	pathlessParts        int
	searchTrace          bool
	parserType           string
	parsedTextEvents     int
	parsedThinkingEvents int
	fragmentSequence     []string
}

func (s *upstreamStreamShape) observe(chunk map[string]any) {
	s.events++
	if s.searchTrace {
		parts, _, _, nextType := sse.ParseSSEChunkForContentDetailed(chunk, true, s.parserType)
		s.parserType = nextType
		for _, part := range parts {
			switch part.Type {
			case "text":
				s.parsedTextEvents++
			case "thinking":
				s.parsedThinkingEvents++
			}
		}
	}
	path, _ := chunk["p"].(string)
	v := chunk["v"]
	switch path {
	case "response/fragments":
		op, _ := chunk["o"].(string)
		s.observeFragments(v, op)
	case "response":
		if patches, ok := v.([]any); ok {
			for _, raw := range patches {
				patch, ok := raw.(map[string]any)
				if !ok || patch["p"] != "fragments" {
					continue
				}
				op, _ := patch["o"].(string)
				s.observeFragments(patch["v"], op)
			}
		}
	case "response/content":
		if text, ok := v.(string); ok && text != "" {
			s.textContentParts++
		}
	case "response/thinking_content":
		if text, ok := v.(string); ok && text != "" {
			s.thinkingContentParts++
		}
	case "":
		if text, ok := v.(string); ok && text != "" {
			s.pathlessParts++
		}
		if payload, ok := v.(map[string]any); ok {
			if response, ok := payload["response"].(map[string]any); ok {
				s.observeFragments(response["fragments"], "initial")
			}
		}
	default:
		if strings.HasPrefix(path, "response/fragments/") && strings.HasSuffix(path, "/content") {
			if text, ok := v.(string); ok && text != "" {
				s.fragmentContentParts++
			}
		}
	}
}

func (s *upstreamStreamShape) observeFragments(raw any, op string) {
	fragments, ok := raw.([]any)
	if !ok {
		return
	}
	if strings.TrimSpace(op) == "" {
		s.fragmentsWithoutOp++
	}
	for _, item := range fragments {
		fragment, ok := item.(map[string]any)
		if !ok {
			continue
		}
		kind, _ := fragment["type"].(string)
		kind = strings.ToUpper(strings.TrimSpace(kind))
		if s.searchTrace && len(s.fragmentSequence) < 40 {
			s.fragmentSequence = append(s.fragmentSequence, safeFragmentKind(kind)+":"+safeFragmentOperation(op))
		}
		switch kind {
		case "THINK", "THINKING":
			s.thinkingFragments++
		case "RESPONSE":
			s.responseFragments++
		default:
			s.otherFragments++
		}
	}
}

func safeFragmentKind(kind string) string {
	switch kind {
	case "THINK", "THINKING", "RESPONSE", "TOOL_SEARCH", "TOOL_OPEN", "TIP", "TEMPLATE_RESPONSE":
		return kind
	default:
		return "OTHER"
	}
}

func safeFragmentOperation(op string) string {
	switch strings.ToUpper(strings.TrimSpace(op)) {
	case "APPEND":
		return "APPEND"
	case "SET":
		return "SET"
	case "INITIAL":
		return "INITIAL"
	case "":
		return "NONE"
	default:
		return "OTHER"
	}
}

func (s upstreamStreamShape) log(lastStatus string) {
	if s.events == 0 {
		return
	}
	status := strings.ToUpper(strings.TrimSpace(lastStatus))
	switch status {
	case "", "WIP", "FINISHED", "INCOMPLETE", "AUTO_CONTINUE", "CONTENT_FILTER":
	default:
		status = "other"
	}
	config.Logger.Info("[upstream_stream_shape]", "events", s.events,
		"thinking_fragments", s.thinkingFragments, "response_fragments", s.responseFragments,
		"other_fragments", s.otherFragments, "fragments_without_op", s.fragmentsWithoutOp,
		"fragment_content_parts", s.fragmentContentParts, "text_content_parts", s.textContentParts,
		"thinking_content_parts", s.thinkingContentParts, "pathless_parts", s.pathlessParts, "status", status)
	if s.searchTrace {
		config.Logger.Info("[upstream_search_shape]", "fragment_sequence", strings.Join(s.fragmentSequence, ","),
			"parsed_text_events", s.parsedTextEvents, "parsed_thinking_events", s.parsedThinkingEvents,
			"last_parser_type", s.parserType)
	}
}
