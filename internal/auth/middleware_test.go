package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
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
		//nolint:errcheck

		w.Write([]byte("ok"))
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)
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

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)
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
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)
	claims := &Claims{Subject: "user_123", Email: "user@example.com"}
	req = req.WithContext(WithAuthClaims(context.Background(), claims))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
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

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)
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

// Test 3.9: InviteGateMiddleware allows active users
func TestInviteGateMiddleware_Active(t *testing.T) {
	querier := &mockQuerier{}

	handler := InviteGateMiddleware(querier)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)
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
