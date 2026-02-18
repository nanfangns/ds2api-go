package admin

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	authn "ds2api/internal/auth"
	"ds2api/internal/config"
)

func (h *Handler) getSettings(w http.ResponseWriter, _ *http.Request) {
	snap := h.Store.Snapshot()
	recommended := defaultRuntimeRecommended(len(snap.Accounts), h.Store.RuntimeAccountMaxInflight())
	needsSync := config.IsVercel() && snap.VercelSyncHash != "" && snap.VercelSyncHash != h.computeSyncHash()
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"admin": map[string]any{
			"has_password_hash":        strings.TrimSpace(snap.Admin.PasswordHash) != "",
			"jwt_expire_hours":         h.Store.AdminJWTExpireHours(),
			"jwt_valid_after_unix":     snap.Admin.JWTValidAfterUnix,
			"default_password_warning": authn.UsingDefaultAdminKey(h.Store),
		},
		"runtime": map[string]any{
			"account_max_inflight": h.Store.RuntimeAccountMaxInflight(),
			"account_max_queue":    h.Store.RuntimeAccountMaxQueue(recommended),
			"global_max_inflight":  h.Store.RuntimeGlobalMaxInflight(recommended),
		},
		"toolcall":          snap.Toolcall,
		"responses":         snap.Responses,
		"embeddings":        snap.Embeddings,
		"claude_mapping":    settingsClaudeMapping(snap),
		"model_aliases":     snap.ModelAliases,
		"env_backed":        h.Store.IsEnvBacked(),
		"needs_vercel_sync": needsSync,
	})
}

func (h *Handler) updateSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}

	adminCfg, runtimeCfg, toolcallCfg, responsesCfg, embeddingsCfg, claudeMap, aliasMap, err := parseSettingsUpdateRequest(req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
		return
	}
	if runtimeCfg != nil {
		if err := validateMergedRuntimeSettings(h.Store.Snapshot().Runtime, runtimeCfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
			return
		}
	}

	if err := h.Store.Update(func(c *config.Config) error {
		if adminCfg != nil {
			if adminCfg.JWTExpireHours > 0 {
				c.Admin.JWTExpireHours = adminCfg.JWTExpireHours
			}
		}
		if runtimeCfg != nil {
			if runtimeCfg.AccountMaxInflight > 0 {
				c.Runtime.AccountMaxInflight = runtimeCfg.AccountMaxInflight
			}
			if runtimeCfg.AccountMaxQueue > 0 {
				c.Runtime.AccountMaxQueue = runtimeCfg.AccountMaxQueue
			}
			if runtimeCfg.GlobalMaxInflight > 0 {
				c.Runtime.GlobalMaxInflight = runtimeCfg.GlobalMaxInflight
			}
		}
		if toolcallCfg != nil {
			if strings.TrimSpace(toolcallCfg.Mode) != "" {
				c.Toolcall.Mode = strings.TrimSpace(toolcallCfg.Mode)
			}
			if strings.TrimSpace(toolcallCfg.EarlyEmitConfidence) != "" {
				c.Toolcall.EarlyEmitConfidence = strings.TrimSpace(toolcallCfg.EarlyEmitConfidence)
			}
		}
		if responsesCfg != nil && responsesCfg.StoreTTLSeconds > 0 {
			c.Responses.StoreTTLSeconds = responsesCfg.StoreTTLSeconds
		}
		if embeddingsCfg != nil && strings.TrimSpace(embeddingsCfg.Provider) != "" {
			c.Embeddings.Provider = strings.TrimSpace(embeddingsCfg.Provider)
		}
		if claudeMap != nil {
			c.ClaudeMapping = claudeMap
			c.ClaudeModelMap = nil
		}
		if aliasMap != nil {
			c.ModelAliases = aliasMap
		}
		return nil
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	h.applyRuntimeSettings()
	needsSync := config.IsVercel() || h.Store.IsEnvBacked()
	writeJSON(w, http.StatusOK, map[string]any{
		"success":             true,
		"message":             "settings updated and hot reloaded",
		"env_backed":          h.Store.IsEnvBacked(),
		"needs_vercel_sync":   needsSync,
		"manual_sync_message": "配置已保存。Vercel 部署请在 Vercel Sync 页面手动同步。",
	})
}

func validateMergedRuntimeSettings(current config.RuntimeConfig, incoming *config.RuntimeConfig) error {
	merged := current
	if incoming != nil {
		if incoming.AccountMaxInflight > 0 {
			merged.AccountMaxInflight = incoming.AccountMaxInflight
		}
		if incoming.AccountMaxQueue > 0 {
			merged.AccountMaxQueue = incoming.AccountMaxQueue
		}
		if incoming.GlobalMaxInflight > 0 {
			merged.GlobalMaxInflight = incoming.GlobalMaxInflight
		}
	}
	return validateRuntimeSettings(merged)
}

func (h *Handler) updateSettingsPassword(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}
	newPassword := strings.TrimSpace(fieldString(req, "new_password"))
	if newPassword == "" {
		newPassword = strings.TrimSpace(fieldString(req, "password"))
	}
	if len(newPassword) < 4 {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "new password must be at least 4 characters"})
		return
	}

	now := time.Now().Unix()
	hash := authn.HashAdminPassword(newPassword)
	if err := h.Store.Update(func(c *config.Config) error {
		c.Admin.PasswordHash = hash
		c.Admin.JWTValidAfterUnix = now
		return nil
	}); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success":              true,
		"message":              "password updated",
		"force_relogin":        true,
		"jwt_valid_after_unix": now,
	})
}

func (h *Handler) applyRuntimeSettings() {
	if h == nil || h.Store == nil || h.Pool == nil {
		return
	}
	accountCount := len(h.Store.Accounts())
	maxPer := h.Store.RuntimeAccountMaxInflight()
	recommended := defaultRuntimeRecommended(accountCount, maxPer)
	maxQueue := h.Store.RuntimeAccountMaxQueue(recommended)
	global := h.Store.RuntimeGlobalMaxInflight(recommended)
	h.Pool.ApplyRuntimeLimits(maxPer, maxQueue, global)
}

func defaultRuntimeRecommended(accountCount, maxPer int) int {
	if maxPer <= 0 {
		maxPer = 1
	}
	if accountCount <= 0 {
		return maxPer
	}
	return accountCount * maxPer
}

func settingsClaudeMapping(c config.Config) map[string]string {
	if len(c.ClaudeMapping) > 0 {
		return c.ClaudeMapping
	}
	if len(c.ClaudeModelMap) > 0 {
		return c.ClaudeModelMap
	}
	return map[string]string{"fast": "deepseek-chat", "slow": "deepseek-reasoner"}
}

func parseSettingsUpdateRequest(req map[string]any) (*config.AdminConfig, *config.RuntimeConfig, *config.ToolcallConfig, *config.ResponsesConfig, *config.EmbeddingsConfig, map[string]string, map[string]string, error) {
	var (
		adminCfg    *config.AdminConfig
		runtimeCfg  *config.RuntimeConfig
		toolcallCfg *config.ToolcallConfig
		respCfg     *config.ResponsesConfig
		embCfg      *config.EmbeddingsConfig
		claudeMap   map[string]string
		aliasMap    map[string]string
	)

	if raw, ok := req["admin"].(map[string]any); ok {
		cfg := &config.AdminConfig{}
		if v, exists := raw["jwt_expire_hours"]; exists {
			n := intFrom(v)
			if n < 1 || n > 720 {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("admin.jwt_expire_hours must be between 1 and 720")
			}
			cfg.JWTExpireHours = n
		}
		adminCfg = cfg
	}

	if raw, ok := req["runtime"].(map[string]any); ok {
		cfg := &config.RuntimeConfig{}
		if v, exists := raw["account_max_inflight"]; exists {
			n := intFrom(v)
			if n < 1 || n > 256 {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("runtime.account_max_inflight must be between 1 and 256")
			}
			cfg.AccountMaxInflight = n
		}
		if v, exists := raw["account_max_queue"]; exists {
			n := intFrom(v)
			if n < 1 || n > 200000 {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("runtime.account_max_queue must be between 1 and 200000")
			}
			cfg.AccountMaxQueue = n
		}
		if v, exists := raw["global_max_inflight"]; exists {
			n := intFrom(v)
			if n < 1 || n > 200000 {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("runtime.global_max_inflight must be between 1 and 200000")
			}
			cfg.GlobalMaxInflight = n
		}
		if cfg.AccountMaxInflight > 0 && cfg.GlobalMaxInflight > 0 && cfg.GlobalMaxInflight < cfg.AccountMaxInflight {
			return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("runtime.global_max_inflight must be >= runtime.account_max_inflight")
		}
		runtimeCfg = cfg
	}

	if raw, ok := req["toolcall"].(map[string]any); ok {
		cfg := &config.ToolcallConfig{}
		if v, exists := raw["mode"]; exists {
			mode := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", v)))
			switch mode {
			case "feature_match", "off":
				cfg.Mode = mode
			default:
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("toolcall.mode must be feature_match or off")
			}
		}
		if v, exists := raw["early_emit_confidence"]; exists {
			level := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", v)))
			switch level {
			case "high", "low", "off":
				cfg.EarlyEmitConfidence = level
			default:
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("toolcall.early_emit_confidence must be high, low or off")
			}
		}
		toolcallCfg = cfg
	}

	if raw, ok := req["responses"].(map[string]any); ok {
		cfg := &config.ResponsesConfig{}
		if v, exists := raw["store_ttl_seconds"]; exists {
			n := intFrom(v)
			if n < 30 || n > 86400 {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("responses.store_ttl_seconds must be between 30 and 86400")
			}
			cfg.StoreTTLSeconds = n
		}
		respCfg = cfg
	}

	if raw, ok := req["embeddings"].(map[string]any); ok {
		cfg := &config.EmbeddingsConfig{}
		if v, exists := raw["provider"]; exists {
			p := strings.TrimSpace(fmt.Sprintf("%v", v))
			if p == "" {
				return nil, nil, nil, nil, nil, nil, nil, fmt.Errorf("embeddings.provider cannot be empty")
			}
			cfg.Provider = p
		}
		embCfg = cfg
	}

	if raw, ok := req["claude_mapping"].(map[string]any); ok {
		claudeMap = map[string]string{}
		for k, v := range raw {
			key := strings.TrimSpace(k)
			val := strings.TrimSpace(fmt.Sprintf("%v", v))
			if key == "" || val == "" {
				continue
			}
			claudeMap[key] = val
		}
	}

	if raw, ok := req["model_aliases"].(map[string]any); ok {
		aliasMap = map[string]string{}
		for k, v := range raw {
			key := strings.TrimSpace(k)
			val := strings.TrimSpace(fmt.Sprintf("%v", v))
			if key == "" || val == "" {
				continue
			}
			aliasMap[key] = val
		}
	}

	return adminCfg, runtimeCfg, toolcallCfg, respCfg, embCfg, claudeMap, aliasMap, nil
}
