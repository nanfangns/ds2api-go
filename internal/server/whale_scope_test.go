package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicSurfaceMatchesWhaleModelAndRoutes(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["test"]}`)
	t.Setenv("DS2API_CHAT_HISTORY_PATH", t.TempDir()+"/history.json")
	app, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	app.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("models status=%d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data) != 1 || payload.Data[0].ID != "deepseek-flash" {
		t.Fatalf("unexpected model list: %+v", payload.Data)
	}
	for _, model := range []string{"deepseek-v4-flash", "deepseek-v4-pro", "deepseek-v4-vision", "deepseek-v4-flash-search", "deepseek-flash-nothinking"} {
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"`+model+`","messages":[{"role":"user","content":"hello"}]}`))
		req.Header.Set("Authorization", "Bearer direct-test-token")
		rec = httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("model %s: status=%d body=%s", model, rec.Code, rec.Body.String())
		}
	}
	for _, path := range []string{"/anthropic/v1/messages", "/v1beta/models/deepseek-flash:generateContent", "/api/tags"} {
		rec = httptest.NewRecorder()
		app.Router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusNotFound {
			t.Fatalf("path %s: status=%d", path, rec.Code)
		}
	}
}

func TestRetiredModelIsRejectedBeforeInlineUpload(t *testing.T) {
	t.Setenv("DS2API_CONFIG_JSON", `{"keys":["test"]}`)
	t.Setenv("DS2API_CHAT_HISTORY_PATH", t.TempDir()+"/history.json")
	app, err := NewApp()
	if err != nil {
		t.Fatal(err)
	}
	for path, body := range map[string]string{
		"/v1/chat/completions": `{"model":"deepseek-v4-flash","messages":[{"role":"user","content":[{"type":"image","data":"aGVsbG8=","mime_type":"image/png"}]}]}`,
		"/v1/responses":        `{"model":"deepseek-v4-flash","input":[{"role":"user","content":[{"type":"image","data":"aGVsbG8=","mime_type":"image/png"}]}]}`,
	} {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer direct-test-token")
		rec := httptest.NewRecorder()
		app.Router.ServeHTTP(rec, req)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "not available") {
			t.Fatalf("%s: expected early model validation, status=%d body=%s", path, rec.Code, rec.Body.String())
		}
	}
}
