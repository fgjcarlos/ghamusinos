package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/auth"
	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

// Test 4a.1: GET /api/me returns authenticated user
func TestMe_ValidUser(t *testing.T) {
	mockQ := &mockMeQuerier{}
	handler := Me(mockQ)

	req := httptest.NewRequest("GET", "/api/me", nil)
	user := &auth.User{
		ID:           "uuid-123",
		ClerkUserID:  "clerk_123",
		Email:        "user@example.com",
		DisplayName:  "Test User",
		InviteStatus: "active",
	}
	req = req.WithContext(auth.WithAuthUser(context.Background(), user))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp["id"] != "uuid-123" || resp["clerk_user_id"] != "clerk_123" {
		t.Errorf("response missing or incorrect user data: %v", resp)
	}
}

// Test 4a.2: GET /api/me returns 401 if no user in context
func TestMe_NoUser(t *testing.T) {
	mockQ := &mockMeQuerier{}
	handler := Me(mockQ)

	req := httptest.NewRequest("GET", "/api/me", nil)
	// No user in context
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	// Check Content-Type is problem+json
	contentType := w.Header().Get("Content-Type")
	if contentType != "application/problem+json" {
		t.Errorf("expected Content-Type 'application/problem+json', got %q", contentType)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	// Check RFC 9457 structure
	if resp["title"] != "Unauthorized" {
		t.Errorf("expected title='Unauthorized', got %v", resp["title"])
	}
	if resp["status"] != float64(401) {
		t.Errorf("expected status=401, got %v", resp["status"])
	}
}

// Mock querier for Me handler tests
type mockMeQuerier struct{}

func (m *mockMeQuerier) CreateInvite(ctx context.Context, arg sqlc.CreateInviteParams) (sqlc.Invite, error) {
	return sqlc.Invite{}, nil
}
func (m *mockMeQuerier) CreateUser(ctx context.Context, arg sqlc.CreateUserParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockMeQuerier) GetActiveInviteByEmail(ctx context.Context, email string) (sqlc.GetActiveInviteByEmailRow, error) {
	return sqlc.GetActiveInviteByEmailRow{}, nil
}
func (m *mockMeQuerier) GetInviteByTokenHash(ctx context.Context, tokenHash string) (sqlc.Invite, error) {
	return sqlc.Invite{}, nil
}
func (m *mockMeQuerier) GetUserByClerkID(ctx context.Context, clerkUserID string) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockMeQuerier) MarkInviteAccepted(ctx context.Context, id pgtype.UUID) error {
	return nil
}
func (m *mockMeQuerier) UpdateUserPreferences(ctx context.Context, arg sqlc.UpdateUserPreferencesParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockMeQuerier) UpdateUserProfile(ctx context.Context, arg sqlc.UpdateUserProfileParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockMeQuerier) UpdateUserInviteStatus(ctx context.Context, arg sqlc.UpdateUserInviteStatusParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
