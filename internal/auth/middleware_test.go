package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgtype"
)

var errNoInvite = errors.New("no valid invite")

// Test 3.5: AuthMiddleware returns 401 on missing Authorization header
func TestAuthMiddleware_MissingAuth(t *testing.T) {
	validator := &mockJWTValidator{
		onValidate: func(ctx context.Context, token string) (*Claims, error) {
			return nil, ErrUnauthenticated
		},
	}

	handler := AuthMiddleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "unauthorized" {
		t.Errorf("expected error='unauthorized', got %s", resp["error"])
	}
}

// Test 3.6.1: AuthMiddleware rejects malformed Bearer header
// SPEC: Scenario "Missing Authorization header returns 401"
func TestAuthMiddleware_MalformedHeader(t *testing.T) {
	validator := &mockJWTValidator{
		onValidate: func(ctx context.Context, token string) (*Claims, error) {
			return nil, ErrUnauthenticated
		},
	}

	handler := AuthMiddleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	// Missing token part (just "Bearer")
	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "unauthorized" {
		t.Errorf("expected error='unauthorized', got %s", resp["error"])
	}

	// Verify no token leak in response
	if strings.Contains(w.Body.String(), "Bearer") {
		t.Error("token leaked in response body")
	}
}

// Test 3.6.2: AuthMiddleware rejects invalid tokens with JSON error
// SPEC: Scenario "Missing Authorization header returns 401"
func TestAuthMiddleware_InvalidToken(t *testing.T) {
	validator := &mockJWTValidator{
		onValidate: func(ctx context.Context, token string) (*Claims, error) {
			return nil, ErrUnauthenticated
		},
	}

	handler := AuthMiddleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer invalid.jwt.token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}

	if w.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %s", w.Header().Get("Content-Type"))
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "unauthorized" {
		t.Errorf("expected error='unauthorized', got %s", resp["error"])
	}

	// Verify no token leak
	if strings.Contains(w.Body.String(), "invalid.jwt.token") {
		t.Error("token leaked in response body")
	}
}

// Test 3.6: AuthMiddleware extracts token and validates
func TestAuthMiddleware_ValidToken(t *testing.T) {
	validator := &mockJWTValidator{
		onValidate: func(ctx context.Context, token string) (*Claims, error) {
			if token == "valid-token" {
				return &Claims{Subject: "user_123", Email: "user@example.com"}, nil
			}
			return nil, ErrUnauthenticated
		},
	}

	handler := AuthMiddleware(validator)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := AuthClaims(r.Context())
		if claims == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	req.Header.Set("Authorization", "Bearer valid-token")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// Test 3.7: ResolveMiddleware injects user into context
func TestResolveMiddleware_InjectsUser(t *testing.T) {
	resolver := &mockUserResolver{
		onResolve: func(ctx context.Context, claims *Claims) (*sqlc.User, error) {
			return &sqlc.User{
				ID:           pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
				ClerkUserID:  claims.Subject,
				Email:        claims.Email,
				InviteStatus: "active",
			}, nil
		},
	}

	handler := ResolveMiddleware(resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := AuthUser(r.Context())
		if user == nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Prepare request with claims already in context
	req := httptest.NewRequest("GET", "/api/test", nil)
	claims := &Claims{Subject: "user_123", Email: "user@example.com"}
	req = req.WithContext(WithAuthClaims(context.Background(), claims))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// Test 3.8.1: InviteGateMiddleware blocks blocked users
// SPEC: Scenario "Blocked user is denied"
func TestInviteGateMiddleware_BlockedUser(t *testing.T) {
	mockQ := &mockQuerier{}

	handler := InviteGateMiddleware(mockQ)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	user := &User{
		ID:           "uuid-blocked",
		ClerkUserID:  "user_blocked",
		Email:        "blocked@example.com",
		InviteStatus: "blocked",
	}
	req = req.WithContext(WithAuthUser(context.Background(), user))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}

	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "forbidden" {
		t.Errorf("expected error='forbidden', got %s", resp["error"])
	}
}

// Test 3.8: InviteGateMiddleware blocks pending users without valid invite
func TestInviteGateMiddleware_PendingNoInvite(t *testing.T) {
	mockQ := &mockQuerier{}
	// Override GetActiveInviteByEmail to return an error for pending users without invite
	mockQ.getActiveInviteFunc = func(email string) (sqlc.GetActiveInviteByEmailRow, error) {
		return sqlc.GetActiveInviteByEmailRow{}, errNoInvite
	}

	handler := InviteGateMiddleware(mockQ)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	user := &User{
		ID:           "uuid-1",
		ClerkUserID:  "user_123",
		Email:        "user@example.com",
		InviteStatus: "pending",
	}
	req = req.WithContext(WithAuthUser(context.Background(), user))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

// Test 3.9.1: InviteGateMiddleware with valid invite lookup
// SPEC: Scenario "Pending user with valid invite is promoted"
// Note: Full acceptance flow tested in integration tests (skipped with -short)
func TestInviteGateMiddleware_FindsValidInvite(t *testing.T) {
	mockQ := &mockQuerier{
		getActiveInviteFunc: func(email string) (sqlc.GetActiveInviteByEmailRow, error) {
			if email == "pending@example.com" {
				// Return a valid, non-expired invite
				return sqlc.GetActiveInviteByEmailRow{
					ID:    pgtype.UUID{Bytes: [16]byte{9}, Valid: true},
					Email: "pending@example.com",
				}, nil
			}
			return sqlc.GetActiveInviteByEmailRow{}, errors.New("no invite")
		},
		markInviteAcceptedFunc: func(id pgtype.UUID) error {
			return nil
		},
	}

	// Verify that GetActiveInviteByEmail is called for pending users
	foundInvite := false
	mockQ.getActiveInviteFunc = func(email string) (sqlc.GetActiveInviteByEmailRow, error) {
		if email == "pending@example.com" {
			foundInvite = true
			return sqlc.GetActiveInviteByEmailRow{
				ID:    pgtype.UUID{Bytes: [16]byte{9}, Valid: true},
				Email: "pending@example.com",
			}, nil
		}
		return sqlc.GetActiveInviteByEmailRow{}, errors.New("no invite")
	}

	mockQ.markInviteAcceptedFunc = func(id pgtype.UUID) error {
		return nil
	}

	handler := InviteGateMiddleware(mockQ)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Handler is called if invite check allows it
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	user := &User{
		ID:           "uuid-pending",
		ClerkUserID:  "user_pending",
		Email:        "pending@example.com",
		InviteStatus: "pending",
	}
	req = req.WithContext(WithAuthUser(context.Background(), user))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// Verify the invite lookup was attempted
	if !foundInvite {
		t.Error("expected GetActiveInviteByEmail to be called for pending user")
	}
}

// Test 3.9.2: InviteGateMiddleware blocks pending user with expired invite
// SPEC: Scenario "Expired invite does not grant access"
func TestInviteGateMiddleware_ExpiredInvite(t *testing.T) {
	mockQ := &mockQuerier{
		getActiveInviteFunc: func(email string) (sqlc.GetActiveInviteByEmailRow, error) {
			if email == "pending-exp@example.com" {
				// Invite exists but is expired (expires_at in past)
				// GetActiveInviteByEmail should NOT return expired invites
				// So we simulate that by returning an error (no active invite found)
				return sqlc.GetActiveInviteByEmailRow{}, errors.New("no active invite")
			}
			return sqlc.GetActiveInviteByEmailRow{}, nil
		},
	}

	handler := InviteGateMiddleware(mockQ)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	user := &User{
		ID:           "uuid-exp",
		ClerkUserID:  "user_exp",
		Email:        "pending-exp@example.com",
		InviteStatus: "pending",
	}
	req = req.WithContext(WithAuthUser(context.Background(), user))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403 for expired invite, got %d", w.Code)
	}
}

// Test 3.9: InviteGateMiddleware allows active users
func TestInviteGateMiddleware_Active(t *testing.T) {
	querier := &mockQuerier{}

	handler := InviteGateMiddleware(querier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest("GET", "/api/test", nil)
	user := &User{
		ID:           "uuid-1",
		ClerkUserID:  "user_123",
		Email:        "user@example.com",
		InviteStatus: "active",
	}
	req = req.WithContext(WithAuthUser(context.Background(), user))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

// Mock JWT validator
type mockJWTValidator struct {
	onValidate func(ctx context.Context, token string) (*Claims, error)
}

func (m *mockJWTValidator) Validate(ctx context.Context, rawToken string) (*Claims, error) {
	if m.onValidate != nil {
		return m.onValidate(ctx, rawToken)
	}
	return nil, ErrUnauthenticated
}

// Mock user resolver
type mockUserResolver struct {
	onResolve func(ctx context.Context, claims *Claims) (*sqlc.User, error)
}

func (m *mockUserResolver) Resolve(ctx context.Context, claims *Claims) (*sqlc.User, error) {
	if m.onResolve != nil {
		return m.onResolve(ctx, claims)
	}
	return nil, ErrUnauthenticated
}
