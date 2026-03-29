package openai

import "testing"

func TestSanitizeLeakedToolHistoryRemovesMarkerBlocks(t *testing.T) {
	raw := "前缀\n[TOOL_CALL_HISTORY]\nfunction.name: exec\nfunction.arguments: {}\n[/TOOL_CALL_HISTORY]\n后缀"
	got := sanitizeLeakedToolHistory(raw)
	if got != "前缀\n\n后缀" {
		t.Fatalf("unexpected sanitized content: %q", got)
	}
}

func TestSanitizeLeakedToolHistoryPreservesChunkWhitespace(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "trailing space kept",
			raw:  "Hello ",
			want: "Hello ",
		},
		{
			name: "leading newline kept",
			raw:  "\nworld",
			want: "\nworld",
		},
		{
			name: "surrounding whitespace around marker is preserved",
			raw:  "A \n[TOOL_RESULT_HISTORY]\nfunction.name: exec\nfunction.arguments: {}\n[/TOOL_RESULT_HISTORY]\n B",
			want: "A \n\n B",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeLeakedToolHistory(tc.raw)
			if got != tc.want {
				t.Fatalf("unexpected sanitize result, want %q got %q", tc.want, got)
			}
		})
	}
}

func TestSanitizeLeakedToolHistoryRemovesEmptyJSONFence(t *testing.T) {
	raw := "before\n```json\n```\nafter"
	got := sanitizeLeakedToolHistory(raw)
	if got != "before\n\nafter" {
		t.Fatalf("unexpected sanitized empty json fence: %q", got)
	}
}

func TestFlushToolSieveDropsToolHistoryLeak(t *testing.T) {
	var state toolStreamSieveState
	chunk := "[TOOL_CALL_HISTORY]\nstatus: already_called\nfunction.name: exec\nfunction.arguments: {}\n[/TOOL_CALL_HISTORY]"
	evts := processToolSieveChunk(&state, chunk, []string{"exec"})
	if len(evts) != 0 {
		t.Fatalf("expected no immediate output before history block is complete, got %+v", evts)
	}
	flushed := flushToolSieve(&state, []string{"exec"})
	if len(flushed) != 0 {
		t.Fatalf("expected history block to be swallowed, got %+v", flushed)
	}
}

func TestFlushToolSieveDropsToolResultHistoryLeak(t *testing.T) {
	var state toolStreamSieveState
	chunk := "[TOOL_RESULT_HISTORY]\nstatus: already_called\nfunction.name: exec\nfunction.arguments: {}\n[/TOOL_RESULT_HISTORY]"
	evts := processToolSieveChunk(&state, chunk, []string{"exec"})
	if len(evts) != 0 {
		t.Fatalf("expected no immediate output before result history block is complete, got %+v", evts)
	}
	flushed := flushToolSieve(&state, []string{"exec"})
	if len(flushed) != 0 {
		t.Fatalf("expected result history block to be swallowed, got %+v", flushed)
	}
}

func TestSanitizeLeakedToolHistoryRemovesLeakedWireToolCallAndResult(t *testing.T) {
	raw := "开始\n[{\"function\":{\"arguments\":\"{\\\"command\\\":\\\"java -version\\\"}\",\"name\":\"exec\"},\"id\":\"callb9a321\",\"type\":\"function\"}]< | Tool | >{\"content\":\"openjdk version 21\",\"tool_call_id\":\"callb9a321\"}\n结束"
	got := sanitizeLeakedToolHistory(raw)
	if got != "开始\n\n结束" {
		t.Fatalf("unexpected sanitize result for leaked wire format: %q", got)
	}
}

func TestSanitizeLeakedToolHistoryRemovesStandaloneMetaMarkers(t *testing.T) {
	raw := "A<| end_of_sentence |><| Assistant |>B<| end_of_thinking |>C<｜end▁of▁thinking｜>D<｜end▁of▁sentence｜>E"
	got := sanitizeLeakedToolHistory(raw)
	if got != "ABCDE" {
		t.Fatalf("unexpected sanitize result for meta markers: %q", got)
	}
}

func TestSanitizeLeakedToolHistoryRemovesAgentXMLLeaks(t *testing.T) {
	raw := "Done.<attempt_completion><result>Some final answer</result></attempt_completion>"
	got := sanitizeLeakedToolHistory(raw)
	if got != "Done.Some final answer" {
		t.Fatalf("unexpected sanitize result for agent XML leak: %q", got)
	}
}

func TestProcessToolSieveChunkSplitsResultHistoryBoundary(t *testing.T) {
	var state toolStreamSieveState
	parts := []string{
		"Hello ",
		"[TOOL_RESULT_HISTORY]\nstatus: already_called\n",
		"function.name: exec\nfunction.arguments: {}\n[/TOOL_RESULT_HISTORY]",
		"world",
	}
	var events []toolStreamEvent
	for _, p := range parts {
		events = append(events, processToolSieveChunk(&state, p, []string{"exec"})...)
	}
	events = append(events, flushToolSieve(&state, []string{"exec"})...)

	var text string
	for _, evt := range events {
		if evt.Content != "" {
			text += evt.Content
		}
		if len(evt.ToolCalls) > 0 {
			t.Fatalf("did not expect parsed tool calls from history leak: %+v", evt.ToolCalls)
		}
	}
	if text != "Hello world" {
		t.Fatalf("expected clean text output preserving boundary spaces, got %q", text)
	}
}
