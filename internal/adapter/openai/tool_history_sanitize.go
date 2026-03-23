package openai

import (
	"regexp"
)

var leakedToolHistoryPattern = regexp.MustCompile(`(?is)\[TOOL_CALL_HISTORY\][\s\S]*?\[/TOOL_CALL_HISTORY\]|\[TOOL_RESULT_HISTORY\][\s\S]*?\[/TOOL_RESULT_HISTORY\]`)
var emptyJSONFencePattern = regexp.MustCompile("(?is)```json\\s*```")

func sanitizeLeakedToolHistory(text string) string {
	if text == "" {
		return text
	}
	out := leakedToolHistoryPattern.ReplaceAllString(text, "")
	out = emptyJSONFencePattern.ReplaceAllString(out, "")
	return out
}
