package toolstream

import (
	"strings"
	"testing"
)

func TestWhaleColonToolCallStreamsWithoutLeakingMarkup(t *testing.T) {
	input := `::tc::
::invoke name="read_file"::
::param name="path"::[[/tmp/a.txt]]::/param::
::/invoke::
::/tc::`
	var state State
	var events []Event
	for _, ch := range input {
		events = append(events, ProcessChunk(&state, string(ch), []string{"read_file"})...)
	}
	events = append(events, Flush(&state, []string{"read_file"})...)
	var text strings.Builder
	calls := 0
	for _, event := range events {
		text.WriteString(event.Content)
		calls += len(event.ToolCalls)
	}
	if calls != 1 || strings.Contains(text.String(), "::tc") || strings.Contains(text.String(), "::invoke") {
		t.Fatalf("calls=%d leaked text=%q events=%#v", calls, text.String(), events)
	}
}
