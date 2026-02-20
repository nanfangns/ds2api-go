package admin

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	authn "ds2api/internal/auth"
)

func TestGetSettingsDefaultPasswordWarning(t *testing.T) {
	t.Setenv("DS2API_ADMIN_KEY", "")
	h := newAdminTestHandler(t, `{"keys":["k1"]}`)
	req := httptest.NewRequest(http.MethodGet, "/admin/settings", nil)
	rec := httptest.NewRecorder()
	h.getSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	admin, _ := body["admin"].(map[string]any)
	warn, _ := admin["default_password_warning"].(bool)
	if !warn {
		t.Fatalf("expected default password warning true, body=%v", body)
	}
}

func TestUpdateSettingsValidation(t *testing.T) {
	h := newAdminTestHandler(t, `{"keys":["k1"]}`)
	payload := map[string]any{
		"runtime": map[string]any{
			"account_max_inflight": 0,
		},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	h.updateSettings(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestUpdateSettingsValidationWithMergedRuntimeSnapshot(t *testing.T) {
	h := newAdminTestHandler(t, `{
		"keys":["k1"],
		"runtime":{
			"account_max_inflight":8,
			"global_max_inflight":8
		}
	}`)
	payload := map[string]any{
		"runtime": map[string]any{
			"account_max_inflight": 16,
		},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	h.updateSettings(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("runtime.global_max_inflight")) {
		t.Fatalf("expected merged runtime validation detail, got %s", rec.Body.String())
	}
}

func TestUpdateSettingsWithoutRuntimeSkipsMergedRuntimeValidation(t *testing.T) {
	h := newAdminTestHandler(t, `{
		"keys":["k1"],
		"runtime":{
			"account_max_inflight":8,
			"global_max_inflight":4
		}
	}`)
	payload := map[string]any{
		"responses": map[string]any{
			"store_ttl_seconds": 600,
		},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	h.updateSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if got := h.Store.Snapshot().Responses.StoreTTLSeconds; got != 600 {
		t.Fatalf("store_ttl_seconds=%d want=600", got)
	}
}

func TestUpdateSettingsHotReloadRuntime(t *testing.T) {
	h := newAdminTestHandler(t, `{
		"keys":["k1"],
		"accounts":[{"email":"a@test.com","token":"t1"},{"email":"b@test.com","token":"t2"}]
	}`)

	payload := map[string]any{
		"runtime": map[string]any{
			"account_max_inflight": 3,
			"account_max_queue":    20,
			"global_max_inflight":  5,
		},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	h.updateSettings(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
	status := h.Pool.Status()
	if got := intFrom(status["max_inflight_per_account"]); got != 3 {
		t.Fatalf("max_inflight_per_account=%d want=3", got)
	}
	if got := intFrom(status["max_queue_size"]); got != 20 {
		t.Fatalf("max_queue_size=%d want=20", got)
	}
	if got := intFrom(status["global_max_inflight"]); got != 5 {
		t.Fatalf("global_max_inflight=%d want=5", got)
	}
}

func TestUpdateSettingsPasswordInvalidatesOldJWT(t *testing.T) {
	hash := authn.HashAdminPassword("old-password")
	h := newAdminTestHandler(t, `{"admin":{"password_hash":"`+hash+`"}}`)

	token, err := authn.CreateJWTWithStore(1, h.Store)
	if err != nil {
		t.Fatalf("create jwt failed: %v", err)
	}
	if _, err := authn.VerifyJWTWithStore(token, h.Store); err != nil {
		t.Fatalf("verify before update failed: %v", err)
	}

	body := map[string]any{"new_password": "new-password"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/admin/settings/password", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	h.updateSettingsPassword(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}

	if _, err := authn.VerifyJWTWithStore(token, h.Store); err == nil {
		t.Fatal("expected old token to be invalid after password update")
	}
	if !authn.VerifyAdminCredential("new-password", h.Store) {
		t.Fatal("expected new password credential to be accepted")
	}
}

func TestConfigImportMergeAndReplace(t *testing.T) {
	h := newAdminTestHandler(t, `{
		"keys":["k1"],
		"accounts":[{"email":"a@test.com","password":"p1"}]
	}`)

	merge := map[string]any{
		"mode": "merge",
		"config": map[string]any{
			"keys": []any{"k1", "k2"},
			"accounts": []any{
				map[string]any{"email": "a@test.com", "password": "p1"},
				map[string]any{"email": "b@test.com", "password": "p2"},
			},
		},
	}
	mergeBytes, _ := json.Marshal(merge)
	mergeReq := httptest.NewRequest(http.MethodPost, "/admin/config/import?mode=merge", bytes.NewReader(mergeBytes))
	mergeRec := httptest.NewRecorder()
	h.configImport(mergeRec, mergeReq)
	if mergeRec.Code != http.StatusOK {
		t.Fatalf("merge status=%d body=%s", mergeRec.Code, mergeRec.Body.String())
	}
	if got := len(h.Store.Keys()); got != 2 {
		t.Fatalf("keys after merge=%d want=2", got)
	}
	if got := len(h.Store.Accounts()); got != 2 {
		t.Fatalf("accounts after merge=%d want=2", got)
	}

	replace := map[string]any{
		"mode": "replace",
		"config": map[string]any{
			"keys": []any{"k9"},
		},
	}
	replaceBytes, _ := json.Marshal(replace)
	replaceReq := httptest.NewRequest(http.MethodPost, "/admin/config/import?mode=replace", bytes.NewReader(replaceBytes))
	replaceRec := httptest.NewRecorder()
	h.configImport(replaceRec, replaceReq)
	if replaceRec.Code != http.StatusOK {
		t.Fatalf("replace status=%d body=%s", replaceRec.Code, replaceRec.Body.String())
	}
	keys := h.Store.Keys()
	if len(keys) != 1 || keys[0] != "k9" {
		t.Fatalf("unexpected keys after replace: %#v", keys)
	}
	if got := len(h.Store.Accounts()); got != 0 {
		t.Fatalf("accounts after replace=%d want=0", got)
	}
}

func TestConfigImportRejectsInvalidRuntimeBounds(t *testing.T) {
	h := newAdminTestHandler(t, `{"keys":["k1"]}`)
	payload := map[string]any{
		"mode": "replace",
		"config": map[string]any{
			"keys": []any{"k2"},
			"runtime": map[string]any{
				"account_max_inflight": 300,
			},
		},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/config/import?mode=replace", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	h.configImport(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("runtime.account_max_inflight")) {
		t.Fatalf("expected runtime bound detail, got %s", rec.Body.String())
	}
	keys := h.Store.Keys()
	if len(keys) != 1 || keys[0] != "k1" {
		t.Fatalf("store should remain unchanged, keys=%v", keys)
	}
}

func TestConfigImportRejectsMergedRuntimeConflict(t *testing.T) {
	h := newAdminTestHandler(t, `{
		"keys":["k1"],
		"runtime":{
			"account_max_inflight":8,
			"global_max_inflight":8
		}
	}`)
	payload := map[string]any{
		"mode": "merge",
		"config": map[string]any{
			"runtime": map[string]any{
				"account_max_inflight": 16,
			},
		},
	}
	b, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, "/admin/config/import?mode=merge", bytes.NewReader(b))
	rec := httptest.NewRecorder()
	h.configImport(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("runtime.global_max_inflight")) {
		t.Fatalf("expected merged runtime validation detail, got %s", rec.Body.String())
	}
	snap := h.Store.Snapshot()
	if snap.Runtime.AccountMaxInflight != 8 || snap.Runtime.GlobalMaxInflight != 8 {
		t.Fatalf("runtime should remain unchanged, runtime=%+v", snap.Runtime)
	}
}
