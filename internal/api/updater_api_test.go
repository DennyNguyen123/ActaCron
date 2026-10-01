package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"actacron/internal/version"
)

func TestHandleGetVersion(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/version", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", w.Code)
	}

	var res map[string]string
	if err := json.Unmarshal(w.Body.Bytes(), &res); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if res["version"] != version.Version {
		t.Errorf("expected version %s, got %s", version.Version, res["version"])
	}
}

func TestHandleCheckUpdateMethodNotAllowed(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("POST", "/api/update/check", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestHandleApplyUpdateMethodNotAllowed(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, nil, nil)

	req := httptest.NewRequest("GET", "/api/update/apply", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}
