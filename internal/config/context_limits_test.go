package config

import "testing"

func TestLoadContextLimitOverrides(t *testing.T) {
	oldUser := EffectiveUserFacingContextLimitTokens
	oldLogical := EffectiveLogicalContextLimitTokens
	t.Cleanup(func() {
		EffectiveUserFacingContextLimitTokens = oldUser
		EffectiveLogicalContextLimitTokens = oldLogical
	})

	t.Setenv(envUserFacingContextLimit, "700000")
	t.Setenv(envLogicalContextLimit, "2100000")
	LoadContextLimitOverrides()
	if EffectiveUserFacingContextLimitTokens != 700000 || EffectiveLogicalContextLimitTokens != 2100000 {
		t.Fatalf("unexpected limits: user=%d logical=%d", EffectiveUserFacingContextLimitTokens, EffectiveLogicalContextLimitTokens)
	}

	t.Setenv(envUserFacingContextLimit, "not-a-number")
	LoadContextLimitOverrides()
	if EffectiveUserFacingContextLimitTokens != DefaultUserFacingContextLimitTokens || EffectiveLogicalContextLimitTokens != DefaultLogicalContextLimitTokens {
		t.Fatalf("invalid override should reset defaults: user=%d logical=%d", EffectiveUserFacingContextLimitTokens, EffectiveLogicalContextLimitTokens)
	}
}
