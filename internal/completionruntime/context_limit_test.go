package completionruntime

import (
	"net/http"
	"testing"

	"ds2api/internal/config"
	"ds2api/internal/promptcompat"
)

func TestUserFacingTokenEstimateScalesInternalCount(t *testing.T) {
	oldUser := config.EffectiveUserFacingContextLimitTokens
	oldLogical := config.EffectiveLogicalContextLimitTokens
	config.EffectiveUserFacingContextLimitTokens = 896_000
	config.EffectiveLogicalContextLimitTokens = 2_850_000
	t.Cleanup(func() {
		config.EffectiveUserFacingContextLimitTokens = oldUser
		config.EffectiveLogicalContextLimitTokens = oldLogical
	})

	if got := userFacingTokenEstimate(2_850_000); got != 896_000 {
		t.Fatalf("expected 896000 at gate, got %d", got)
	}
	if got := userFacingTokenEstimate(1_425_000); got != 448_000 {
		t.Fatalf("expected 448000 at half gate, got %d", got)
	}
}

func TestContextGateErrorReturnsBadRequestBeforeUpstream(t *testing.T) {
	oldUser := config.EffectiveUserFacingContextLimitTokens
	oldLogical := config.EffectiveLogicalContextLimitTokens
	config.EffectiveUserFacingContextLimitTokens = 896_000
	config.EffectiveLogicalContextLimitTokens = 2_850_000
	t.Cleanup(func() {
		config.EffectiveUserFacingContextLimitTokens = oldUser
		config.EffectiveLogicalContextLimitTokens = oldLogical
	})

	stdReq := promptcompat.StandardRequest{
		ResolvedModel: "deepseek-v4-flash",
		RefFileTokens: 2_850_001,
		PassThrough:   map[string]any{"max_tokens": float64(100)},
	}
	outErr := ContextGateError(stdReq, nil)
	if outErr == nil {
		t.Fatal("expected context gate error")
	}
	if outErr.Status != http.StatusBadRequest || outErr.Code != "invalid_request_error" || outErr.Param != "" {
		t.Fatalf("unexpected error shape: %#v", outErr)
	}
	if outErr.Message == "" {
		t.Fatal("expected context error message")
	}
}
