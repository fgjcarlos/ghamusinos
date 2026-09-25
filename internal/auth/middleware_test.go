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
	//nolint:errcheck
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

	handler := InviteGateMiddleware(mockQ, &mockInvitePromoter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	handler := InviteGateMiddleware(querier, &mockInvitePromoter{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

// Test 3.10: Pending user with valid invite is promoted to active through
// the InvitePromoter in a single transaction, and the next request from the
// now-active user does NOT call the promoter again (issue #168, A1 AC:
// "Un usuario pending con invitación válida queda active tras la primera
// petición" + "La segunda petición del mismo usuario no ejecuta ningún
// UPDATE").
func TestInviteGateMiddleware_PendingPromotesOnceAndThenPasses(t *testing.T) {
	// Real UUID so pgtypeUUIDFromString inside the middleware succeeds.
	const userID = "550e8400-e29b-41d4-a716-446655440000"

	mockQ := &mockQuerier{}
	mockQ.getActiveInviteFunc = func(email string) (sqlc.GetActiveInviteByEmailRow, error) {
		return sqlc.GetActiveInviteByEmailRow{ID: pgtype.UUID{Bytes: [16]byte{9}, Valid: true}}, nil
	}
	promoter := &mockInvitePromoter{
		onPromote: func(ctx context.Context, inviteID, uid pgtype.UUID) (sqlc.User, error) {
			return sqlc.User{
				ID:           uid,
				ClerkUserID:  "user_123",
				Email:        "user@example.com",
				InviteStatus: "active",
			}, nil
		},
	}

	var promotedSeen *User
	handler := InviteGateMiddleware(mockQ, promoter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		promotedSeen = AuthUser(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	pending := &User{
		ID:           userID,
		ClerkUserID:  "user_123",
		Email:        "user@example.com",
		InviteStatus: "pending",
	}

	// First request: pending → promoter runs → handler proceeds with active user.
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)
	req = req.WithContext(WithAuthUser(context.Background(), pending))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("first request: expected 200, got %d", w.Code)
	}
	if promoter.promoteCalls != 1 {
		t.Errorf("expected 1 promoter call on first request, got %d", promoter.promoteCalls)
	}
	if promotedSeen == nil || promotedSeen.InviteStatus != "active" {
		t.Errorf("handler should see active user after promotion, got %+v", promotedSeen)
	}

	// Second request: user is now active → promoter is NOT consulted.
	active := &User{
		ID:           userID,
		ClerkUserID:  "user_123",
		Email:        "user@example.com",
		InviteStatus: "active",
	}
	req2 := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)
	req2 = req2.WithContext(WithAuthUser(context.Background(), active))
	w2 := httptest.NewRecorder()
	handler.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Fatalf("second request: expected 200, got %d", w2.Code)
	}
	if promoter.promoteCalls != 1 {
		t.Errorf("second request must not invoke promoter; total calls = %d", promoter.promoteCalls)
	}
}

// Test 3.11: A failing InvitePromoter returns 500 to the client and the
// invite is NOT marked accepted (the promoter is the only place that
// commits the atomic write; if it errors out we surface that to the
// caller). issue #168, A1 AC: "si el segundo falla, la invitación no debe
// quedar marcada como aceptada".
func TestInviteGateMiddleware_PromoterError(t *testing.T) {
	const userID = "550e8400-e29b-41d4-a716-446655440000"

	mockQ := &mockQuerier{}
	mockQ.getActiveInviteFunc = func(email string) (sqlc.GetActiveInviteByEmailRow, error) {
		return sqlc.GetActiveInviteByEmailRow{ID: pgtype.UUID{Bytes: [16]byte{9}, Valid: true}}, nil
	}
	promoter := &mockInvitePromoter{
		onPromote: func(ctx context.Context, inviteID, uid pgtype.UUID) (sqlc.User, error) {
			return sqlc.User{}, errors.New("connection lost mid-transaction")
		},
	}

	handler := InviteGateMiddleware(mockQ, promoter)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	pending := &User{
		ID:           userID,
		ClerkUserID:  "user_123",
		Email:        "user@example.com",
		InviteStatus: "pending",
	}
	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)
	req = req.WithContext(WithAuthUser(context.Background(), pending))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 on promoter failure, got %d", w.Code)
	}
	if promoter.promoteCalls != 1 {
		t.Errorf("promoter should have been called once, got %d", promoter.promoteCalls)
	}
}

// Test 3.12: ResolveMiddleware translates ErrEmailTaken into a 409 with a
// clear, user-facing message instead of an opaque 500 (issue #168, AC:
// "Una colisión de email devuelve 409 con mensaje, no 500").
func TestResolveMiddleware_EmailCollision_Returns409(t *testing.T) {
	resolver := &mockUserResolver{
		onResolve: func(ctx context.Context, claims *Claims) (*sqlc.User, error) {
			return nil, ErrEmailTaken
		},
	}

	handler := ResolveMiddleware(resolver)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), "GET", "/api/test", nil)
	claims := &Claims{Subject: "clerk_dup", Email: "taken@example.com"}
	req = req.WithContext(WithAuthClaims(context.Background(), claims))
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 on email collision, got %d", w.Code)
	}

	var resp map[string]string
	//nolint:errcheck
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] == "" || resp["error"] == "internal error" {
		t.Errorf("expected a meaningful conflict message, got %q", resp["error"])
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

// Mock invite promoter for InviteGateMiddleware tests. Captures the call
// count so tests can assert that the second request from an already-active
// user does not invoke the promoter again (issue #168, AC: "La segunda
// petición del mismo usuario no ejecuta ningún UPDATE").
type mockInvitePromoter struct {
	promoteCalls int
	onPromote    func(ctx context.Context, inviteID, userID pgtype.UUID) (sqlc.User, error)
}

func (m *mockInvitePromoter) PromotePendingInvite(ctx context.Context, inviteID, userID pgtype.UUID) (sqlc.User, error) {
	m.promoteCalls++
	if m.onPromote != nil {
		return m.onPromote(ctx, inviteID, userID)
	}
	return sqlc.User{
		ID:           userID,
		ClerkUserID:  "user_123",
		Email:        "user@example.com",
		InviteStatus: "active",
	}, nil
}
