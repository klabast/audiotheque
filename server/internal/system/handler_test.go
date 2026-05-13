package system

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockAuthService struct {
	adminExists bool
	err         error
}

func (m *mockAuthService) DoesAdminUserExist() (bool, error) {
	return m.adminExists, m.err
}

func TestHandleSystem_NoAdminUser_RequiresAdminUser(t *testing.T) {
	mockAuth := &mockAuthService{adminExists: false}
	handler := NewHandler(mockAuth)

	req := httptest.NewRequest(http.MethodGet, "/api/system", nil)
	w := httptest.NewRecorder()

	handler.HandleSystem(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response SystemResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if !response.RequiresAdminUser {
		t.Error("expected requiresAdminUser to be true when admin doesn't exist")
	}

	if response.Version == "" {
		t.Error("expected version to be set")
	}
}

func TestHandleSystem_AdminUserExists_NoSetupRequired(t *testing.T) {
	mockAuth := &mockAuthService{adminExists: true}
	handler := NewHandler(mockAuth)

	req := httptest.NewRequest(http.MethodGet, "/api/system", nil)
	w := httptest.NewRecorder()

	handler.HandleSystem(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response SystemResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}

	if response.RequiresAdminUser {
		t.Error("expected requiresAdminUser to be false when admin exists")
	}
}
