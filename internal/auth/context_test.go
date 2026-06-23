package auth

import (
	"context"
	"testing"
)

func TestAuthClaimsContext(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(context.Context) context.Context
		verify func(*testing.T, *Claims, error)
	}{
		{
			name: "empty context returns nil",
			setup: func(ctx context.Context) context.Context {
				return ctx
			},
			verify: func(t *testing.T, claims *Claims, err error) {
				if claims != nil {
					t.Errorf("expected nil claims in empty context, got %v", claims)
				}
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
			},
		},
		{
			name: "populated context returns claims",
			setup: func(ctx context.Context) context.Context {
				claims := &Claims{
					Subject: "user_123",
					Email:   "test@example.com",
					Name:    "Test User",
				}
				return WithAuthClaims(ctx, claims)
			},
			verify: func(t *testing.T, claims *Claims, err error) {
				if claims == nil {
					t.Error("expected claims in context, got nil")
				}
				if claims != nil && claims.Subject != "user_123" {
					t.Errorf("expected Subject=user_123, got %s", claims.Subject)
				}
				if claims != nil && claims.Email != "test@example.com" {
					t.Errorf("expected Email=test@example.com, got %s", claims.Email)
				}
				if claims != nil && claims.Name != "Test User" {
					t.Errorf("expected Name=Test User, got %s", claims.Name)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = tt.setup(ctx)
			claims := AuthClaims(ctx)
			tt.verify(t, claims, nil)
		})
	}
}

func TestAuthUserContext(t *testing.T) {
	tests := []struct {
		name   string
		setup  func(context.Context) context.Context
		verify func(*testing.T, *User)
	}{
		{
			name: "empty context returns nil",
			setup: func(ctx context.Context) context.Context {
				return ctx
			},
			verify: func(t *testing.T, user *User) {
				if user != nil {
					t.Errorf("expected nil user in empty context, got %v", user)
				}
			},
		},
		{
			name: "populated context returns user",
			setup: func(ctx context.Context) context.Context {
				user := &User{
					ID:           "uuid-123",
					ClerkUserID:  "clerk_user_123",
					Email:        "user@example.com",
					DisplayName:  "John Doe",
					InviteStatus: "active",
				}
				return WithAuthUser(ctx, user)
			},
			verify: func(t *testing.T, user *User) {
				if user == nil {
					t.Error("expected user in context, got nil")
				}
				if user != nil && user.ID != "uuid-123" {
					t.Errorf("expected ID=uuid-123, got %s", user.ID)
				}
				if user != nil && user.ClerkUserID != "clerk_user_123" {
					t.Errorf("expected ClerkUserID=clerk_user_123, got %s", user.ClerkUserID)
				}
				if user != nil && user.Email != "user@example.com" {
					t.Errorf("expected Email=user@example.com, got %s", user.Email)
				}
				if user != nil && user.DisplayName != "John Doe" {
					t.Errorf("expected DisplayName=John Doe, got %s", user.DisplayName)
				}
				if user != nil && user.InviteStatus != "active" {
					t.Errorf("expected InviteStatus=active, got %s", user.InviteStatus)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			ctx = tt.setup(ctx)
			user := AuthUser(ctx)
			tt.verify(t, user)
		})
	}
}

func TestContextChaining(t *testing.T) {
	// Test that we can set and retrieve both claims and user in same context
	ctx := context.Background()
	claims := &Claims{
		Subject: "user_abc",
		Email:   "abc@example.com",
		Name:    "ABC User",
	}
	user := &User{
		ID:          "uuid-abc",
		ClerkUserID: "user_abc",
		Email:       "abc@example.com",
	}

	ctx = WithAuthClaims(ctx, claims)
	ctx = WithAuthUser(ctx, user)

	retrievedClaims := AuthClaims(ctx)
	retrievedUser := AuthUser(ctx)

	if retrievedClaims == nil {
		t.Error("expected claims after chaining")
	}
	if retrievedClaims != nil && retrievedClaims.Subject != "user_abc" {
		t.Errorf("claims corrupted: expected user_abc, got %s", retrievedClaims.Subject)
	}

	if retrievedUser == nil {
		t.Error("expected user after chaining")
	}
	if retrievedUser != nil && retrievedUser.ID != "uuid-abc" {
		t.Errorf("user corrupted: expected uuid-abc, got %s", retrievedUser.ID)
	}
}
