// Package accounthealth recognizes explicit upstream account restrictions.
package accounthealth

import (
	"encoding/json"
	"strings"
	"time"
)

// Classify requires an explicit account restriction; device risk, network
// errors, generic HTTP 403s and ambiguous business codes are not account bans.
func Classify(raw []byte, now time.Time) (string, int64) {
	text := strings.ToLower(strings.TrimSpace(string(raw)))
	if text == "" || strings.Contains(text, "risk_device_detected") || strings.Contains(text, "device fingerprint") || strings.Contains(text, "device_rejected") {
		return "", 0
	}
	status := ""
	until := int64(0)
	var body map[string]any
	if json.Unmarshal(raw, &body) == nil {
		data, _ := body["data"].(map[string]any)
		if code, ok := data["biz_code"].(float64); ok && code == 11 {
			return "", 0
		}
		bizData, _ := data["biz_data"].(map[string]any)
		muted, _ := bizData["is_muted"].(float64)
		if muted == 1 {
			status = "muted"
		}
		for _, key := range []string{"mute_until", "muted_until"} {
			if n, ok := bizData[key].(float64); ok && n > 0 {
				until = int64(n)
				break
			}
		}
		// Examine messages only, never model output or unrelated JSON fields.
		text = ""
		if msg, ok := body["msg"].(string); ok {
			text += strings.ToLower(msg) + " "
		}
		if msg, ok := data["biz_msg"].(string); ok {
			text += strings.ToLower(msg)
		}
	}
	if strings.Contains(text, "not muted") || strings.Contains(text, "not banned") {
		return "", 0
	}
	if strings.Contains(text, "user is muted") || strings.Contains(text, "account is muted") || strings.Contains(text, "禁言") {
		status = "muted"
	}
	if strings.Contains(text, "account banned") || strings.Contains(text, "account has been banned") || strings.Contains(text, "account is banned") || strings.Contains(text, "封号") || strings.Contains(text, "封禁") {
		status = "banned"
	}
	if status == "muted" && until <= now.Unix() {
		until = now.Add(time.Hour).Unix()
	}
	if status != "muted" {
		until = 0
	}
	return status, until
}
