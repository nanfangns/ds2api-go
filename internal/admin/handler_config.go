package admin

import (
	"crypto/md5"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"ds2api/internal/config"
)

func (h *Handler) getConfig(w http.ResponseWriter, _ *http.Request) {
	snap := h.Store.Snapshot()
	safe := map[string]any{
		"keys":     snap.Keys,
		"accounts": []map[string]any{},
		"claude_mapping": func() map[string]string {
			if len(snap.ClaudeMapping) > 0 {
				return snap.ClaudeMapping
			}
			return snap.ClaudeModelMap
		}(),
	}
	accounts := make([]map[string]any, 0, len(snap.Accounts))
	for _, acc := range snap.Accounts {
		token := strings.TrimSpace(acc.Token)
		preview := ""
		if token != "" {
			if len(token) > 20 {
				preview = token[:20] + "..."
			} else {
				preview = token
			}
		}
		accounts = append(accounts, map[string]any{
			"identifier":    acc.Identifier(),
			"email":         acc.Email,
			"mobile":        acc.Mobile,
			"has_password":  strings.TrimSpace(acc.Password) != "",
			"has_token":     token != "",
			"token_preview": preview,
		})
	}
	safe["accounts"] = accounts
	writeJSON(w, http.StatusOK, safe)
}

func (h *Handler) updateConfig(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}
	old := h.Store.Snapshot()
	err := h.Store.Update(func(c *config.Config) error {
		if keys, ok := toStringSlice(req["keys"]); ok {
			c.Keys = keys
		}
		if accountsRaw, ok := req["accounts"].([]any); ok {
			existing := map[string]config.Account{}
			for _, a := range old.Accounts {
				existing[a.Identifier()] = a
			}
			accounts := make([]config.Account, 0, len(accountsRaw))
			for _, item := range accountsRaw {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				acc := toAccount(m)
				id := acc.Identifier()
				if prev, ok := existing[id]; ok {
					if strings.TrimSpace(acc.Password) == "" {
						acc.Password = prev.Password
					}
					if strings.TrimSpace(acc.Token) == "" {
						acc.Token = prev.Token
					}
				}
				accounts = append(accounts, acc)
			}
			c.Accounts = accounts
		}
		if m, ok := req["claude_mapping"].(map[string]any); ok {
			newMap := map[string]string{}
			for k, v := range m {
				newMap[k] = fmt.Sprintf("%v", v)
			}
			c.ClaudeMapping = newMap
		}
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	h.Pool.Reset()
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "message": "配置已更新"})
}

func (h *Handler) addKey(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	_ = json.NewDecoder(r.Body).Decode(&req)
	key, _ := req["key"].(string)
	key = strings.TrimSpace(key)
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "Key 不能为空"})
		return
	}
	err := h.Store.Update(func(c *config.Config) error {
		for _, k := range c.Keys {
			if k == key {
				return fmt.Errorf("Key 已存在")
			}
		}
		c.Keys = append(c.Keys, key)
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "total_keys": len(h.Store.Snapshot().Keys)})
}

func (h *Handler) deleteKey(w http.ResponseWriter, r *http.Request) {
	key := chi.URLParam(r, "key")
	err := h.Store.Update(func(c *config.Config) error {
		idx := -1
		for i, k := range c.Keys {
			if k == key {
				idx = i
				break
			}
		}
		if idx < 0 {
			return fmt.Errorf("Key 不存在")
		}
		c.Keys = append(c.Keys[:idx], c.Keys[idx+1:]...)
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]any{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "total_keys": len(h.Store.Snapshot().Keys)})
}

func (h *Handler) batchImport(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "无效的 JSON 格式"})
		return
	}
	importedKeys, importedAccounts := 0, 0
	err := h.Store.Update(func(c *config.Config) error {
		if keys, ok := req["keys"].([]any); ok {
			existing := map[string]bool{}
			for _, k := range c.Keys {
				existing[k] = true
			}
			for _, k := range keys {
				key := strings.TrimSpace(fmt.Sprintf("%v", k))
				if key == "" || existing[key] {
					continue
				}
				c.Keys = append(c.Keys, key)
				existing[key] = true
				importedKeys++
			}
		}
		if accounts, ok := req["accounts"].([]any); ok {
			existing := map[string]bool{}
			for _, a := range c.Accounts {
				existing[a.Identifier()] = true
			}
			for _, item := range accounts {
				m, ok := item.(map[string]any)
				if !ok {
					continue
				}
				acc := toAccount(m)
				id := acc.Identifier()
				if id == "" || existing[id] {
					continue
				}
				c.Accounts = append(c.Accounts, acc)
				existing[id] = true
				importedAccounts++
			}
		}
		return nil
	})
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	h.Pool.Reset()
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "imported_keys": importedKeys, "imported_accounts": importedAccounts})
}

func (h *Handler) exportConfig(w http.ResponseWriter, _ *http.Request) {
	h.configExport(w, nil)
}

func (h *Handler) configExport(w http.ResponseWriter, _ *http.Request) {
	snap := h.Store.Snapshot()
	jsonStr, b64, err := h.Store.ExportJSONAndBase64()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"config":  snap,
		"json":    jsonStr,
		"base64":  b64,
	})
}

func (h *Handler) configImport(w http.ResponseWriter, r *http.Request) {
	var req map[string]any
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid json"})
		return
	}

	mode := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("mode")))
	if mode == "" {
		mode = strings.TrimSpace(strings.ToLower(fieldString(req, "mode")))
	}
	if mode == "" {
		mode = "merge"
	}
	if mode != "merge" && mode != "replace" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "mode must be merge or replace"})
		return
	}

	payload := req
	if raw, ok := req["config"].(map[string]any); ok && len(raw) > 0 {
		payload = raw
	}
	rawJSON, err := json.Marshal(payload)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": "invalid config payload"})
		return
	}
	var incoming config.Config
	if err := json.Unmarshal(rawJSON, &incoming); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"detail": err.Error()})
		return
	}

	importedKeys, importedAccounts := 0, 0
	err = h.Store.Update(func(c *config.Config) error {
		next := c.Clone()
		if mode == "replace" {
			next = incoming.Clone()
			next.VercelSyncHash = c.VercelSyncHash
			next.VercelSyncTime = c.VercelSyncTime
			importedKeys = len(next.Keys)
			importedAccounts = len(next.Accounts)
		} else {
			existingKeys := map[string]struct{}{}
			for _, k := range next.Keys {
				existingKeys[k] = struct{}{}
			}
			for _, k := range incoming.Keys {
				key := strings.TrimSpace(k)
				if key == "" {
					continue
				}
				if _, ok := existingKeys[key]; ok {
					continue
				}
				existingKeys[key] = struct{}{}
				next.Keys = append(next.Keys, key)
				importedKeys++
			}

			existingAccounts := map[string]struct{}{}
			for _, acc := range next.Accounts {
				existingAccounts[acc.Identifier()] = struct{}{}
			}
			for _, acc := range incoming.Accounts {
				id := acc.Identifier()
				if id == "" {
					continue
				}
				if _, ok := existingAccounts[id]; ok {
					continue
				}
				existingAccounts[id] = struct{}{}
				next.Accounts = append(next.Accounts, acc)
				importedAccounts++
			}

			if len(incoming.ClaudeMapping) > 0 {
				if next.ClaudeMapping == nil {
					next.ClaudeMapping = map[string]string{}
				}
				for k, v := range incoming.ClaudeMapping {
					next.ClaudeMapping[k] = v
				}
			}
			if len(incoming.ClaudeModelMap) > 0 {
				if next.ClaudeModelMap == nil {
					next.ClaudeModelMap = map[string]string{}
				}
				for k, v := range incoming.ClaudeModelMap {
					next.ClaudeModelMap[k] = v
				}
			}

			if len(incoming.ModelAliases) > 0 {
				if next.ModelAliases == nil {
					next.ModelAliases = map[string]string{}
				}
				for k, v := range incoming.ModelAliases {
					next.ModelAliases[k] = v
				}
			}
			if strings.TrimSpace(incoming.Toolcall.Mode) != "" {
				next.Toolcall.Mode = incoming.Toolcall.Mode
			}
			if strings.TrimSpace(incoming.Toolcall.EarlyEmitConfidence) != "" {
				next.Toolcall.EarlyEmitConfidence = incoming.Toolcall.EarlyEmitConfidence
			}
			if incoming.Responses.StoreTTLSeconds > 0 {
				next.Responses.StoreTTLSeconds = incoming.Responses.StoreTTLSeconds
			}
			if strings.TrimSpace(incoming.Embeddings.Provider) != "" {
				next.Embeddings.Provider = incoming.Embeddings.Provider
			}
			if strings.TrimSpace(incoming.Admin.PasswordHash) != "" {
				next.Admin.PasswordHash = incoming.Admin.PasswordHash
			}
			if incoming.Admin.JWTExpireHours > 0 {
				next.Admin.JWTExpireHours = incoming.Admin.JWTExpireHours
			}
			if incoming.Admin.JWTValidAfterUnix > 0 {
				next.Admin.JWTValidAfterUnix = incoming.Admin.JWTValidAfterUnix
			}
			if incoming.Runtime.AccountMaxInflight > 0 {
				next.Runtime.AccountMaxInflight = incoming.Runtime.AccountMaxInflight
			}
			if incoming.Runtime.AccountMaxQueue > 0 {
				next.Runtime.AccountMaxQueue = incoming.Runtime.AccountMaxQueue
			}
			if incoming.Runtime.GlobalMaxInflight > 0 {
				next.Runtime.GlobalMaxInflight = incoming.Runtime.GlobalMaxInflight
			}
		}

		normalizeSettingsConfig(&next)
		if err := validateSettingsConfig(next); err != nil {
			return newRequestError(err.Error())
		}

		*c = next
		return nil
	})
	if err != nil {
		if detail, ok := requestErrorDetail(err); ok {
			writeJSON(w, http.StatusBadRequest, map[string]any{"detail": detail})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"detail": err.Error()})
		return
	}

	h.Pool.Reset()
	writeJSON(w, http.StatusOK, map[string]any{
		"success":           true,
		"mode":              mode,
		"imported_keys":     importedKeys,
		"imported_accounts": importedAccounts,
		"message":           "config imported",
	})
}

func (h *Handler) computeSyncHash() string {
	snap := h.Store.Snapshot().Clone()
	snap.VercelSyncHash = ""
	snap.VercelSyncTime = 0
	b, _ := json.Marshal(snap)
	sum := md5.Sum(b)
	return fmt.Sprintf("%x", sum)
}
