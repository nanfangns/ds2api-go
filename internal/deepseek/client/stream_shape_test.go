package client

import (
	"strings"
	"testing"
)

func TestUpstreamStreamShapeCountsProtocolMarkers(t *testing.T) {
	s := &continueState{}
	for _, chunk := range []string{
		`{"v":{"response":{"fragments":[{"type":"THINK","content":"private"}]}}}`,
		`{"p":"response/fragments","v":[{"type":"RESPONSE","content":"private answer"}]}`,
		`{"p":"response/fragments/-1/content","v":"more private text"}`,
		`{"v":"more private text"}`,
		`{"p":"response/status","v":"FINISHED"}`,
	} {
		s.observe(chunk)
	}
	if s.shape.events != 5 || s.shape.thinkingFragments != 1 || s.shape.responseFragments != 1 || s.shape.fragmentsWithoutOp != 1 || s.shape.fragmentContentParts != 1 || s.shape.pathlessParts != 1 || s.lastStatus != "FINISHED" {
		t.Fatalf("unexpected diagnostic counts: %+v", s.shape)
	}
}

func TestSearchShapeTracksOnlySafeProtocolNames(t *testing.T) {
	s := &continueState{shape: upstreamStreamShape{searchTrace: true, parserType: "thinking"}}
	for _, chunk := range []string{
		`{"v":{"response":{"fragments":[{"type":"THINK","content":"private"}]}}}`,
		`{"p":"response/fragments","o":"APPEND","v":[{"type":"TOOL_SEARCH"}]}`,
		`{"p":"response/fragments","v":[{"type":"RESPONSE"}]}`,
		`{"v":"private answer"}`,
		`{"p":"response/fragments","o":"secret","v":[{"type":"private-content"}]}`,
	} {
		s.observe(chunk)
	}
	if s.shape.parsedTextEvents != 1 || s.shape.parserType != "text" {
		t.Fatalf("unexpected parser diagnostics: %+v", s.shape)
	}
	joined := strings.Join(s.shape.fragmentSequence, ",")
	if joined != "THINK:INITIAL,TOOL_SEARCH:APPEND,RESPONSE:NONE,OTHER:OTHER" {
		t.Fatalf("unsafe or unexpected sequence: %s", joined)
	}
}
