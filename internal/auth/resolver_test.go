package auth

import (
	"context"
	"errors"
	"testing"

	"github.com/fgjcarlos/ghamusinos/internal/db/sqlc"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
)

// Test 3.1: Known ClerkID returns existing user
func TestResolveUser_ExistingUser(t *testing.T) {
	cache := &mockUserQuerier{
		users: map[string]sqlc.User{
			"clerk_123": {
				ID:           pgtype.UUID{Bytes: [16]byte{1}, Valid: true},
				ClerkUserID:  "clerk_123",
				Email:        "existing@example.com",
				InviteStatus: "active",
			},
		},
	}

	resolver := NewUserResolver(cache)
	claims := &Claims{Subject: "clerk_123", Email: "existing@example.com"}

	user, err := resolver.Resolve(context.Background(), claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("user should not be nil")
	}
	if user.ClerkUserID != "clerk_123" {
		t.Errorf("expected clerk_123, got %s", user.ClerkUserID)
	}
	if user.InviteStatus != "active" {
		t.Errorf("expected active status, got %s", user.InviteStatus)
	}
}

// Test 3.2: Unknown ClerkID creates pending user on first call
func TestResolveUser_UnknownClerkID_CreatesUser(t *testing.T) {
	mockQ := &mockUserQuerier{
		users: make(map[string]sqlc.User),
	}

	// Set the callback after creation to capture mockQ
	mockQ.onCreateUser = func(clerkID, email, name string) (sqlc.User, error) {
		newUser := sqlc.User{
			ID:           pgtype.UUID{Bytes: [16]byte{2}, Valid: true},
			ClerkUserID:  clerkID,
			Email:        email,
			DisplayName:  pgtype.Text{String: name, Valid: true},
			InviteStatus: "pending",
		}
		mockQ.users[clerkID] = newUser
		return newUser, nil
	}

	resolver := NewUserResolver(mockQ)
	claims := &Claims{Subject: "clerk_new", Email: "new@example.com", Name: "New User"}

	user, err := resolver.Resolve(context.Background(), claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("user should not be nil")
	}
	if user.ClerkUserID != "clerk_new" {
		t.Errorf("expected clerk_new, got %s", user.ClerkUserID)
	}
	if user.InviteStatus != "pending" {
		t.Errorf("expected pending status, got %s", user.InviteStatus)
	}
	if !mockQ.users["clerk_new"].DisplayName.Valid || mockQ.users["clerk_new"].DisplayName.String != "New User" {
		t.Errorf("expected display name to be set")
	}
}

// Test 3.3: Race condition on unique constraint handled gracefully
func TestResolveUser_RaceCondition_Conflict(t *testing.T) {
	mockQ := &mockUserQuerier{
		users: make(map[string]sqlc.User),
	}

	callCount := 0
	mockQ.onCreateUser = func(clerkID, email, name string) (sqlc.User, error) {
		callCount++
		if callCount == 1 {
			// First create attempt: simulate concurrent insert causing conflict
			// Still need to add the user to simulate the race where another goroutine inserted it
			mockQ.users[clerkID] = sqlc.User{
				ID:           pgtype.UUID{Bytes: [16]byte{3}, Valid: true},
				ClerkUserID:  clerkID,
				Email:        email,
				InviteStatus: "pending",
			}
			err := &pgconn.PgError{Code: "23505"} // UNIQUE_VIOLATION
			return sqlc.User{}, err
		}
		// Should not reach here after conflict
		return sqlc.User{}, errors.New("unexpected second create call")
	}

	resolver := NewUserResolver(mockQ)
	claims := &Claims{Subject: "clerk_race", Email: "race@example.com"}

	user, err := resolver.Resolve(context.Background(), claims)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("user should not be nil after conflict retry")
	}
	if user.ClerkUserID != "clerk_race" {
		t.Errorf("expected clerk_race, got %s", user.ClerkUserID)
	}
	if callCount != 1 {
		t.Errorf("expected 1 create attempt (conflict + refetch), got %d", callCount)
	}
}

// Test 3.4: DB error during create returns error
func TestResolveUser_CreateError(t *testing.T) {
	dbErr := errors.New("connection lost")
	cache := &mockUserQuerier{
		onCreateUser: func(clerkID, email, name string) (sqlc.User, error) {
			return sqlc.User{}, dbErr
		},
	}

	resolver := NewUserResolver(cache)
	claims := &Claims{Subject: "clerk_err", Email: "error@example.com"}

	_, err := resolver.Resolve(context.Background(), claims)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, dbErr) {
		t.Errorf("expected %v, got %v", dbErr, err)
	}
}

// mockUserQuerier for testing user resolution (with resolver-specific features)
type mockUserQuerier struct {
	users        map[string]sqlc.User
	onCreateUser func(clerkID, email, name string) (sqlc.User, error)
}

func (m *mockUserQuerier) GetUserByClerkID(ctx context.Context, clerkUserID string) (sqlc.User, error) {
	if user, ok := m.users[clerkUserID]; ok {
		return user, nil
	}
	return sqlc.User{}, errors.New("not found")
}

func (m *mockUserQuerier) CreateUser(ctx context.Context, params sqlc.CreateUserParams) (sqlc.User, error) {
	if m.onCreateUser != nil {
		return m.onCreateUser(params.ClerkUserID, params.Email, params.DisplayName.String)
	}
	// Default: just store it
	newUser := sqlc.User{
		ID:           pgtype.UUID{Bytes: [16]byte{4}, Valid: true},
		ClerkUserID:  params.ClerkUserID,
		Email:        params.Email,
		DisplayName:  params.DisplayName,
		InviteStatus: params.InviteStatus,
	}
	m.users[params.ClerkUserID] = newUser
	return newUser, nil
}

// Implement remaining Querier methods as stubs
func (m *mockUserQuerier) CreateInvite(ctx context.Context, arg sqlc.CreateInviteParams) (sqlc.Invite, error) {
	return sqlc.Invite{}, nil
}
func (m *mockUserQuerier) GetActiveInviteByEmail(ctx context.Context, email string) (sqlc.GetActiveInviteByEmailRow, error) {
	return sqlc.GetActiveInviteByEmailRow{}, nil
}
func (m *mockUserQuerier) GetInviteByTokenHash(ctx context.Context, tokenHash string) (sqlc.Invite, error) {
	return sqlc.Invite{}, nil
}
func (m *mockUserQuerier) MarkInviteAccepted(ctx context.Context, id pgtype.UUID) error {
	return nil
}
func (m *mockUserQuerier) UpdateUserPreferences(ctx context.Context, arg sqlc.UpdateUserPreferencesParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockUserQuerier) UpdateUserProfile(ctx context.Context, arg sqlc.UpdateUserProfileParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockUserQuerier) UpdateUserInviteStatus(ctx context.Context, arg sqlc.UpdateUserInviteStatusParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
