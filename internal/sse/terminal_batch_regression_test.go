package sse

import (
	"context"
	"strings"
	"testing"
)

const terminalBatchStream = "data: {\"p\":\"response/thinking_content\",\"v\":\"think\"}\n" +
	"data: {\"p\":\"response\",\"o\":\"BATCH\",\"v\":[{\"p\":\"fragments\",\"o\":\"APPEND\",\"v\":[{\"type\":\"RESPONSE\",\"content\":\"answer\"}]},{\"p\":\"status\",\"v\":\"FINISHED\"}]}\n" +
	"data: [DONE]\n"

const searchFragmentStatusStream = "data: {\"p\":\"response/fragments\",\"v\":[{\"type\":\"TOOL_SEARCH\"}]}\n" +
	"data: {\"p\":\"response/fragments/-1\",\"o\":\"BATCH\",\"v\":[{\"p\":\"status\",\"v\":\"FINISHED\"}]}\n" +
	"data: {\"p\":\"response/fragments\",\"v\":[{\"type\":\"RESPONSE\"}]}\n" +
	"data: {\"v\":\"search answer\"}\n" +
	"data: {\"p\":\"response/status\",\"v\":\"FINISHED\"}\n" +
	"data: [DONE]\n"

func TestSearchFragmentFinishedDoesNotStopNonStreamingResponse(t *testing.T) {
	result := CollectStream(makeHTTPResponse(searchFragmentStatusStream), true, true)
	if result.Text != "search answer" {
		t.Fatalf("search answer lost after fragment finished: %+v", result)
	}
}

func TestSearchFragmentFinishedDoesNotStopStreamingResponse(t *testing.T) {
	results, done := StartParsedLinePump(context.Background(), strings.NewReader(searchFragmentStatusStream), true, "thinking")
	var answer strings.Builder
	for result := range results {
		if result.Stop {
			break
		}
		for _, part := range result.Parts {
			if part.Type == "text" {
				answer.WriteString(part.Text)
			}
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if answer.String() != "search answer" {
		t.Fatalf("search answer lost after fragment finished: %q", answer.String())
	}
}

func TestTerminalBatchPreservesAnswerInNonStream(t *testing.T) {
	result := CollectStream(makeHTTPResponse(terminalBatchStream), true, true)
	if result.Thinking != "think" || result.Text != "answer" {
		t.Fatalf("terminal batch lost output: thinking=%q text=%q", result.Thinking, result.Text)
	}
}

func TestTerminalBatchPreservesAnswerInStream(t *testing.T) {
	results, done := StartParsedLinePump(context.Background(), strings.NewReader(terminalBatchStream), true, "thinking")
	var thinking, answer strings.Builder
	for result := range results {
		for _, part := range result.Parts {
			if part.Type == "thinking" {
				thinking.WriteString(part.Text)
			} else {
				answer.WriteString(part.Text)
			}
		}
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if thinking.String() != "think" || answer.String() != "answer" {
		t.Fatalf("terminal batch lost output: thinking=%q text=%q", thinking.String(), answer.String())
	}
}

func TestResponseFragmentWithoutOperationSwitchesFromThinking(t *testing.T) {
	line := []byte(`data: {"p":"response/fragments","v":[{"type":"RESPONSE","content":"answer"}]}`)
	result := ParseDeepSeekContentLine(line, true, "thinking")
	if result.Stop || result.NextType != "text" || len(result.Parts) != 1 || result.Parts[0].Text != "answer" || result.Parts[0].Type != "text" {
		t.Fatalf("response fragment without an operation was not recognized: %+v", result)
	}
}

func TestNestedResponseFragmentWithoutOperationUpdatesFollowingChunks(t *testing.T) {
	line := []byte(`data: {"p":"response","o":"BATCH","v":[{"p":"fragments","v":[{"type":"RESPONSE","content":"ans"}]}]}`)
	result := ParseDeepSeekContentLine(line, true, "thinking")
	if result.NextType != "text" || len(result.Parts) != 1 || result.Parts[0].Type != "text" {
		t.Fatalf("nested response did not update content type: %+v", result)
	}
	continued := ParseDeepSeekContentLine([]byte(`data: {"v":"wer"}`), true, result.NextType)
	if len(continued.Parts) != 1 || continued.Parts[0].Type != "text" {
		t.Fatalf("continued answer classified as thinking: %+v", continued)
	}
}

func TestTipFragmentWithoutOperationIsNotAnswer(t *testing.T) {
	result := ParseDeepSeekContentLine([]byte(`data: {"p":"response/fragments","v":[{"type":"TIP","content":"notice"}]}`), true, "thinking")
	if len(result.Parts) != 0 || result.NextType != "thinking" {
		t.Fatalf("tip was treated as answer: %+v", result)
	}
}
