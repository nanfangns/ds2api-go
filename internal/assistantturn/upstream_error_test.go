package assistantturn

import (
	"net/http"
	"strings"
	"testing"
)

func TestThinkingOnlyDoesNotClaimUpstreamRateLimit(t *testing.T) {
	status, message, code := UpstreamEmptyOutputDetail(false, "", "reasoning")
	if status != http.StatusBadGateway || code != "upstream_empty_output" || strings.Contains(strings.ToLower(message), "rate limit") {
		t.Fatalf("unexpected empty-output error: %d %q %q", status, code, message)
	}
}
