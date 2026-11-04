package toolcall

import (
	"reflect"
	"testing"
)

func TestParseWhaleColonToolCall(t *testing.T) {
	text := `::tc::
::invoke name="read_file"::
::param name="path"::[[/tmp/a.txt]]::/param::
::/invoke::
::/tc::`
	calls := ParseToolCalls(text, []string{"read_file"})
	if len(calls) != 1 || calls[0].Name != "read_file" || calls[0].Input["path"] != "/tmp/a.txt" {
		t.Fatalf("unexpected calls: %#v", calls)
	}
}

func TestColonToolMarkupInCodeIsNotExecuted(t *testing.T) {
	text := "```text\n::tc::::invoke name=\"read_file\"::::param name=\"path\"::[[a]]::/param::::/invoke::::/tc::\n```"
	if calls := ParseToolCalls(text, []string{"read_file"}); len(calls) != 0 {
		t.Fatalf("code example executed: %#v", calls)
	}
}

func TestParseColonToolCallStructuredAndRawValues(t *testing.T) {
	text := `::tc::::invoke name="write_file"::` +
		`::param name="content"::[[before ::/tc:: and ] ] ]] ]] after]]::/param::` +
		`::param name="options"::[[{"items":[1,2],"active":true}]]::/param::` +
		`::/invoke::::/tc::`
	calls := ParseToolCalls(text, []string{"write_file"})
	if len(calls) != 1 {
		t.Fatalf("expected one tool call: %#v", calls)
	}
	if calls[0].Input["content"] != "before ::/tc:: and ] ] ]] after" {
		t.Fatalf("raw content changed: %#v", calls[0].Input["content"])
	}
	want := map[string]any{"items": []any{float64(1), float64(2)}, "active": true}
	if !reflect.DeepEqual(calls[0].Input["options"], want) {
		t.Fatalf("structured argument = %#v, want %#v", calls[0].Input["options"], want)
	}
}

func TestColonRawMarkersDoNotChangeOrdinaryTextOrLegacyMarkup(t *testing.T) {
	text := `ordinary [[ text ` +
		`<tool_calls><invoke name="write_file"><parameter name="content">[[legacy]]</parameter></invoke></tool_calls>`
	calls := ParseToolCalls(text, []string{"write_file"})
	if len(calls) != 1 || calls[0].Input["content"] != "[[legacy]]" {
		t.Fatalf("legacy markup after ordinary brackets was altered: %#v", calls)
	}
}
