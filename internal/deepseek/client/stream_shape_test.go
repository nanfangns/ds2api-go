package client

import "testing"

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
