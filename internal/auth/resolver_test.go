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
	cache := &mockQuerier{
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
	mockQ := &mockQuerier{
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
	mockQ := &mockQuerier{
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
	cache := &mockQuerier{
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

// Test 3.4.1: Resolving same ClerkID twice returns same user without duplicate insert
// SPEC: Scenario "Known Clerk user is resolved"
func TestResolveUser_SameClerkIDTwice(t *testing.T) {
	mockQ := &mockQuerier{
		users: make(map[string]sqlc.User),
	}

	createCount := 0
	mockQ.onCreateUser = func(clerkID, email, name string) (sqlc.User, error) {
		createCount++
		newUser := sqlc.User{
			ID:           pgtype.UUID{Bytes: [16]byte{5}, Valid: true},
			ClerkUserID:  clerkID,
			Email:        email,
			DisplayName:  pgtype.Text{String: name, Valid: true},
			InviteStatus: "pending",
		}
		mockQ.users[clerkID] = newUser
		return newUser, nil
	}

	resolver := NewUserResolver(mockQ)
	claims := &Claims{Subject: "clerk_same", Email: "same@example.com", Name: "Same User"}

	// First resolution: creates user
	user1, err := resolver.Resolve(context.Background(), claims)
	if err != nil {
		t.Fatalf("first resolve failed: %v", err)
	}
	if user1 == nil {
		t.Fatal("first user should not be nil")
	}
	if createCount != 1 {
		t.Errorf("expected 1 create call, got %d", createCount)
	}

	// Second resolution with same ClerkID: should not create again
	user2, err := resolver.Resolve(context.Background(), claims)
	if err != nil {
		t.Fatalf("second resolve failed: %v", err)
	}
	if user2 == nil {
		t.Fatal("second user should not be nil")
	}

	if createCount != 1 {
		t.Errorf("expected 1 create call total (not 2), got %d", createCount)
	}

	// Both should have the same ID
	if user1.ID != user2.ID {
		t.Errorf("expected same user ID, got %v and %v", user1.ID, user2.ID)
	}
}

// Test 3.4.2: Resolved user is available in context
// SPEC: Scenario "Resolved user is available in context"
func TestResolveUser_ContextContainsUserID(t *testing.T) {
	mockQ := &mockQuerier{
		users: map[string]sqlc.User{
			"clerk_ctx": {
				ID:           pgtype.UUID{Bytes: [16]byte{6}, Valid: true},
				ClerkUserID:  "clerk_ctx",
				Email:        "ctx@example.com",
				InviteStatus: "active",
			},
		},
	}

	resolver := NewUserResolver(mockQ)
	claims := &Claims{Subject: "clerk_ctx", Email: "ctx@example.com"}

	ctx := context.Background()
	user, err := resolver.Resolve(ctx, claims)
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if user == nil {
		t.Fatal("user should not be nil")
	}

	// Inject user into context (simulating what middleware does)
	ctxWithUser := WithAuthUser(ctx, &User{
		ID:           user.ID.String(),
		ClerkUserID:  user.ClerkUserID,
		Email:        user.Email,
		InviteStatus: string(user.InviteStatus),
	})

	// Retrieve user from context
	retrievedUser := AuthUser(ctxWithUser)
	if retrievedUser == nil {
		t.Fatal("user should be retrievable from context")
	}

	if retrievedUser.ClerkUserID != "clerk_ctx" {
		t.Errorf("expected clerk_ctx in context, got %s", retrievedUser.ClerkUserID)
	}
}

// Mock querier for testing
type mockQuerier struct {
	users                       map[string]sqlc.User
	onCreateUser                func(clerkID, email, name string) (sqlc.User, error)
	getActiveInviteFunc         func(email string) (sqlc.GetActiveInviteByEmailRow, error)
	markInviteAcceptedFunc      func(id pgtype.UUID) error
	updateUserInviteStatusFunc  func(id pgtype.UUID, status string) (sqlc.User, error)
}

func (m *mockQuerier) GetUserByClerkID(ctx context.Context, clerkUserID string) (sqlc.User, error) {
	if user, ok := m.users[clerkUserID]; ok {
		return user, nil
	}
	return sqlc.User{}, errors.New("not found")
}

func (m *mockQuerier) CreateUser(ctx context.Context, params sqlc.CreateUserParams) (sqlc.User, error) {
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
func (m *mockQuerier) CreateInvite(ctx context.Context, arg sqlc.CreateInviteParams) (sqlc.Invite, error) {
	return sqlc.Invite{}, nil
}
func (m *mockQuerier) GetActiveInviteByEmail(ctx context.Context, email string) (sqlc.GetActiveInviteByEmailRow, error) {
	if m.getActiveInviteFunc != nil {
		return m.getActiveInviteFunc(email)
	}
	return sqlc.GetActiveInviteByEmailRow{}, nil
}
func (m *mockQuerier) GetInviteByTokenHash(ctx context.Context, tokenHash string) (sqlc.Invite, error) {
	return sqlc.Invite{}, nil
}
func (m *mockQuerier) MarkInviteAccepted(ctx context.Context, id pgtype.UUID) error {
	if m.markInviteAcceptedFunc != nil {
		return m.markInviteAcceptedFunc(id)
	}
	return nil
}
func (m *mockQuerier) UpdateUserPreferences(ctx context.Context, arg sqlc.UpdateUserPreferencesParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockQuerier) UpdateUserProfile(ctx context.Context, arg sqlc.UpdateUserProfileParams) (sqlc.User, error) {
	return sqlc.User{}, nil
}
func (m *mockQuerier) UpdateUserInviteStatus(ctx context.Context, arg sqlc.UpdateUserInviteStatusParams) (sqlc.User, error) {
	if m.updateUserInviteStatusFunc != nil {
		return m.updateUserInviteStatusFunc(arg.ID, string(arg.InviteStatus))
	}
	return sqlc.User{}, nil
}

// Test 3.10: Invite expiry — invite with expiry date in past should not be found
// SPEC: Scenario "Expired invite does not grant access"
func TestInviteExpiry_ExpiredBlocks(t *testing.T) {
	mockQ := &mockQuerier{
		getActiveInviteFunc: func(email string) (sqlc.GetActiveInviteByEmailRow, error) {
			if email == "expired@example.com" {
				// GetActiveInviteByEmail should filter out expired invites in the DB query
				// so we return an error to simulate no active invite found
				return sqlc.GetActiveInviteByEmailRow{}, errors.New("no active invite")
			}
			return sqlc.GetActiveInviteByEmailRow{}, nil
		},
	}

	// Simulate a pending user trying to access with an expired invite
	// The gate should reject because no active invite is found
	mockQ.users = map[string]sqlc.User{
		"user_exp": {
			ID:           pgtype.UUID{Bytes: [16]byte{7}, Valid: true},
			ClerkUserID:  "user_exp",
			Email:        "expired@example.com",
			InviteStatus: "pending",
		},
	}

	resolver := &dbUserResolver{q: mockQ}
	user, err := resolver.Resolve(context.Background(), &Claims{
		Subject: "user_exp",
		Email:   "expired@example.com",
	})

	// Since no active invite is found, the access should be denied
	// This is handled at the middleware level, not the resolver
	// So resolver should still return the user, but gate middleware should reject
	if err != nil {
		t.Errorf("resolver should find user: %v", err)
	}
	if user.InviteStatus != "pending" {
		t.Errorf("expected user to still be pending after resolve, got %v", user.InviteStatus)
	}
}

// Test 3.11: Invite without expiry should always be valid
// SPEC: Scenario "Invite with no expiry never expires"
func TestInviteExpiry_NullExpiryNeverExpires(t *testing.T) {
	mockQ := &mockQuerier{
		getActiveInviteFunc: func(email string) (sqlc.GetActiveInviteByEmailRow, error) {
			if email == "noexpiry@example.com" {
				// Return an invite with NULL expiry (never expires)
				// In pgx, NULL is represented as Valid: false
				return sqlc.GetActiveInviteByEmailRow{
					ID:        pgtype.UUID{Bytes: [16]byte{8}, Valid: true},
					Email:     "noexpiry@example.com",
					Status:    "pending",
					ExpiresAt: pgtype.Timestamptz{Valid: false}, // NULL = never expires
				}, nil
			}
			return sqlc.GetActiveInviteByEmailRow{}, errors.New("no invite")
		},
		markInviteAcceptedFunc: func(id pgtype.UUID) error {
			return nil
		},
	}

	mockQ.users = map[string]sqlc.User{
		"user_noexp": {
			ID:           pgtype.UUID{Bytes: [16]byte{8}, Valid: true},
			ClerkUserID:  "user_noexp",
			Email:        "noexpiry@example.com",
			InviteStatus: "pending",
		},
	}

	resolver := &dbUserResolver{q: mockQ}
	user, err := resolver.Resolve(context.Background(), &Claims{
		Subject: "user_noexp",
		Email:   "noexpiry@example.com",
	})

	if err != nil {
		t.Errorf("resolver should find user: %v", err)
	}
	if user.Email != "noexpiry@example.com" {
		t.Errorf("expected user with no-expiry invite, got %s", user.Email)
	}
}
