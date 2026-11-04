package config

import (
	"os"
	"strconv"
	"strings"
)

// These limits are a conservative preflight guard for the current DeepSeek
// web-backed integration. The user-facing value is intentionally separate
// from the local tokenizer estimate; both can be overridden per deployment.
const (
	DefaultUserFacingContextLimitTokens = 896_000
	DefaultLogicalContextLimitTokens    = 2_850_000
)

const (
	envUserFacingContextLimit = "DS2API_USER_FACING_CONTEXT_LIMIT_TOKENS"
	envLogicalContextLimit    = "DS2API_LOGICAL_CONTEXT_LIMIT_TOKENS"
)

var (
	EffectiveUserFacingContextLimitTokens = DefaultUserFacingContextLimitTokens
	EffectiveLogicalContextLimitTokens    = DefaultLogicalContextLimitTokens
)

func init() {
	LoadContextLimitOverrides()
}

// LoadContextLimitOverrides reads optional deployment-specific limits.
// Invalid values reset the pair to known defaults rather than leaving a
// partially applied configuration.
func LoadContextLimitOverrides() {
	user := DefaultUserFacingContextLimitTokens
	logical := DefaultLogicalContextLimitTokens
	valid := true

	userRaw := strings.TrimSpace(os.Getenv(envUserFacingContextLimit))
	logicalRaw := strings.TrimSpace(os.Getenv(envLogicalContextLimit))

	if userRaw != "" {
		parsed, err := strconv.Atoi(userRaw)
		if err != nil || parsed <= 0 {
			Logger.Warn("[context] invalid user-facing context limit override; using defaults",
				"env", envUserFacingContextLimit, "value", userRaw)
			valid = false
		} else {
			user = parsed
		}
	}
	if logicalRaw != "" {
		parsed, err := strconv.Atoi(logicalRaw)
		if err != nil || parsed <= 0 {
			Logger.Warn("[context] invalid logical context limit override; using defaults",
				"env", envLogicalContextLimit, "value", logicalRaw)
			valid = false
		} else {
			logical = parsed
		}
	}
	if valid && logical < user {
		Logger.Warn("[context] logical context limit is below the user-facing limit; using defaults",
			"logical", logical, "user_facing", user)
		valid = false
	}
	if !valid {
		user = DefaultUserFacingContextLimitTokens
		logical = DefaultLogicalContextLimitTokens
	}

	EffectiveUserFacingContextLimitTokens = user
	EffectiveLogicalContextLimitTokens = logical
}
