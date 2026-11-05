package openai

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestGetModelRouteDirectAndRejectsRetiredIDs(t *testing.T) {
	h := &openAITestSurface{}
	r := chi.NewRouter()
	registerOpenAITestRoutes(r, h)
	for model, want := range map[string]int{"deepseek-flash": http.StatusOK, "deepseek-flash-nothinking": http.StatusNotFound, "deepseek-v4-pro": http.StatusNotFound, "gpt-4.1": http.StatusNotFound} {
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/models/"+model, nil))
		if rec.Code != want {
			t.Fatalf("%s: status=%d want=%d", model, rec.Code, want)
		}
	}
}

func TestGetModelRouteNotFound(t *testing.T) {
	h := &openAITestSurface{}
	r := chi.NewRouter()
	registerOpenAITestRoutes(r, h)

	req := httptest.NewRequest(http.MethodGet, "/v1/models/not-exists", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d body=%s", rec.Code, rec.Body.String())
	}
}
